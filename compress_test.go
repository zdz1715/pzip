package pzip

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCompressStripPrefixAddPrefixAndComment(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b", "c"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "b", "c", "one.txt"), []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "b", "c", "two.txt"), []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(root, "out.zip")
	err := Compress(context.Background(), dst, &CompressOptions{
		Sources:     []string{filepath.Join(root, "a", "b", "c")},
		StripPrefix: filepath.Join(root, "a", "b"),
		AddPrefix:   "release",
		Recursive:   true,
		Concurrency: 2,
		Level:       -1,
		Comment:     "hello comment",
	})
	if err != nil {
		t.Fatal(err)
	}

	r, err := zip.OpenReader(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if r.Comment != "hello comment" {
		t.Fatalf("comment = %q, want %q", r.Comment, "hello comment")
	}

	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	slices.Sort(names)
	want := []string{"release/c/", "release/c/one.txt", "release/c/two.txt"}
	if !slices.Equal(names, want) {
		t.Fatalf("entries = %v, want %v", names, want)
	}
}
