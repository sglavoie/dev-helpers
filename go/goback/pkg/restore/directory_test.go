package restore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/destinationlock"
	"golang.org/x/sys/unix"
)

func directoryFixture(t *testing.T) Request {
	t.Helper()
	req := fixture(t)
	req.Path, req.Recursive = "nested", true
	root := backupFile(req)
	for _, path := range []string{"deeper/sub", "empty"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0750); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		".hidden": "hidden", "deeper/notes.txt": "another version",
		"deeper/empty-file": "", "deeper/sub/-odd'\" \n.txt": "binary\x00data",
	} {
		write(t, filepath.Join(root, path), content)
	}
	// Read-only source directories must remain read-only after recovery, but
	// staging still needs to be writable until all their children are copied.
	modified := time.Unix(1700000000, 123000000)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		mode := fs.FileMode(0440)
		if entry.IsDir() {
			mode = 0550
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
		return os.Chtimes(path, modified, modified)
	}); err != nil {
		t.Fatal(err)
	}
	// Give testing's cleanup permission to remove the original read-only tree.
	t.Cleanup(func() {
		_ = filepath.WalkDir(req.BackupRoot, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				_ = os.Chmod(path, 0700)
			}
			return nil
		})
		_ = filepath.WalkDir(req.Destination, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				_ = os.Chmod(path, 0700)
			}
			return nil
		})
	})
	return req
}

type recoveredEntry struct {
	directory bool
	mode      fs.FileMode
	modified  time.Time
	content   string
}

