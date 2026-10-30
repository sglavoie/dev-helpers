package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type restoreFixture struct {
	root, home, config, locks, source, backup, recovery string
}

func restoreCLIFixture(t *testing.T) restoreFixture {
	t.Helper()
	f := restoreFixture{root: t.TempDir()}
	f.home = filepath.Join(f.root, "home")
	f.locks = filepath.Join(f.root, "locks")
	f.source = filepath.Join(f.root, "source")
	f.backup = filepath.Join(f.root, "backup")
	f.recovery = filepath.Join(f.root, "recovery")
	f.config = filepath.Join(f.home, "config.json")
	for _, dir := range []string{f.home, f.locks, f.source, f.backup, f.recovery} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	content, err := json.Marshal(map[string]any{
		"confirmExec": false,
		"profiles": map[string]any{"recover": map[string]any{
			"source": f.source + "/", "destination": f.backup,
			"rsync": map[string]any{"daily": map[string]any{"archive": true}, "weekly": map[string]any{"archive": true}, "monthly": map[string]any{"archive": true}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.config, content, 0600); err != nil {
		t.Fatal(err)
	}
	return f
}

func restoreCommand(t *testing.T, f restoreFixture, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	processArgs := append([]string{"-test.run=^TestProfileCommandProcess$", "--", "--config", f.config}, args...)
	command := exec.CommandContext(ctx, os.Args[0], processArgs...)
	command.Env = append(os.Environ(), "GOBACK_TEST_COMMAND=1", "HOME="+f.home, "TMPDIR="+f.locks)
	output, err := command.CombinedOutput()
	return string(output), err
}

// Use the actual command tree and real rsync: create a backup, remove the entire
// live source, and recover the lost file. A restore must not refresh backup health.
func TestRestoreCLIBacksUpDeletesAndRecoversAFile(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is required for the backup-to-restore integration test")
	}
	for _, kind := range []string{"daily", "weekly", "monthly"} {
		t.Run(kind, func(t *testing.T) {
			f := restoreCLIFixture(t)
			original := filepath.Join(f.source, "notes.txt")
			content := []byte("recover these notes\x00\n")
			if err := os.WriteFile(original, content, 0740); err != nil {
				t.Fatal(err)
			}
			modified := time.Unix(1700000000, 0)
			if err := os.Chtimes(original, modified, modified); err != nil {
				t.Fatal(err)
			}
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
			out, err := restoreCommand(t, f, "restore", kind, "notes.txt", "--to", f.recovery)
			if err != nil || !strings.Contains(out, "SHA-256 content verified") {
				t.Fatalf("restore: %v\n%s", err, out)
			}
			recovered := filepath.Join(f.recovery, "notes.txt")
			got, err := os.ReadFile(recovered)
			if err != nil || !bytes.Equal(got, content) {
				t.Fatalf("recovered content: %q (%v)", got, err)
			}
			info, err := os.Stat(recovered)
			if err != nil || info.Mode().Perm() != 0740 || !info.ModTime().Equal(modified) {
				t.Fatalf("recovered metadata: %v (%v)", info, err)
			}
			after, err := os.ReadFile(historyPath)
			if err != nil || !bytes.Equal(history, after) {
				t.Fatalf("restore changed backup history: %v", err)
			}
			if _, err := os.Stat(f.source); !os.IsNotExist(err) {
				t.Fatalf("restore recreated the original source: %v", err)
			}
		})
	}
}

type restoreTreeEntry struct {
	mode     fs.FileMode
	size     int64
	modified time.Time
	content  string
}

func restoreTree(t *testing.T, root string) map[string]restoreTreeEntry {
	t.Helper()
	entries := map[string]restoreTreeEntry{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := restoreTreeEntry{mode: info.Mode(), size: info.Size(), modified: info.ModTime()}
		if info.Mode().IsRegular() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.content = string(content)
		}
		entries[path] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func putBackupFile(t *testing.T, f restoreFixture, kind, path, content string) {
	t.Helper()
	file := filepath.Join(f.backup, kind, path)
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreCLIDryRunPrintsExactPathsAndWritesNothing(t *testing.T) {
	f := restoreCLIFixture(t)
	putBackupFile(t, f, "weekly", "nested/notes.txt", "saved notes")
	before := restoreTree(t, f.root)
	out, err := restoreCommand(t, f, "restore", "weekly", "nested/notes.txt", "--to", f.recovery, "--dry-run")
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	backup, _ := filepath.EvalSymlinks(f.backup)
	recovery, _ := filepath.EvalSymlinks(f.recovery)
	for _, want := range []string{"Profile: recover", filepath.Join(backup, "weekly", "nested", "notes.txt"), filepath.Join(recovery, "notes.txt"), "Dry run", "content verification runs during a real restore"} {
		if !strings.Contains(out, want) {
			t.Fatalf("preview missing %q:\n%s", want, out)
		}
	}
	if !reflect.DeepEqual(before, restoreTree(t, f.root)) {
		t.Fatal("dry run changed files, directory metadata, locks, or history")
	}
}

func TestRestoreCLIRejectsInvalidRequestsWithoutWrites(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no arguments", nil, "accepts 2 arg"},
		{"missing file argument", []string{"daily"}, "accepts 2 arg"},
		{"extra arguments", []string{"daily", "notes.txt", "extra"}, "accepts 2 arg"},
		{"missing to", []string{"daily", "notes.txt"}, `required flag(s) "to"`},
		{"bad kind", []string{"yearly", "notes.txt", "--to", "RECOVERY"}, "backup type"},
		{"all profiles", []string{"daily", "notes.txt", "--all", "--to", "RECOVERY"}, "--all does not apply"},
		{"unknown profile", []string{"daily", "notes.txt", "--profile", "missing", "--to", "RECOVERY"}, `profile "missing" not found`},
		{"missing file", []string{"daily", "missing.txt", "--to", "RECOVERY"}, "open backup file"},
		{"escaping file", []string{"daily", "../notes.txt", "--to", "RECOVERY"}, "relative path"},
		{"missing directory", []string{"daily", "notes.txt", "--to", "RECOVERY/missing", "--dry-run"}, "existing recovery directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := restoreCLIFixture(t)
			putBackupFile(t, f, "daily", "notes.txt", "saved notes")
			args := []string{"restore"}
			for _, arg := range tc.args {
				args = append(args, strings.ReplaceAll(arg, "RECOVERY", f.recovery))
			}
			before := restoreTree(t, f.root)
			out, err := restoreCommand(t, f, args...)
			if err == nil || !strings.Contains(out, tc.want) {
				t.Fatalf("invalid request: %v\n%s", err, out)
			}
			if !reflect.DeepEqual(before, restoreTree(t, f.root)) {
				t.Fatal("invalid request changed the filesystem")
			}
		})
	}
}

func TestRestoreCLIUsesExactlyOneSelectedProfile(t *testing.T) {
	f := restoreCLIFixture(t)
	putBackupFile(t, f, "monthly", "notes.txt", "saved notes")
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	profile := map[string]any{"hostname": hostname, "source": "/offline/source", "destination": f.backup, "rsync": map[string]any{"daily": map[string]any{"archive": true}}}
	content, err := json.Marshal(map[string]any{"profiles": map[string]any{"a": profile, "b": profile}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.config, content, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := restoreCommand(t, f, "restore", "monthly", "notes.txt", "--to", f.recovery)
	if err == nil || !strings.Contains(out, "restore requires one profile") {
		t.Fatalf("ambiguous restore: %v\n%s", err, out)
	}
	// A retained monthly copy remains recoverable even when monthly is no longer
	// configured for new backups, and the configured live source is offline.
	out, err = restoreCommand(t, f, "restore", "monthly", "notes.txt", "--to", f.recovery, "--profile", "b")
	if err != nil || !strings.Contains(out, "Profile: b") || !strings.Contains(out, "content verified") {
		t.Fatalf("explicit profile restore: %v\n%s", err, out)
	}
}

func TestRestoreCLINeverCreatesOrRepairsConfiguration(t *testing.T) {
	for _, content := range []string{"missing", `{"profiles":`, `{"profiels":{}}`} {
		t.Run(content, func(t *testing.T) {
			f := restoreCLIFixture(t)
			if content == "missing" {
				if err := os.Remove(f.config); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(f.config, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			before := restoreTree(t, f.root)
			out, err := restoreCommand(t, f, "restore", "daily", "notes.txt", "--to", f.recovery, "--dry-run")
			if err == nil || strings.Contains(out, "create one") || strings.Contains(out, "recreate") {
				t.Fatalf("configuration error prompted: %v\n%s", err, out)
			}
			if !reflect.DeepEqual(before, restoreTree(t, f.root)) {
				t.Fatal("restore created or repaired configuration")
			}
		})
	}
}
