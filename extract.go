package pzip

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sync/errgroup"
)

type ExtractTarget struct {
	Path    string
	Symlink string
}

func (e *ExtractTarget) String() string {
	builder := new(strings.Builder)
	builder.WriteString(e.Path)
	if e.Symlink != "" {
		builder.WriteString(" -> ")
		builder.WriteString(e.Symlink)
	}
	return builder.String()
}

type ExtractEvent struct {
	File   *File
	Target *ExtractTarget
}

type ExtractOptions struct {
	Destination string
	Concurrency int
	Filter      Filter
	Progress    func(ExtractEvent)
}

func (o *ExtractOptions) validate() error {
	if o == nil {
		return errors.New("extract options must not be nil")
	}
	if o.Concurrency <= 0 {
		o.Concurrency = runtime.GOMAXPROCS(0)
	}
	return nil
}

func (o *ExtractOptions) extractFile(file *File) (target *ExtractTarget, err error) {
	targetPath, err := o.targetPath(file.Name)
	if err != nil {
		return nil, err
	}
	target = &ExtractTarget{Path: targetPath}

	dir := filepath.Dir(target.Path)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return target, fmt.Errorf("create directory %q: %w", dir, err)
	}

	if strings.HasSuffix(filepath.ToSlash(file.Name), "/") {
		return target, o.writeDir(target.Path, file)
	}

	if IsSymlink(file.Mode()) {
		l, err := o.writeLink(target.Path, file)
		if err != nil {
			return target, err
		}
		target.Symlink = l
		return target, nil
	}

	return target, o.writeFile(target.Path, file)
}

func (o *ExtractOptions) targetPath(name string) (string, error) {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || filepath.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe zip entry path %q", name)
	}
	if o.Destination == "" {
		return cleanName, nil
	}

	base, err := filepath.Abs(o.Destination)
	if err != nil {
		return "", err
	}
	target := filepath.Join(base, cleanName)
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe zip entry path %q", name)
	}
	return target, nil
}

func (o *ExtractOptions) writeLink(outputPath string, file *File) (string, error) {
	srcFile, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open file %q: %w", file.Name, err)
	}
	defer srcFile.Close()

	buf := make([]byte, file.UncompressedSize64)
	if _, err = io.ReadFull(srcFile, buf); err != nil {
		return "", err
	}
	link := string(buf)
	return link, os.Symlink(link, outputPath)
}

func (o *ExtractOptions) writeDir(outputPath string, file *File) error {
	err := os.Mkdir(outputPath, file.Mode())
	if os.IsExist(err) {
		if err = os.Chmod(outputPath, file.Mode()); err != nil {
			return fmt.Errorf("chmod directory %q: %w", outputPath, err)
		}
	} else if err != nil {
		return fmt.Errorf("create directory %q: %w", outputPath, err)
	}

	return nil
}

func (o *ExtractOptions) writeFile(outputPath string, file *File) (err error) {
	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
	if err != nil {
		return fmt.Errorf("create file %q: %w", outputPath, err)
	}

	defer func() {
		if cerr := outputFile.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close output file %q: %w", outputPath, cerr)
		}
	}()

	var srcFile io.ReadCloser
	if file.Method == zip.Store {
		srcReFile, err := file.OpenRaw()
		if err != nil {
			return fmt.Errorf("open file %q: %w", file.Name, err)
		}
		srcFile = io.NopCloser(srcReFile)
	} else {
		srcFile, err = file.Open()
	}
	if err != nil {
		return fmt.Errorf("open file %q: %w", file.Name, err)
	}

	defer func() {
		if cerr := srcFile.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close source file %q: %w", file.Name, cerr)
		}
	}()

	if _, err = io.Copy(outputFile, srcFile); err != nil {
		return fmt.Errorf("decompress file %q: %w", file.Name, err)
	}
	return nil
}

func Extract(ctx context.Context, path string, opts *ExtractOptions) error {
	if err := opts.validate(); err != nil {
		return err
	}

	reader, err := OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Concurrency)

	for _, f := range reader.File {
		if !opts.Filter.Accept(f.Name) {
			continue
		}
		f := f
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			t, extractErr := opts.extractFile(f)
			if extractErr != nil {
				return extractErr
			}
			if opts.Progress != nil {
				opts.Progress(ExtractEvent{File: f, Target: t})
			}
			return nil
		})
	}

	return g.Wait()
}

func Comment(path string) (string, error) {
	reader, err := OpenReader(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	return reader.Comment, nil
}