func directorySnapshot(t *testing.T, root string) map[string]recoveredEntry {
	t.Helper()
	result := map[string]recoveredEntry{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := recoveredEntry{directory: info.IsDir(), mode: info.Mode().Perm(), modified: info.ModTime()}
		if !info.IsDir() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.content = string(content)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[relative] = value
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRecursiveRestorePreservesStructureContentAndMetadata(t *testing.T) {
	for _, kind := range []string{"daily", "weekly", "monthly"} {
		t.Run(kind, func(t *testing.T) {
			req := directoryFixture(t)
			if kind != "daily" {
				if err := os.Rename(filepath.Join(req.BackupRoot, "daily"), filepath.Join(req.BackupRoot, kind)); err != nil {
					t.Fatal(err)
				}
				req.BackupType = kind
			}
			before := directorySnapshot(t, backupFile(req))
			plan := prepare(t, req)
			var total int64
			for _, entry := range before {
				total += int64(len(entry.content))
			}
			if !plan.IsDir || plan.Files != 5 || plan.Directories != 4 || plan.Size != total {
				t.Fatalf("incorrect directory preview: %+v", plan)
			}
			assertEmpty(t, req.Destination)
			if err := Restore(context.Background(), plan); err != nil {
				t.Fatal(err)
			}
			if got := directorySnapshot(t, plan.Destination); !reflect.DeepEqual(before, got) {
				t.Fatalf("recovered tree differs:\nwant: %#v\ngot:  %#v", before, got)
			}
			if !reflect.DeepEqual(before, directorySnapshot(t, backupFile(req))) {
				t.Fatal("restore changed the backup")
			}
			entries, err := os.ReadDir(req.Destination)
			if err != nil || len(entries) != 1 || entries[0].Name() != "nested" {
				t.Fatalf("staging directories left behind: %v (%v)", entries, err)
			}
		})
	}
}

func TestRecursiveRestoreHandlesAnEmptyDirectory(t *testing.T) {
	req := directoryFixture(t)
	req.Path += "/empty/"
	plan := prepare(t, req)
	if plan.Files != 0 || plan.Directories != 1 || plan.Size != 0 {
		t.Fatalf("empty directory preview: %+v", plan)
	}
	if err := Restore(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	assertEmpty(t, plan.Destination)
	if got, want := directorySnapshot(t, plan.Destination), directorySnapshot(t, backupFile(req)); !reflect.DeepEqual(got, want) {
		t.Fatalf("empty directory metadata: got %v, want %v", got, want)
	}
}

func TestRecursiveFlagStillAllowsSingleFileRecovery(t *testing.T) {
	req := fixture(t)
	req.Recursive = true
	if err := Restore(context.Background(), prepare(t, req)); err != nil {
		t.Fatal(err)
	}
}

func TestRecursivePrepareRejectsUnsupportedDescendantsBeforeWriting(t *testing.T) {
	for _, kind := range []string{"symlink file", "symlink directory", "dangling symlink", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			req := directoryFixture(t)
			parent := filepath.Join(backupFile(req), "deeper")
			if err := os.Chmod(parent, 0750); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(parent, "unsupported")
			var err error
			switch kind {
			case "symlink file":
				err = os.Symlink(filepath.Join(backupFile(req), "notes.txt"), path)
			case "symlink directory":
				err = os.Symlink(req.Destination, path)
			case "dangling symlink":
				err = os.Symlink("absent", path)
			case "fifo":
				err = unix.Mkfifo(path, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = Prepare(req)
			if err == nil || !strings.Contains(err.Error(), "unsupported") || !strings.Contains(err.Error(), "regular files and directories") {
				t.Fatalf("unsupported descendant accepted: %v", err)
			}
			assertEmpty(t, req.Destination)
		})
	}
}

func TestRecursiveRestoreDetectsChangesToDescendantsAfterPreview(t *testing.T) {
	req := directoryFixture(t)
	plan := prepare(t, req)
	path := filepath.Join(backupFile(req), "deeper/notes.txt")
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	write(t, path, "updated backup")
	if err := Restore(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "changed since preview") {
		t.Fatalf("changed descendant accepted: %v", err)
	}
	assertEmpty(t, req.Destination)
}

func TestRecursiveRestoreCleansUpOnErrorsAndConcurrentChanges(t *testing.T) {
	for _, action := range []string{
		"copy failure", "cancel", "stage corruption", "backup corruption", "extra staged file",
		"target file appeared", "target empty directory appeared", "target symlink appeared",
		"recovery directory replaced", "backup file became symlink", "backup gained file",
	} {
		t.Run(action, func(t *testing.T) {
			req := directoryFixture(t)
			plan := prepare(t, req)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			copied := 0
			copyFile := func(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
				n, err := copyContext(ctx, dst, src)
				if err != nil {
					return n, err
				}
				copied++
				// Inject after copying the first file, except corruption of that
				// early file, which happens after all the other files were copied.
				if action == "stage corruption" || action == "backup corruption" {
					if copied != plan.Files {
						return n, nil
					}
				} else if copied != 1 {
					return n, nil
				}
				switch action {
				case "copy failure":
					return n, errors.New("simulated I/O error")
				case "cancel":
					cancel()
				case "stage corruption", "extra staged file":
					entries, err := os.ReadDir(req.Destination)
					if err != nil || len(entries) != 1 {
						t.Fatalf("staging entries: %v (%v)", entries, err)
					}
					path := ".hidden"
					if action == "extra staged file" {
						path = "unexpected"
					}
					write(t, filepath.Join(req.Destination, entries[0].Name(), path), "broken")
				case "backup corruption":
					path := filepath.Join(backupFile(req), ".hidden")
					info, err := os.Stat(path)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(path, 0640); err != nil {
						t.Fatal(err)
					}
					write(t, path, "broken") // Same length, inode, permissions and mtime.
					if err := os.Chmod(path, info.Mode().Perm()); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
						t.Fatal(err)
					}
				case "target file appeared":
					write(t, plan.Destination, "keep concurrent file")
				case "target empty directory appeared":
					if err := os.Mkdir(plan.Destination, 0700); err != nil {
						t.Fatal(err)
					}
				case "target symlink appeared":
					if err := os.Symlink("missing", plan.Destination); err != nil {
						t.Fatal(err)
					}
				case "recovery directory replaced":
					if err := os.Rename(req.Destination, req.Destination+"-old"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(req.Destination, 0700); err != nil {
						t.Fatal(err)
					}
				case "backup file became symlink":
					parent := filepath.Join(backupFile(req), "deeper")
					if err := os.Chmod(parent, 0750); err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(parent, "notes.txt")
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(filepath.Join(backupFile(req), "notes.txt"), path); err != nil {
						t.Fatal(err)
					}
				case "backup gained file":
					if err := os.Chmod(backupFile(req), 0750); err != nil {
						t.Fatal(err)
					}
					write(t, filepath.Join(backupFile(req), "new-file"), "new content")
				}
				return n, nil
			}
			err := restore(ctx, plan, copyFile)
			if err == nil {
				t.Fatal("unsafe recursive restore succeeded")
			}
			if strings.Contains(err.Error(), "remove restore staging directory") {
				t.Fatalf("cleanup failed: %v", err)
			}
			if strings.Contains(action, "corruption") && !strings.Contains(err.Error(), "content verification failed") {
				t.Fatalf("verification missed changed bytes: %v", err)
			}
			if action == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation error: %v", err)
			}
			switch action {
			case "target file appeared":
				content, _ := os.ReadFile(plan.Destination)
				if string(content) != "keep concurrent file" {
					t.Fatalf("concurrent file changed: %q", content)
				}
			case "target empty directory appeared":
				assertEmpty(t, plan.Destination)
			case "target symlink appeared":
				link, err := os.Readlink(plan.Destination)
				if err != nil || link != "missing" {
					t.Fatalf("concurrent symlink changed: %q (%v)", link, err)
				}
			default:
				assertEmpty(t, req.Destination)
			}
			entries, _ := os.ReadDir(req.Destination)
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".goback-restore-") {
					t.Fatalf("staging directory left behind: %s", entry.Name())
				}
			}
			if action == "recovery directory replaced" {
				assertEmpty(t, req.Destination+"-old")
			}
		})
	}
}

