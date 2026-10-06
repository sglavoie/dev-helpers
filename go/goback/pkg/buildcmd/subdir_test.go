package buildcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScopedExclusionsKeepBackupFilterRoot(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
	root := t.TempDir()
	for _, name := range []string{"Documents/private.txt", "Documents/keep.txt", "Documents/nested/private.txt", "Elsewhere/private.txt"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, subdir string
		filters      []string
		depth        int
		want         []string
	}{
		{"anchored", "Documents", []string{"--exclude=/Documents/private.txt"}, 0, []string{"Documents/private.txt"}},
		{"absolute subdir", filepath.Join(root, "Documents"), []string{"--exclude=/Documents/private.txt"}, 0, []string{"Documents/private.txt"}},
		{"outside matches hidden", "Documents", []string{"--exclude=private.txt"}, 0, []string{"Documents/nested/private.txt", "Documents/private.txt"}},
		{"depth from subdir", "Documents", []string{"--exclude=private.txt"}, 1, []string{"Documents/private.txt"}},
		{"excluded ancestor", "Documents/nested", []string{"--exclude=/Documents/"}, 0, []string{"Documents/nested/"}},
		{"includes retain root", "Documents", []string{"--include=/Documents/", "--include=/Documents/keep.txt", "--exclude=*"}, 0, []string{"Documents/nested/", "Documents/private.txt"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := findExcludedInSubdir(root, tc.filters, tc.subdir, tc.depth)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, subdir := range []string{"../outside", filepath.Dir(root), "missing"} {
		if _, err := findExcludedInSubdir(root, nil, subdir, 0); err == nil {
			t.Fatalf("accepted invalid subdir %q", subdir)
		}
	}
}
