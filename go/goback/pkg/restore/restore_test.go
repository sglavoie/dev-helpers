package restore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/destinationlock"
	"golang.org/x/sys/unix"
)

func fixture(t *testing.T) Request {
	t.Helper()
	root := t.TempDir()
	req := Request{BackupRoot: filepath.Join(root, "backup"), BackupType: "daily", Path: "nested/notes.txt", Destination: filepath.Join(root, "recovery")}
	for _, path := range []string{filepath.Dir(backupFile(req)), req.Destination} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, backupFile(req), "saved content\x00\n")
	return req
}

func backupFile(req Request) string { return filepath.Join(req.BackupRoot, req.BackupType, req.Path) }

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func prepare(t *testing.T, req Request) Plan {
	t.Helper()
	plan, err := Prepare(req)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func assertEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatalf("expected empty directory %s, got %v (%v)", path, entries, err)
	}
}

func TestRestorePreservesContentPermissionsAndModificationTime(t *testing.T) {
	for _, kind := range []string{"daily", "weekly", "monthly"} {
		t.Run(kind, func(t *testing.T) {
			req := fixture(t)
			if kind != "daily" {
				if err := os.Rename(filepath.Join(req.BackupRoot, "daily"), filepath.Join(req.BackupRoot, kind)); err != nil {
					t.Fatal(err)
				}
				req.BackupType = kind
			}
			modified := time.Unix(1700000000, 123000000)
			if err := os.Chmod(backupFile(req), 0751); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(backupFile(req), modified, modified); err != nil {
				t.Fatal(err)
			}
			plan := prepare(t, req)
			if plan.Size != 15 || plan.Mode != 0751 || !plan.ModTime.Equal(modified) {
				t.Fatalf("incorrect preview: %+v", plan)
			}
			assertEmpty(t, req.Destination)
			if err := Restore(context.Background(), plan); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(plan.Destination)
			if err != nil || string(content) != "saved content\x00\n" {
				t.Fatalf("recovered content %q: %v", content, err)
			}
			info, err := os.Stat(plan.Destination)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0751 || !info.ModTime().Equal(modified) {
				t.Fatalf("recovered metadata: %v, %v", info.Mode(), info.ModTime())
			}
			entries, err := os.ReadDir(req.Destination)
			if err != nil || len(entries) != 1 || entries[0].Name() != "notes.txt" {
				t.Fatalf("unexpected recovery entries: %v (%v)", entries, err)
			}
			original, err := os.ReadFile(backupFile(req))
			if err != nil || string(original) != string(content) {
				t.Fatalf("backup changed: %q (%v)", original, err)
			}
		})
	}
}

func TestPrepareRejectsUnsafeOrUnsupportedPaths(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*testing.T, *Request)
		want string
	}{
		{"backup type", func(t *testing.T, r *Request) { r.BackupType = "mirror" }, "backup type"},
		{"empty file", func(t *testing.T, r *Request) { r.Path = "" }, "relative path"},
		{"absolute file", func(t *testing.T, r *Request) { r.Path = backupFile(*r) }, "relative path"},
		{"escape", func(t *testing.T, r *Request) { r.Path = "../../notes.txt" }, "relative path"},
		{"interior traversal", func(t *testing.T, r *Request) { r.Path = "nested/../nested/notes.txt" }, "must not contain"},
		{"directory root", func(t *testing.T, r *Request) { r.Path = "." }, "relative path"},
		{"directory", func(t *testing.T, r *Request) { r.Path = "nested" }, "regular file"},
		{"missing file", func(t *testing.T, r *Request) { r.Path = "missing.txt" }, "open backup file"},
		{"no backup root", func(t *testing.T, r *Request) { r.BackupRoot = "" }, "not configured"},
		{"no recovery directory", func(t *testing.T, r *Request) { r.Destination = "" }, "--to"},
		{"absent recovery directory", func(t *testing.T, r *Request) { r.Destination += "/missing" }, "existing recovery directory"},
		{"recovery is file", func(t *testing.T, r *Request) { r.Destination = backupFile(*r) }, "not a directory"},
		{"recovery inside backup", func(t *testing.T, r *Request) { r.Destination = filepath.Join(r.BackupRoot, "daily") }, "must not overlap"},
		{"recovery contains backup", func(t *testing.T, r *Request) { r.Destination = filepath.Dir(r.BackupRoot) }, "must not overlap"},
		{"symlink file", func(t *testing.T, r *Request) {
			target := backupFile(*r)
			r.Path = "link.txt"
			if err := os.Symlink(target, backupFile(*r)); err != nil {
				t.Fatal(err)
			}
		}, "symlinks are not supported"},
		{"symlink parent", func(t *testing.T, r *Request) {
			if err := os.Symlink(filepath.Dir(backupFile(*r)), filepath.Join(r.BackupRoot, "daily", "link")); err != nil {
				t.Fatal(err)
			}
			r.Path = "link/notes.txt"
		}, "symlinks are not supported"},
		{"symlink snapshot", func(t *testing.T, r *Request) {
			if err := os.Symlink(filepath.Join(r.BackupRoot, "daily"), filepath.Join(r.BackupRoot, "weekly")); err != nil {
				t.Fatal(err)
			}
			r.BackupType = "weekly"
		}, "symlinks are not supported"},
		{"recovery alias inside backup", func(t *testing.T, r *Request) {
			alias := filepath.Join(filepath.Dir(r.Destination), "alias")
			if err := os.Symlink(r.BackupRoot, alias); err != nil {
				t.Fatal(err)
			}
			r.Destination = alias
		}, "must not overlap"},
		{"fifo", func(t *testing.T, r *Request) {
			r.Path = "pipe"
			if err := unix.Mkfifo(backupFile(*r), 0600); err != nil {
				t.Fatal(err)
			}
		}, "regular file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := fixture(t)
			recovery := req.Destination
			tc.edit(t, &req)
			_, err := Prepare(req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			assertEmpty(t, recovery)
		})
	}
}

