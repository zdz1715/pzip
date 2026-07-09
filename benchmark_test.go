package pzip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type benchmarkDataset struct {
	name string
	root string
}

func BenchmarkCompress(b *testing.B) {
	for _, dataset := range createBenchmarkDatasets(b) {
		dataset := dataset
		b.Run(dataset.name, func(b *testing.B) {
			outRoot := b.TempDir()

			b.Run("sequential-baseline", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					dst := filepath.Join(outRoot, fmt.Sprintf("seq-%d.zip", i))
					if err := compressSequentialForBenchmark(dst, dataset.root); err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run("parallel", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					dst := filepath.Join(outRoot, fmt.Sprintf("parallel-%d.zip", i))
					if err := Compress(context.Background(), dst, benchmarkCompressOptions(dataset.root, runtime.GOMAXPROCS(0))); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func BenchmarkExtract(b *testing.B) {
	for _, dataset := range createBenchmarkDatasets(b) {
		dataset := dataset
		b.Run(dataset.name, func(b *testing.B) {
			archive := filepath.Join(b.TempDir(), "bench.zip")
			if err := Compress(context.Background(), archive, benchmarkCompressOptions(dataset.root, runtime.GOMAXPROCS(0))); err != nil {
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
		})
	}
}

func createBenchmarkDatasets(b *testing.B) []benchmarkDataset {
	b.Helper()
	return []benchmarkDataset{
		{name: "many-small", root: createBenchmarkTree(b, "many-small", 1024, 4<<10)},
		{name: "medium-files", root: createBenchmarkTree(b, "medium-files", 64, 256<<10)},
		{name: "large-files", root: createBenchmarkTree(b, "large-files", 4, 16<<20)},
	}
}

func benchmarkCompressOptions(sourceRoot string, concurrency int) *CompressOptions {
	return &CompressOptions{
		Sources:     []string{sourceRoot},
		StripPrefix: filepath.Dir(sourceRoot),
		Recursive:   true,
		Concurrency: concurrency,
		Level:       -1,
	}
}

func createBenchmarkTree(b *testing.B, name string, files int, size int) string {
	b.Helper()

	root := filepath.Join(b.TempDir(), name)
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
		CompressOptions: benchmarkCompressOptions(sourceRoot, 1),
		tempRoot:        tempRoot,
		objects:         newObjectPool(),
	}
	cfg.setDefaults()

	return cfg.scan(context.Background(), func(spec entrySpec) error {
		obj, err := cfg.prepare(spec)
		if err != nil {
			return err
		}
		defer func() {
			_ = obj.Close()
			cfg.release(obj)
		}()
		if err := obj.Compress(); err != nil {
			return err
		}
		return obj.WriteTo(zw)
	})
}
