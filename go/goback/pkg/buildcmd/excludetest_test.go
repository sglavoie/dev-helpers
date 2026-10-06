package buildcmd

import (
	"testing"
)

func TestParseListOnly(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
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
			name:    "unrecognized lines fail closed",
			wantErr: true,
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
			got, err := parseListOnly(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected invalid listing error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
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
