package pzip

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zdz1715/pzip/flate"
)

const (
	sequentialWrites = 1
)

type ArchiveOptions struct {
	SkipPath

	tempRoot string

	NewCompressor flate.NewWriterFunc
	Files         []string
	Level         int
	Concurrency   int
	Comment       string
	Dereference   bool
	Recurse       bool
	After         func(hdr *FileHeader)
}

func (o *ArchiveOptions) filterFile() {
	if !o.Recurse && len(o.Files) > 0 {
		newFiles := make([]string, 0, len(o.Files))
		for _, file := range o.Files {
			if file != "." {
				newFiles = append(newFiles, file)
			}
		}
		o.Files = newFiles
	}

}

func (o *ArchiveOptions) Validate() error {
	o.filterFile()
	if len(o.Files) == 0 {
		return errors.New("no files to archive")
	}
	if o.Concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1, got %d", o.Concurrency)
	}
	return validLevel(o.Level)
}

func (o *ArchiveOptions) archiveFile(fileAbsPath, file string, fn func(absPath string, obj *Object) error) (error, error) {
	if o.Recurse {
		return o.recurseArchiveFile(file, "", fn)
	}
	info, err := os.Lstat(file)
	if err != nil {
		return err, nil
	}

	obj, err := DefaultObjectPool.New(file, info, o.Level, o.NewCompressor)
	if err != nil {
		return err, nil
	}

	obj.Root = o.tempRoot

	return nil, fn(fileAbsPath, obj)
}

func (o *ArchiveOptions) recurseArchiveFile(file string, link string, fn func(absPath string, obj *Object) error) (error, error) {
	var submitErr error
	walkErr := filepath.WalkDir(file, func(path string, d fs.DirEntry, err error) error {
		if err != nil || submitErr != nil {
			return err
		}

		if path == "." || path == ".." || path == "./" {
			return nil
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		// skip temp dir
		if absPath == o.tempRoot || filepath.Dir(absPath) == o.tempRoot {
			return nil
		}

		pathOverride := path
		if link != "" {
			pathOverride = filepath.Join(link, strings.TrimPrefix(path, file))
		}

		if o.Skip(pathOverride) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if o.Dereference && IsSymlink(info.Mode()) {
			target, linkErr := os.Readlink(path)
			if linkErr != nil {
				return linkErr
			}

			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}

			err, submitErr = o.recurseArchiveFile(target, path, fn)
			if err != nil {
				return fmt.Errorf("%s -> %s: %w", path, target, err)
			}

			return nil
		}

		obj, err := DefaultObjectPool.New(pathOverride, info, o.Level, o.NewCompressor)
		if err != nil {
			return err
		}
		obj.Root = o.tempRoot
		submitErr = fn(absPath, obj)

		return nil
	})
	return walkErr, submitErr
}

