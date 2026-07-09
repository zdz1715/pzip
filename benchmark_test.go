package pzip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func BenchmarkCompress(b *testing.B) {
	sourceRoot := createBenchmarkTree(b, 64, 256<<10)
	outRoot := b.TempDir()

	b.Run("sequential-baseline", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := filepath.Join(outRoot, fmt.Sprintf("seq-%d.zip", i))
			if err := compressSequentialForBenchmark(dst, sourceRoot); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("parallel-ready-no-pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := filepath.Join(outRoot, fmt.Sprintf("parallel-no-pool-%d.zip", i))
			if err := compressNoPoolForBenchmark(dst, sourceRoot); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("parallel-ready-pooled", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := filepath.Join(outRoot, fmt.Sprintf("parallel-pooled-%d.zip", i))
			if err := Compress(context.Background(), dst, &CompressOptions{
				Sources:     []string{sourceRoot},
				StripPrefix: filepath.Dir(sourceRoot),
				Recursive:   true,
				Concurrency: runtime.GOMAXPROCS(0),
				Level:       -1,
			}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkExtract(b *testing.B) {
	sourceRoot := createBenchmarkTree(b, 64, 256<<10)
	archive := filepath.Join(b.TempDir(), "bench.zip")
	if err := Compress(context.Background(), archive, &CompressOptions{
		Sources:     []string{sourceRoot},
		StripPrefix: filepath.Dir(sourceRoot),
		Recursive:   true,
		Concurrency: runtime.GOMAXPROCS(0),
		Level:       -1,
	}); err != nil {
		b.Fatal(err)
	}

	outRoot := b.TempDir()

	b.Run("sequential-baseline", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := filepath.Join(outRoot, fmt.Sprintf("extract-seq-%d", i))
			if err := Extract(context.Background(), archive, &ExtractOptions{
				Destination: dst,
				Concurrency: 1,
			}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("parallel", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := filepath.Join(outRoot, fmt.Sprintf("extract-parallel-%d", i))
			if err := Extract(context.Background(), archive, &ExtractOptions{
				Destination: dst,
				Concurrency: runtime.GOMAXPROCS(0),
			}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func createBenchmarkTree(b *testing.B, files int, size int) string {
	b.Helper()

	root := filepath.Join(b.TempDir(), "src")
	if err := os.MkdirAll(root, 0755); err != nil {
		b.Fatal(err)
	}

	data := make([]byte, size)
	for i := range data {
		data[i] = byte('a' + i%8)
	}

	for i := 0; i < files; i++ {
		dir := filepath.Join(root, fmt.Sprintf("dir-%02d", i%8))
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatal(err)
		}
		path := filepath.Join(dir, fmt.Sprintf("file-%03d.txt", i))
		if err := os.WriteFile(path, data, 0644); err != nil {
			b.Fatal(err)
		}
	}
	return root
}

func compressSequentialForBenchmark(dst string, sourceRoot string) (err error) {
	tempRoot, err := os.MkdirTemp(filepath.Dir(dst), ".pzip-bench-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempRoot)

	tmpFile, err := os.CreateTemp(tempRoot, filepath.Base(dst))
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			if closeErr := tmpFile.Close(); closeErr != nil {
				err = closeErr
			} else if renameErr := os.Rename(tmpFile.Name(), dst); renameErr != nil {
				err = renameErr
			}
		} else {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}
	}()

	zw := NewWriter(tmpFile)
	defer func() {
		if closeErr := zw.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	cfg := &compressConfig{
		CompressOptions: &CompressOptions{
			Sources:     []string{sourceRoot},
			StripPrefix: filepath.Dir(sourceRoot),
			Recursive:   true,
			Concurrency: 1,
			Level:       -1,
		},
		tempRoot: tempRoot,
	}
	cfg.setDefaults()

	return cfg.scan(context.Background(), func(spec entrySpec) error {
		obj, err := cfg.prepare(spec)
		if err != nil {
			return err
		}
		defer obj.Close()
		if err := obj.Compress(); err != nil {
			return err
		}
		return obj.WriteTo(zw)
	})
}

func compressNoPoolForBenchmark(dst string, sourceRoot string) (err error) {
	opts := &CompressOptions{
		Sources:     []string{sourceRoot},
		StripPrefix: filepath.Dir(sourceRoot),
		Recursive:   true,
		Concurrency: runtime.GOMAXPROCS(0),
		Level:       -1,
	}
	if err := opts.validate(); err != nil {
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

	return compressToWriter(context.Background(), tmpFile, &compressConfig{
		CompressOptions: opts,
		tempRoot:        tempRoot,
	})
}
