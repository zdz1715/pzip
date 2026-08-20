package pzip

import "testing"

func TestStripArchivePath(t *testing.T) {
	tests := []struct {
		name       string
		archive    string
		prefix     string
		components int
		want       string
		wantOK     bool
	}{
		{name: "prefix", archive: "release/bin/app", prefix: "release", want: "bin/app", wantOK: true},
		{name: "prefix boundary", archive: "release-old/bin/app", prefix: "release", wantOK: false},
		{name: "prefix entry", archive: "release", prefix: "release", wantOK: false},
		{name: "components", archive: "release/bin/app", components: 2, want: "app", wantOK: true},
		{name: "components too shallow", archive: "release", components: 1, wantOK: false},
		{name: "clean", archive: "release//bin/../app", want: "release/app", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := stripArchivePath(tt.archive, tt.prefix, tt.components)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("stripArchivePath() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestValidateStripOptions(t *testing.T) {
	if err := validateStripOptions("release", 1); err == nil {
		t.Fatal("expected mutually exclusive options error")
	}
	if err := validateStripOptions("", -1); err == nil {
		t.Fatal("expected negative strip components error")
	}
	if err := validateStripOptions("release", 0); err != nil {
		t.Fatalf("unexpected strip prefix error: %v", err)
	}
	if err := validateStripOptions("", 1); err != nil {
		t.Fatalf("unexpected strip components error: %v", err)
	}
}
