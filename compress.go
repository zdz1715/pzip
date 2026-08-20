package pzip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/zdz1715/pzip/flate"
)

type CompressorFactory func(w io.Writer, level int) (flate.Writer, error)

type CompressEvent struct {
	Header *FileHeader
}

type CompressOptions struct {
	Sources         []string
	StripPrefix     string
	StripComponents int
	AddPrefix       string
	Concurrency     int
	Level           int
	Comment         string
	Recursive       bool
	FollowLinks     bool
	Filter          Filter
	Compressor      CompressorFactory
	Progress        func(CompressEvent)
}

type compressConfig struct {
	*CompressOptions
	tempRoot string
	objects  *objectPool
}

type entrySpec struct {
	sourcePath string
	name       string
	info       fs.FileInfo
}

func (o *CompressOptions) validate() error {
	if o == nil {
		return errors.New("compress options must not be nil")
	}
	if len(o.Sources) == 0 {
		return errors.New("no sources to compress")
	}
	if err := validateStripOptions(o.StripPrefix, o.StripComponents); err != nil {
		return err
	}
	return validLevel(o.Level)
}

func validLevel(level int) error {
	if level < -2 || level > 9 {
		return fmt.Errorf("invalid compression level %d: want value in range [-2, 9]", level)
	}
	return nil
}

func (o *CompressOptions) setDefaults() {
	if o.Concurrency <= 0 {
		o.Concurrency = runtime.GOMAXPROCS(0)
	}
	if o.Compressor == nil {
		o.Compressor = flate.NewWriter
	}
}

func Compress(ctx context.Context, dst string, opts *CompressOptions) (err error) {
	if dst == "" {
		return errors.New("compress destination must not be empty")
	}
	if err = opts.validate(); err != nil {
		return err
	}
	opts.setDefaults()

	absZipPath, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	tempRoot, err := os.MkdirTemp(filepath.Dir(absZipPath), ".pzip-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempRoot)

	tmpFile, err := os.CreateTemp(tempRoot, filepath.Base(absZipPath))
	if err != nil {
		return err
	}

	defer func() {
		if err == nil {
			if closeErr := tmpFile.Close(); closeErr != nil {
				err = closeErr
			} else if renameErr := os.Rename(tmpFile.Name(), absZipPath); renameErr != nil {
				err = renameErr
			}
		} else {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}
	}()

	return compressToWriter(ctx, tmpFile, &compressConfig{
		CompressOptions: opts,
		tempRoot:        tempRoot,
		objects:         newObjectPool(),
	})
}

func CompressToWriter(ctx context.Context, w io.Writer, opts *CompressOptions) error {
	if err := opts.validate(); err != nil {
		return err
	}
	opts.setDefaults()

	tempRoot, err := os.MkdirTemp("", ".pzip-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempRoot)

	return compressToWriter(ctx, w, &compressConfig{
		CompressOptions: opts,
		tempRoot:        tempRoot,
		objects:         newObjectPool(),
	})
}

func compressToWriter(ctx context.Context, dst io.Writer, cfg *compressConfig) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	zw := NewWriter(dst)
	if cfg.Comment != "" {
		if err = zw.SetComment(cfg.Comment); err != nil {
			return err
		}
	}
	defer func() {
		if closeErr := zw.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed close writer: %w", closeErr))
		}
	}()

	ready := make(chan *Object, cfg.Concurrency)
	writerDone := make(chan error, 1)

	go func() {
		writerDone <- cfg.writeReadyObjects(ctx, zw, ready)
	}()

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.Concurrency)

	scanErr := cfg.scan(gctx, func(spec entrySpec) error {
		g.Go(func() error {
			obj, prepareErr := cfg.prepare(spec)
			if prepareErr != nil {
				return prepareErr
			}

			if compressErr := obj.Compress(); compressErr != nil {
				_ = obj.Close()
				cfg.release(obj)
				return compressErr
			}

			select {
			case ready <- obj:
				return nil
			case <-gctx.Done():
				_ = obj.Close()
				cfg.release(obj)
				return gctx.Err()
			}
		})
		return nil
	})

	if scanErr != nil {
		cancel()
	}

	compressErr := g.Wait()
	close(ready)

	if writeErr := <-writerDone; writeErr != nil {
		cancel()
		err = errors.Join(err, fmt.Errorf("write: %w", writeErr))
	}
	if scanErr != nil {
		err = errors.Join(err, fmt.Errorf("scan: %w", scanErr))
	}
	if compressErr != nil {
		err = errors.Join(err, fmt.Errorf("compress: %w", compressErr))
	}
	return err
}