func TestRecursiveRestoreCoordinatesWithBackupAndRecoveryWriters(t *testing.T) {
	for _, path := range []string{"backup", "recovery"} {
		t.Run(path, func(t *testing.T) {
			req := directoryFixture(t)
			plan := prepare(t, req)
			locked := req.BackupRoot
			if path == "recovery" {
				locked = req.Destination
			}
			_, release, err := destinationlock.Acquire(context.Background(), locked)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(release)
			if err := Restore(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "another goback operation") {
				t.Fatalf("overlapping writer was not refused: %v", err)
			}
			assertEmpty(t, req.Destination)
		})
	}
}

func TestPublishDirectoryAtomicallyRefusesExistingEntries(t *testing.T) {
	for _, kind := range []string{"file", "empty directory", "nonempty directory", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			parent, err := os.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer parent.Close()
			stage, err := makeDirectory(parent, "stage")
			if err != nil {
				t.Fatal(err)
			}
			stage.Close()
			target := filepath.Join(root, "target")
			var stat unix.Stat_t
			switch kind {
			case "file":
				fd, err := unix.Openat(int(parent.Fd()), "target", unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				file := os.NewFile(uintptr(fd), target)
				_, err = file.WriteString("keep")
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			case "empty directory", "nonempty directory":
				dir, err := makeDirectory(parent, "target")
				if err != nil {
					t.Fatal(err)
				}
				if kind == "nonempty directory" {
					fd, err := unix.Openat(int(dir.Fd()), "keep", unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY, 0600)
					if err != nil {
						t.Fatal(err)
					}
					unix.Close(fd)
				}
				dir.Close()
			case "dangling symlink":
				if err := unix.Symlinkat("missing", int(parent.Fd()), "target"); err != nil {
					t.Fatal(err)
				}
			}
			if err := unix.Fstatat(int(parent.Fd()), "target", &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
				t.Fatal(err)
			}
			if err := publishDirectory(int(parent.Fd()), "stage", int(parent.Fd()), "target"); err == nil {
				t.Fatal("publication overwrote existing entry")
			}
			var after unix.Stat_t
			if err := unix.Fstatat(int(parent.Fd()), "target", &after, unix.AT_SYMLINK_NOFOLLOW); err != nil || stat.Ino != after.Ino || stat.Mode != after.Mode {
				t.Fatalf("existing entry changed: %v", err)
			}
		})
	}
}

func TestRecursivePreparationCanBeCancelledWithoutWrites(t *testing.T) {
	req := directoryFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PrepareContext(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preparation: %v", err)
	}
	assertEmpty(t, req.Destination)
}
