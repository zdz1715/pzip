package pzip

import (
	"path/filepath"
	"testing"
)

func TestSkipPath_Skip(t *testing.T) {
	tests := []struct {
		path string
		skip SkipPath
		want bool
	}{
		{
			path: "test.zip",
			skip: SkipPath{
				Includes: []string{"*.zip"},
			},
			want: false,
		},
		{
			path: "test.zip",
			skip: SkipPath{
				Includes: []string{filepath.Join("**", "*.zip")},
			},
			want: false,
		},
		{
			path: filepath.Join("1", "2", "test.zip"),
			skip: SkipPath{
				Includes: []string{"*.zip"},
			},
			want: true,
		},
		{
			path: filepath.Join("1", "2", "test.zip"),
			skip: SkipPath{
				Includes: []string{filepath.Join("**", "*.zip")},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		target := tt.skip.Skip(tt.path)
		if target != tt.want {
			t.Errorf("Skip(%v) = %v, want %v", tt.path, target, tt.want)
		}
	}
}