func TestRestoreNeverOverwritesAnExistingEntry(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "directory"} {
		for _, afterPreview := range []bool{false, true} {
			t.Run(kind+"/after-preview="+strconv.FormatBool(afterPreview), func(t *testing.T) {
				req := fixture(t)
				plan := prepare(t, req)
				target := plan.Destination
				switch kind {
				case "file":
					write(t, target, "keep existing")
				case "symlink":
					if err := os.Symlink("missing-target", target); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(target, 0700); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				if afterPreview {
					err = Restore(context.Background(), plan)
				} else {
					_, err = Prepare(req)
				}
				if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
					t.Fatalf("error = %v", err)
				}
				if kind == "file" {
					content, _ := os.ReadFile(target)
					if string(content) != "keep existing" {
						t.Fatalf("existing content changed: %q", content)
					}
				} else if kind == "symlink" {
					link, err := os.Readlink(target)
					if err != nil || link != "missing-target" {
						t.Fatalf("existing link changed: %q (%v)", link, err)
					}
				} else {
					assertEmpty(t, target)
				}
			})
		}
	}
}

func TestRestoreStopsWhenTheBackupChangesAfterPreview(t *testing.T) {
	req := fixture(t)
	plan := prepare(t, req)
	write(t, backupFile(req), "changed backup")
	if err := Restore(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "changed since preview") {
		t.Fatalf("error = %v", err)
	}
	assertEmpty(t, req.Destination)
}

func TestRestoreCoordinatesWithBackupAndRecoveryWriters(t *testing.T) {
	for _, which := range []string{"backup", "recovery"} {
		t.Run(which, func(t *testing.T) {
			req := fixture(t)
			plan := prepare(t, req)
			path := req.BackupRoot
			if which == "recovery" {
				path = req.Destination
			}
			_, release, err := destinationlock.Acquire(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(release)
			if err := Restore(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "another goback operation") {
				t.Fatalf("error = %v", err)
			}
			assertEmpty(t, req.Destination)
			release()
			if err := Restore(context.Background(), plan); err != nil {
				t.Fatalf("retry after lock release: %v", err)
			}
		})
	}
}

func TestRestoreCleansUpAfterCopyFailureCorruptionAndCancellation(t *testing.T) {
	for _, action := range []string{"copy failure", "stage corruption", "backup changed", "cancel", "target appeared", "directory replaced"} {
		t.Run(action, func(t *testing.T) {
			req := fixture(t)
			plan := prepare(t, req)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			copyFile := func(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
				n, err := copyContext(ctx, dst, src)
				if err != nil {
					return n, err
				}
				switch action {
				case "copy failure":
					return n, errors.New("simulated I/O failure")
				case "stage corruption":
					entries, err := os.ReadDir(req.Destination)
					if err != nil || len(entries) != 1 {
						t.Fatalf("staging entries: %v (%v)", entries, err)
					}
					write(t, filepath.Join(req.Destination, entries[0].Name()), "corrupt content")
				case "backup changed":
					write(t, backupFile(req), "updated backup")
				case "cancel":
					cancel()
				case "target appeared":
					write(t, plan.Destination, "keep concurrent file")
				case "directory replaced":
					if err := os.Rename(req.Destination, req.Destination+"-old"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(req.Destination, 0700); err != nil {
						t.Fatal(err)
					}
				}
				return n, nil
			}
			err := restore(ctx, plan, copyFile)
			if err == nil {
				t.Fatal("unsafe restore succeeded")
			}
			if (action == "stage corruption" || action == "backup changed") && !strings.Contains(err.Error(), "content verification failed") {
				t.Fatalf("verification did not detect changed bytes: %v", err)
			}
			if action == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation error = %v", err)
			}
			if action == "target appeared" {
				content, _ := os.ReadFile(plan.Destination)
				if string(content) != "keep concurrent file" {
					t.Fatalf("concurrent file overwritten: %q", content)
				}
				entries, _ := os.ReadDir(req.Destination)
				if len(entries) != 1 {
					t.Fatalf("staging file was not removed: %v", entries)
				}
			} else {
				assertEmpty(t, req.Destination)
			}
			if action == "directory replaced" {
				assertEmpty(t, req.Destination+"-old")
			}
		})
	}
}

func TestRestoreHandlesEmptyFilesAndLiteralFilenames(t *testing.T) {
	for _, name := range []string{"empty", "-leading-dash", "quotes'and\"more", "a file with spaces", "line\nbreak", "$(touch should-not-exist)*?"} {
		t.Run(name, func(t *testing.T) {
			req := fixture(t)
			req.Path = name
			write(t, backupFile(req), "")
			plan := prepare(t, req)
			if err := Restore(context.Background(), plan); err != nil {
				t.Fatal(err)
			}
			entries, _ := os.ReadDir(req.Destination)
			if len(entries) != 1 || entries[0].Name() != name {
				t.Fatalf("literal filename was not preserved: %v", entries)
			}
		})
	}
}

func TestRestoreWithAlreadyCancelledContextWritesNothing(t *testing.T) {
	req := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Restore(ctx, prepare(t, req)); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	assertEmpty(t, req.Destination)
}
