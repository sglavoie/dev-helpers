package cmd

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Compare relative structure, bytes, permissions and mtimes. Directory sizes
// describe filesystem storage and need not match across copied directories.
func restoredDirectoryTree(t *testing.T, root string) map[string]restoreTreeEntry {
	t.Helper()
	result := map[string]restoreTreeEntry{}
	for path, entry := range restoreTree(t, root) {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		if entry.mode.IsDir() {
			entry.size = 0
		}
		result[relative] = entry
	}
	return result
}

func TestRestoreCLIBacksUpDeletesAndRecoversADirectoryTree(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is required for the backup-to-restore integration test")
	}
	for _, kind := range []string{"daily", "weekly", "monthly"} {
		t.Run(kind, func(t *testing.T) {
			f := restoreCLIFixture(t)
			project := filepath.Join(f.source, "Projects", "my-app")
			for _, path := range []string{"src/nested", "empty"} {
				if err := os.MkdirAll(filepath.Join(project, path), 0750); err != nil {
					t.Fatal(err)
				}
			}
			for path, content := range map[string]string{
				"notes.txt": "project notes\n", "src/notes.txt": "different notes\x00\n",
				"src/nested/quotes' and\" newline\n.txt": "literal filename", ".hidden": "",
			} {
				if err := os.WriteFile(filepath.Join(project, path), []byte(content), 0740); err != nil {
					t.Fatal(err)
				}
			}
			modified := time.Unix(1700000000, 0)
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				return os.Chtimes(path, modified, modified)
			}); err != nil {
				t.Fatal(err)
			}
			want := restoredDirectoryTree(t, project)
			if out, err := restoreCommand(t, f, "run", "daily"); err != nil {
				t.Fatalf("backup: %v\n%s", err, out)
			}
			if err := os.RemoveAll(f.source); err != nil {
				t.Fatal(err)
			}
			if kind != "daily" {
				if out, err := restoreCommand(t, f, "run", kind); err != nil {
					t.Fatalf("derived backup: %v\n%s", err, out)
				}
			}
			historyPath := filepath.Join(f.home, ".goback.db")
			history, err := os.ReadFile(historyPath)
			if err != nil {
				t.Fatal(err)
			}
			out, err := restoreCommand(t, f, "restore", kind, "Projects/my-app", "--to", f.recovery, "--recursive")
			if err != nil || !strings.Contains(out, "SHA-256 content verified") || !strings.Contains(out, "Files: 4") {
				t.Fatalf("restore: %v\n%s", err, out)
			}
			got := restoredDirectoryTree(t, filepath.Join(f.recovery, "my-app"))
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("recovered project differs:\nwant: %#v\ngot:  %#v", want, got)
			}
			after, err := os.ReadFile(historyPath)
			if err != nil || !bytes.Equal(history, after) {
				t.Fatalf("restore changed backup history: %v", err)
			}
			if _, err := os.Stat(f.source); !os.IsNotExist(err) {
				t.Fatalf("restore recreated the original source: %v", err)
			}
			entries, err := os.ReadDir(f.recovery)
			if err != nil || len(entries) != 1 || entries[0].Name() != "my-app" {
				t.Fatalf("unexpected recovery entries: %v (%v)", entries, err)
			}
		})
	}
}

func TestRestoreCLIRecursiveDryRunReportsTotalsWithoutWrites(t *testing.T) {
	f := restoreCLIFixture(t)
	putBackupFile(t, f, "weekly", "Projects/my-app/notes.txt", "hello")
	putBackupFile(t, f, "weekly", "Projects/my-app/nested/notes.txt", "world!")
	if err := os.Mkdir(filepath.Join(f.backup, "weekly", "Projects/my-app/empty"), 0750); err != nil {
		t.Fatal(err)
	}
	before := restoreTree(t, f.root)
	out, err := restoreCommand(t, f, "restore", "weekly", "Projects/my-app", "--to", f.recovery, "--recursive", "--dry-run")
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	backup, _ := filepath.EvalSymlinks(f.backup)
	recovery, _ := filepath.EvalSymlinks(f.recovery)
	for _, want := range []string{
		filepath.Join(backup, "weekly", "Projects/my-app"), filepath.Join(recovery, "my-app"),
		"Files: 2", "Size: 11 bytes", "Directories: 3", "Dry run", "content verification runs during a real restore",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("preview missing %q:\n%s", want, out)
		}
	}
	if !reflect.DeepEqual(before, restoreTree(t, f.root)) {
		t.Fatal("recursive dry run changed files, directory metadata, locks, or history")
	}
}

func TestRestoreCLIRejectsUnsupportedDirectoryRequestsWithoutWrites(t *testing.T) {
	for _, kind := range []string{"missing recursive flag", "nested symlink", "nested fifo", "existing file", "existing empty directory", "existing nonempty directory", "existing dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := restoreCLIFixture(t)
			putBackupFile(t, f, "daily", "Projects/my-app/notes.txt", "saved")
			project := filepath.Join(f.backup, "daily", "Projects/my-app")
			target := filepath.Join(f.recovery, "my-app")
			args := []string{"restore", "daily", "Projects/my-app", "--to", f.recovery, "--recursive"}
			want := "refusing to overwrite"
			switch kind {
			case "missing recursive flag":
				args = args[:len(args)-1]
				want = "use --recursive"
			case "nested symlink":
				if err := os.Symlink("missing", filepath.Join(project, "link")); err != nil {
					t.Fatal(err)
				}
				want = "unsupported backup entry"
			case "nested fifo":
				if err := unix.Mkfifo(filepath.Join(project, "pipe"), 0600); err != nil {
					t.Fatal(err)
				}
				want = "unsupported backup entry"
			case "existing file":
				if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "existing empty directory", "existing nonempty directory":
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "existing nonempty directory" {
					if err := os.WriteFile(filepath.Join(target, "keep"), []byte("keep"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "existing dangling symlink":
				if err := os.Symlink("missing", target); err != nil {
					t.Fatal(err)
				}
			}
			before := restoreTree(t, f.root)
			out, err := restoreCommand(t, f, args...)
			if err == nil || !strings.Contains(out, want) {
				t.Fatalf("invalid recursive request: %v\n%s", err, out)
			}
			if !reflect.DeepEqual(before, restoreTree(t, f.root)) {
				t.Fatal("invalid recursive request changed the filesystem")
			}
		})
	}
}
