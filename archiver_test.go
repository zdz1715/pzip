package pzip

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
)

func TestArchiver_ArchiveAll(t *testing.T) {

	root := "testdata/no-symlink"

	err := os.Chdir(root)
	if err != nil {
		t.Fatal(err)
	}

	err = Archive(context.Background(), "../no-symlink-test.zip", &ArchiveOptions{
		Files: []string{
			".",
		},
		//NewCompressor: func(w io.Writer, level int) (flate.Writer, error) {
		//	return flate.NewWriter(w, level)
		//},
		SkipPath: SkipPath{
			//Excludes: []string{"**/*.zip"},
		},
		Recurse:     true,
		Concurrency: runtime.GOMAXPROCS(0),
		Level:       -1,
		After: func(hdr *FileHeader) {
			md := "stored"
			if hdr.Method == zip.Deflate {
				md = "deflated"
			}
			_, _ = fmt.Fprintf(os.Stdout, "  adding: %s (%s)\n", hdr.Name, md)
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestArchiver_Archive(t *testing.T) {

	root := "testdata"

	err := os.Chdir(root)
	if err != nil {
		t.Fatal(err)
	}

	err = Archive(context.Background(), "no-symlink-test.zip", &ArchiveOptions{
		Files: []string{
			"no-symlink",
			".gitignore",
		},
		SkipPath: SkipPath{
			//Excludes: []string{"**/*.zip"},
		},
		Recurse:     true,
		Concurrency: runtime.GOMAXPROCS(0),
		Level:       -1,
		After: func(hdr *FileHeader) {
			md := "stored"
			if hdr.Method == zip.Deflate {
				md = "deflated"
			}
			_, _ = fmt.Fprintf(os.Stdout, "  adding: %s (%s)\n", hdr.Name, md)
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestArchiver_ArchiveSymlink(t *testing.T) {
	root := "testdata"

	err := os.Chdir(root)
	if err != nil {
		t.Fatal(err)
	}

	err = Archive(context.Background(), "symlink-test.zip", &ArchiveOptions{
		Files: []string{
			"symlink",
			".gitignore",
		},
		//Dereference: true,
		Recurse:     true,
		Concurrency: 1,
		Level:       -1,
		After: func(hdr *FileHeader) {
			md := "stored"
			if hdr.Method == zip.Deflate {
				md = "deflated"
			}
			_, _ = fmt.Fprintf(os.Stdout, "  adding: %s (%s)\n", hdr.Name, md)
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestExtract(t *testing.T) {
	root := "testdata"

	err := os.Chdir(root)
	if err != nil {
		t.Fatal(err)
	}

	err = Extract(context.Background(), "no-symlink-test.zip", &ExtractOptions{
		Concurrency: runtime.GOMAXPROCS(0),
		Before: func(path string, r *ReadCloser) {
			_, _ = fmt.Fprintf(os.Stdout, "Archive: %s\n", path)
			_, _ = fmt.Fprintf(os.Stdout, "Comment: %s\n", r.Comment)
		},
		After: func(f *File, target *ExtractTarget) {
			md := "extracting"
			if f.FileInfo().IsDir() {
				md = "creating"
			}

			if f.Method == zip.Deflate {
				md = "inflating"
			}

			if IsSymlink(f.Mode()) {
				md = "symlinking"
			}

			_, _ = fmt.Fprintf(os.Stdout, "  %s: %s\n", md, target)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

}
