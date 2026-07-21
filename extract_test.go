package pzip

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTruncatesExistingFile(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.Mkdir(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("short"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(root, "out.zip")
	err := Compress(context.Background(), dst, &CompressOptions{
		Sources:     []string{filepath.Join(src, "file.txt")},
		StripPrefix: src,
		Concurrency: 1,
		Level:       -1,
	})
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "out")
	if err := os.Mkdir(out, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "file.txt"), []byte("short with stale suffix"), 0644); err != nil {
		t.Fatal(err)
	}

	err = Extract(context.Background(), dst, &ExtractOptions{
		Destination: out,
		Concurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(out, "file.txt"), "short")
}

func TestExtractFilesAndFilter(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skip.log"), []byte("skip"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "keep.md"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(root, "out.zip")
	err := Compress(context.Background(), archive, &CompressOptions{
		Sources:     []string{src},
		StripPrefix: filepath.Dir(src),
		Recursive:   true,
		Concurrency: 2,
		Level:       -1,
	})
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "out")
	err = Extract(context.Background(), archive, &ExtractOptions{
		Destination: out,
		Concurrency: 2,
		Filter:      NewFilter([]string{"*.txt", "*.md"}, []string{"*.log"}),
	})
	if err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(out, "src", "keep.txt"), "keep")
	assertFileContent(t, filepath.Join(out, "src", "nested", "keep.md"), "nested")
	if _, err := os.Stat(filepath.Join(out, "src", "skip.log")); !os.IsNotExist(err) {
		t.Fatalf("skip.log exists or stat failed unexpectedly: %v", err)
	}
}

func TestExtractRejectsZipSlip(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "slip.zip")

	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("evil")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "out")
	err = Extract(context.Background(), archive, &ExtractOptions{
		Destination: out,
		Concurrency: 1,
	})
	if err == nil {
		t.Fatal("expected zip slip error")
	}
	if _, statErr := os.Stat(filepath.Join(root, "evil.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("evil.txt exists or stat failed unexpectedly: %v", statErr)
	}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
