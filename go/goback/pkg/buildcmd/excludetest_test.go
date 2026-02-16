package buildcmd

import (
	"testing"
)

func TestCollapseToTopLevel(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "empty input",
			in:   nil,
			want: nil,
		},
		{
			name: "single file",
			in:   []string{"README.md"},
			want: []string{"README.md"},
		},
		{
			name: "single directory",
			in:   []string{".cache/"},
			want: []string{".cache/"},
		},
		{
			name: "directory with children",
			in: []string{
				".cache/",
				".cache/foo",
				".cache/bar/",
				".cache/bar/baz.txt",
			},
			want: []string{".cache/"},
		},
		{
			name: "multiple independent directories with children",
			in: []string{
				".cache/",
				".cache/a",
				".cache/b",
				"node_modules/",
				"node_modules/pkg/",
				"node_modules/pkg/index.js",
			},
			want: []string{".cache/", "node_modules/"},
		},
		{
			name: "file not child of preceding directory",
			in: []string{
				".cache/",
				".cache/foo",
				"readme.txt",
			},
			want: []string{".cache/", "readme.txt"},
		},
		{
			name: "path without trailing slash still collapses children",
			in: []string{
				".cache",
				".cache/",
				".cache/foo",
				".cachedata",
			},
			// ".cache" appears first; ".cache/" and ".cache/foo" start with
			// ".cache/" so they are children. ".cachedata" does NOT start
			// with ".cache/" so it survives.
			want: []string{".cache", ".cachedata"},
		},
		{
			name: "nested directories only top ancestor survives",
			in: []string{
				"a/",
				"a/b/",
				"a/b/c",
				"a/b/d/",
				"a/b/d/e",
			},
			want: []string{"a/"},
		},
		{
			name: "interleaved files and directories",
			in: []string{
				"a.txt",
				"b/",
				"b/x",
				"c.log",
				"d/",
				"d/y/",
				"d/y/z",
			},
			want: []string{"a.txt", "b/", "c.log", "d/"},
		},
		{
			name: "children without parent directory collapse to first component",
			in: []string{
				".DS_Store",
				".gallery/.DS_Store",
				"CacheClip/.DS_Store",
				"CacheClip/audio/foo",
				"CacheClip/audio/bar",
				"TV/.DS_Store",
				"TV/Media.localized/.DS_Store",
				"desktop.ini",
			},
			want: []string{".DS_Store", ".gallery/", "CacheClip/", "TV/", "desktop.ini"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := collapseToTopLevel(tc.in)
			if !slicesEqual(got, tc.want) {
				t.Errorf("collapseToTopLevel(%v)\n  got  %v\n  want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseListOnly(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "empty string",
			in:   "",
			want: nil,
		},
		{
			name: "single file",
			in:   "drwxr-xr-x       4,096 2024/01/15 10:30:00 Documents/",
			want: []string{"Documents/"},
		},
		{
			name: "filename with spaces",
			in:   "-rw-r--r--       1,234 2024/01/15 10:30:00 My Documents/some file.txt",
			want: []string{"My Documents/some file.txt"},
		},
		{
			name: "root entries are skipped",
			in: "drwxr-xr-x       4,096 2024/01/15 10:30:00 .\n" +
				"drwxr-xr-x       4,096 2024/01/15 10:30:00 ./\n" +
				"-rw-r--r--         100 2024/01/15 10:30:00 file.txt",
			want: []string{"file.txt"},
		},
		{
			name: "lines with fewer than 5 fields are skipped",
			in: "short line\n" +
				"also too short\n" +
				"-rw-r--r--       1,234 2024/01/15 10:30:00 valid.txt",
			want: []string{"valid.txt"},
		},
		{
			name: "multiple entries",
			in: "drwxr-xr-x       4,096 2024/01/15 10:30:00 dir/\n" +
				"-rw-r--r--         512 2024/02/20 08:15:30 dir/file.txt\n" +
				"-rw-r--r--       2,048 2024/03/01 12:00:00 another.log",
			want: []string{"dir/", "dir/file.txt", "another.log"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseListOnly(tc.in)
			if !slicesEqual(got, tc.want) {
				t.Errorf("parseListOnly(%q)\n  got  %v\n  want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsDSStore(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".DS_Store", true},
		{"foo/.DS_Store", true},
		{"foo/bar/.DS_Store", true},
		{"foo/.DS_Store_backup", false},
		{".DS_Storefoo", false},
		{"DS_Store", false},
		{"foo/bar/baz", false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := isDSStore(tc.path)
			if got != tc.want {
				t.Errorf("isDSStore(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestDepthExcludePattern(t *testing.T) {
	tests := []struct {
		depth int
		want  string
	}{
		{1, "*/*"},
		{2, "*/*/*"},
		{3, "*/*/*/*"},
	}

	for _, tc := range tests {
		got := depthExcludePattern(tc.depth)
		if got != tc.want {
			t.Errorf("depthExcludePattern(%d) = %q, want %q", tc.depth, got, tc.want)
		}
	}
}

// slicesEqual compares two string slices, treating nil and empty as equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