func (cfg *compressConfig) writeReadyObjects(ctx context.Context, zw *Writer, ready <-chan *Object) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case obj, ok := <-ready:
			if !ok {
				return nil
			}
			if err := obj.WriteTo(zw); err != nil {
				_ = obj.Close()
				cfg.release(obj)
				return err
			}
			if cfg.Progress != nil {
				cfg.Progress(CompressEvent{Header: obj.header})
			}
			if err := obj.Close(); err != nil {
				cfg.release(obj)
				return err
			}
			cfg.release(obj)
		}
	}
}

func (cfg *compressConfig) prepare(spec entrySpec) (*Object, error) {
	var obj *Object
	if cfg.objects != nil {
		obj = cfg.objects.Get()
	} else {
		obj = &Object{
			compressedData: newObjectBuffer(),
		}
	}
	obj.Root = cfg.tempRoot
	if err := obj.Reset(spec.sourcePath, spec.info, cfg.Level, cfg.Compressor); err != nil {
		cfg.release(obj)
		return nil, err
	}
	obj.header.Name = spec.name
	return obj, nil
}

func (cfg *compressConfig) release(obj *Object) {
	if cfg.objects != nil {
		cfg.objects.Put(obj)
	}
}

func (cfg *compressConfig) scan(ctx context.Context, emit func(entrySpec) error) error {
	for _, source := range cfg.Sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !cfg.Recursive {
			if source == "." {
				continue
			}
			if err := cfg.emitPath(source, emit); err != nil {
				return err
			}
			continue
		}
		if err := cfg.walkSource(ctx, source, "", emit); err != nil {
			return err
		}
	}
	return nil
}

func (cfg *compressConfig) walkSource(ctx context.Context, source, linkName string, emit func(entrySpec) error) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == "." || path == ".." || path == "./" {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		displayPath := path
		if linkName != "" {
			displayPath = filepath.Join(linkName, strings.TrimPrefix(path, source))
		}

		if cfg.FollowLinks && IsSymlink(info.Mode()) {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			return cfg.walkSource(ctx, target, displayPath, emit)
		}

		return cfg.emitPathAs(path, displayPath, info, emit)
	})
}

func (cfg *compressConfig) emitPath(path string, emit func(entrySpec) error) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return cfg.emitPathAs(path, path, info, emit)
}

func (cfg *compressConfig) emitPathAs(sourcePath, displayPath string, info fs.FileInfo, emit func(entrySpec) error) error {
	name, ok, err := cfg.entryName(displayPath)
	if err != nil || !ok {
		return err
	}
	if !cfg.Filter.Accept(name) {
		return nil
	}
	return emit(entrySpec{
		sourcePath: sourcePath,
		name:       name,
		info:       info,
	})
}

func (cfg *compressConfig) entryName(sourcePath string) (string, bool, error) {
	sourcePath = filepath.Clean(sourcePath)
	if cfg.StripPrefix != "" {
		prefix := filepath.Clean(cfg.StripPrefix)
		rel, err := filepath.Rel(prefix, sourcePath)
		if err != nil {
			return "", false, err
		}
		if rel == "." {
			return "", false, nil
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", false, fmt.Errorf("%q is not under strip prefix %q", sourcePath, cfg.StripPrefix)
		}
		sourcePath = rel
	}

	name := HeaderName(sourcePath)
	if cfg.StripComponents > 0 {
		var ok bool
		name, ok = stripArchivePath(name, "", cfg.StripComponents)
		if !ok {
			return "", false, nil
		}
	}
	if cfg.AddPrefix != "" {
		name = path.Join(path.Clean(filepath.ToSlash(cfg.AddPrefix)), name)
	}
	name = HeaderName(name)
	return name, name != "" && name != ".", nil
}
