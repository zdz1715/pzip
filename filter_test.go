package pzip

import "testing"

func TestDefaultMatcherInfoZIPSemantics(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		entry   string
		want    bool
	}{
		{name: "star matches directories", pattern: "*.md", entry: "docs/guide/readme.md", want: true},
		{name: "question mark matches separator", pattern: "docs?readme.md", entry: "docs/readme.md", want: true},
		{name: "star spans multiple directories", pattern: "docs/*.md", entry: "docs/guide/readme.md", want: true},
		{name: "double star has no special directory semantics", pattern: "**/*.md", entry: "readme.md", want: false},
		{name: "double star matches nested entry", pattern: "**/*.md", entry: "docs/readme.md", want: true},
		{name: "bang negates character class", pattern: "[!a].md", entry: "b.md", want: true},
		{name: "caret negates character class", pattern: "[^a].md", entry: "b.md", want: true},
		{name: "caret class rejects excluded character", pattern: "[^a].md", entry: "a.md", want: false},
		{name: "brace alternatives are supported", pattern: "*.{go,md}", entry: "docs/readme.md", want: true},
		{name: "escaped star is literal", pattern: `\*.md`, entry: "*.md", want: true},
		{name: "invalid pattern does not match", pattern: "[", entry: "readme.md", want: false},
	}

	matcher := DefaultMatcher{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matcher.Match(tt.pattern, tt.entry); got != tt.want {
				t.Fatalf("Match(%q, %q) = %v, want %v", tt.pattern, tt.entry, got, tt.want)
			}
		})
	}
}

func TestFilterInfoZIPSemantics(t *testing.T) {
	filter := NewFilter([]string{"*.md"}, []string{"*/vendor/*"})

	for _, entry := range []string{"README.md", "docs/README.md", "docs/guide/README.md"} {
		if !filter.Accept(entry) {
			t.Errorf("Accept(%q) = false, want true", entry)
		}
	}
	for _, entry := range []string{"README.txt", "src/vendor/README.md"} {
		if filter.Accept(entry) {
			t.Errorf("Accept(%q) = true, want false", entry)
		}
	}
}

func BenchmarkFilterAccept(b *testing.B) {
	filter := NewFilter(
		[]string{"*.go", "*.md", "docs/{guide,reference}/*.html"},
		[]string{"*/vendor/*", "*_test.go", "*/testdata/*"},
	)
	names := []string{
		"main.go",
		"cmd/pzip/main.go",
		"docs/guide/getting-started.html",
		"internal/archive/reader_test.go",
		"vendor/example.com/module/file.go",
		"assets/application.css",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filter.Accept(names[i%len(names)])
	}
}
