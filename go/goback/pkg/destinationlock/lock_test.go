package destinationlock

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOverlappingDestinationsExcludeEachOther(t *testing.T) {
	for _, paths := range [][2]string{{"/backup", "/backup"}, {"/backup", "/backup/daily"}, {"/backup/daily", "/backup"}} {
		t.Run(paths[0]+"-"+paths[1], func(t *testing.T) {
			dir := t.TempDir()
			release, err := LockResolved(dir, paths[0])
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if other, err := LockResolved(dir, paths[1]); err == nil {
				other()
				t.Fatal("overlapping writer acquired lock")
			}
			release()
			other, err := LockResolved(dir, paths[1])
			if err != nil {
				t.Fatalf("failed lock attempt leaked locks: %v", err)
			}
			other()
		})
	}
}

func TestSiblingsCanRunIndependently(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{"/backup/a", "/backup/b", "/backup-other"} {
		release, err := LockResolved(dir, path)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
	}
}

func TestAliasesAndSequentialContextReuse(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	ctx, release, err := Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, other, err := Acquire(context.Background(), alias); err == nil {
		other()
		t.Fatal("alias bypassed lock")
	}
	if _, nested, err := Acquire(ctx, alias); err != nil {
		t.Fatal(err)
	} else {
		nested()
	}
	if _, other, err := Acquire(context.Background(), root); err == nil {
		other()
		t.Fatal("nested release gave up outer lock")
	}
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root + "-old") })
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if _, other, err := Acquire(ctx, root); err == nil {
		other()
		t.Fatal("replaced directory reused held lock")
	}
}