func Archive(ctx context.Context, path string, opts *ArchiveOptions) (err error) {
	if opts == nil {
		return fmt.Errorf("archive options must not be nil")
	}

	if err = opts.Validate(); err != nil {
		return
	}

	if opts.NewCompressor == nil {
		opts.NewCompressor = func(w io.Writer, level int) (flate.Writer, error) {
			return flate.NewFastWriter(w, level)
		}
	}

	absZipPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	opts.tempRoot, err = os.MkdirTemp(filepath.Dir(absZipPath), ".pzip-")
	if err != nil {
		return err
	}

	defer os.RemoveAll(opts.tempRoot)

	tmpFile, err := os.CreateTemp(opts.tempRoot, filepath.Base(absZipPath))
	if err != nil {
		return
	}

	// close tmp file and rename
	defer func() {
		if err == nil {
			if closeErr := tmpFile.Close(); closeErr != nil {
				err = closeErr
			} else {
				if renameErr := os.Rename(tmpFile.Name(), absZipPath); renameErr != nil {
					err = renameErr
				}
			}
		} else {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}
	}()

	w := NewWriter(tmpFile)
	// Execute before tmpFile close
	defer func() {
		if closeErr := w.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("header end write: %w", closeErr))
		}
	}()
	// sequential write
	writeWorker := NewFailFastWorker[Object](func(params *Object) error {
		defer func() {
			_ = params.Close()
			DefaultObjectPool.Put(params)
		}()

		if writeErr := params.Archive(w); writeErr != nil {
			return writeErr
		}
		if opts.After != nil {
			opts.After(params.header)
		}
		return nil
	}, sequentialWrites, sequentialWrites)

	// parallel compression
	compressWorker := NewFailFastWorker[Object](func(params *Object) error {
		var compressErr error
		defer func() {
			if compressErr != nil {
				// delete overflow file
				_ = params.Close()
			}
		}()
		if compressErr = params.Compress(); compressErr != nil {
			return compressErr
		}

		if compressErr = writeWorker.Submit(params); compressErr != nil {
			return compressErr
		}
		return nil
	}, opts.Concurrency, opts.Concurrency)

	compressWorker.Start(ctx)
	writeWorker.Start(ctx)

	var (
		submitErr   error
		fileAbsPath string
	)
	// add File
	for _, file := range opts.Files {
		fileAbsPath, err = filepath.Abs(file)
		if err != nil {
			return err
		}
		err, submitErr = opts.archiveFile(fileAbsPath, file, func(absPtah string, obj *Object) error {
			if absPtah == absZipPath {
				return nil
			}
			return compressWorker.Submit(obj)
		})

		if err != nil {
			return err
		}

		// stop submit, wait worker error
		if submitErr != nil {
			break
		}
	}

	if execErr := compressWorker.Wait(); execErr != nil {
		err = errors.Join(err, fmt.Errorf("compress: %w", execErr))
	}

	if execErr := writeWorker.Wait(); execErr != nil {
		err = errors.Join(err, fmt.Errorf("write: %w", execErr))
	}

	return
}

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

type ExtractOptions struct {
	SkipPath

	OutDir      string
	Concurrency int
	Before      func(path string, r *ReadCloser)
	After       func(f *File, target *ExtractTarget)
}

func (o *ExtractOptions) Validate() error {
	if o.Concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1, got %d", o.Concurrency)
	}
	return nil
}

func (o *ExtractOptions) extractFile(file *File) (target *ExtractTarget, err error) {
	target = &ExtractTarget{
		Path: file.Name,
	}
	if o.OutDir != "" {
		target.Path = filepath.Join(o.OutDir, file.Name)
	}

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

func (o *ExtractOptions) writeLink(outputPath string, file *File) (string, error) {
	srcFile, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open file %q: %w", file.Name, err)
	}
	defer func() {
		if cerr := srcFile.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close source file %q: %w", file.Name, cerr)
		}
	}()

	buf := make([]byte, file.CompressedSize64)
	_, err = srcFile.Read(buf)
	if err != nil {
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
	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, file.Mode())
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
	if opts == nil {
		return errors.New("extract options must not be nil")
	}
	if err := opts.Validate(); err != nil {
		return err
	}

	reader, err := OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	if opts.Before != nil {
		opts.Before(path, reader)
	}

	worker := NewFailFastWorker[File](func(params *File) error {
		t, extractErr := opts.extractFile(params)
		if extractErr != nil {
			return extractErr
		}
		if opts.After != nil {
			opts.After(params, t)
		}
		return nil
	}, opts.Concurrency, opts.Concurrency)

	worker.Start(ctx)

	for _, f := range reader.File {
		if opts.Skip(f.Name) {
			continue
		}
		// stop submit, wait error
		if submitErr := worker.Submit(f); submitErr != nil {
			break
		}
	}

	return worker.Wait()
}

func GetComment(path string) (string, error) {
	reader, err := OpenReader(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	return reader.Comment, nil
}
