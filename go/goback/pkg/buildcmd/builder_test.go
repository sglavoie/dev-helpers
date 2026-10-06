package buildcmd

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/spf13/viper"
)

func snapshotFixture(t *testing.T) (string, string, string) {
	t.Helper()
	viper.Reset()
	previous := config.ActiveProfileName
	config.ActiveProfileName = "test"
	t.Cleanup(func() { viper.Reset(); config.ActiveProfileName = previous })
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	src, dest := filepath.Join(root, "Sam's files"), filepath.Join(root, "backup")
	for _, dir := range []string{src, dest} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "file"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	viper.Set("profiles.test.source", src)
	viper.Set("profiles.test.destination", dest)
	viper.Set("profiles.test.rsync.daily.archive", true)
	log := filepath.Join(root, "args")
	t.Setenv("GOBACK_TEST_ARGS", log)
	if err := os.WriteFile(filepath.Join(root, "rsync"), []byte("#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$GOBACK_TEST_ARGS\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	return src, dest, log
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists or cannot be checked: %v", path, err)
	}
}

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
}

func TestSnapshotDryRunsPreserveDestinationAndHistory(t *testing.T) {
	for _, setting := range []string{"cliDryRun", "profiles.test.rsync.daily.dryRun"} {
		t.Run(setting, func(t *testing.T) {
			_, dest, log := snapshotFixture(t)
			viper.Set(setting, true)
			b, err := BuildDaily()
			if err != nil {
				t.Fatal(err)
			}
			requireAbsent(t, filepath.Join(dest, "daily"))
			if result := b.Execute(context.Background()); result.Err != nil {
				t.Fatal(result.Err)
			}
			if !slices.Contains(readArgs(t, log), "--dry-run") {
				t.Fatal("rsync did not receive --dry-run")
			}
			requireAbsent(t, filepath.Join(dest, "daily"))
			requireAbsent(t, filepath.Join(os.Getenv("HOME"), ".goback.db"))
		})
	}
}

func TestSnapshotPassesLiteralArgumentsAndCopyableCommand(t *testing.T) {
	src, dest, log := snapshotFixture(t)
	viper.Set("cliDryRun", true)
	pattern := "a 'quoted' $HOME `echo expanded` $(echo expanded) *\nnext"
	viper.Set("profiles.test.rsync.daily.excludedPatterns", []string{pattern})
	b, err := BuildDaily()
	if err != nil {
		t.Fatal(err)
	}
	if result := b.Execute(context.Background()); result.Err != nil {
		t.Fatal(result.Err)
	}
	want := []string{"--archive", "--dry-run", "--itemize-changes", "--stats", "--exclude=" + pattern, "--", src, filepath.Join(dest, "daily")}
	if got := readArgs(t, log); !slices.Equal(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
	// The displayed command must also survive copying into a shell unchanged.
	if output, err := exec.Command("sh", "-c", b.CommandString()).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	if got := readArgs(t, log); !slices.Equal(got, want) {
		t.Fatalf("copied command args = %q, want %q", got, want)
	}
}

func TestSnapshotCreatesDestinationOnlyAtExecution(t *testing.T) {
	_, dest, _ := snapshotFixture(t)
	b, err := BuildDaily()
	if err != nil {
		t.Fatal(err)
	}
	requireAbsent(t, filepath.Join(dest, "daily"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result := b.Execute(ctx); !result.Interrupted {
		t.Fatalf("result = %+v", result)
	}
	requireAbsent(t, filepath.Join(dest, "daily"))
	if result := b.Execute(context.Background()); result.Err != nil {
		t.Fatal(result.Err)
	}
	if info, err := os.Stat(filepath.Join(dest, "daily")); err != nil || !info.IsDir() {
		t.Fatalf("destination not created: %v", err)
	}
	db.QueryRows("SELECT exit_code FROM backups", func(rows *sql.Rows) {
		var code int
		if !rows.Next() {
			t.Fatal("successful transfer not recorded")
		}
		if err := rows.Scan(&code); err != nil {
			t.Fatal(err)
		}
		if code != 0 || rows.Next() {
			t.Fatal("expected exactly one successful transfer")
		}
	})
}

func TestSnapshotPathValidation(t *testing.T) {
	for _, name := range []string{"sibling prefix", "nested", "symlink nested", "same", "destination contains source", "destination file", "dangling symlink"} {
		t.Run(name, func(t *testing.T) {
			src, dest, _ := snapshotFixture(t)
			target := filepath.Join(dest, "daily")
			switch name {
			case "sibling prefix":
				dest = src + "-backup"
				if err := os.Mkdir(dest, 0755); err != nil {
					t.Fatal(err)
				}
				viper.Set("profiles.test.destination", dest)
			case "nested":
				viper.Set("profiles.test.destination", src)
			case "symlink nested", "same", "destination contains source", "dangling symlink":
				link := src
				if name == "symlink nested" {
					link = filepath.Join(src, "child")
					if err := os.Mkdir(link, 0755); err != nil {
						t.Fatal(err)
					}
				}
				if name == "destination contains source" {
					link = filepath.Dir(src)
				}
				if name == "dangling symlink" {
					link = filepath.Join(dest, "missing")
				}
				if err := os.Symlink(link, target); err != nil {
					t.Fatal(err)
				}
			case "destination file":
				if err := os.WriteFile(target, []byte("file"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := BuildDaily()
			if (err == nil) != (name == "sibling prefix") {
				t.Fatalf("BuildDaily() error = %v", err)
			}
			if name == "nested" {
				requireAbsent(t, filepath.Join(src, "daily"))
			}
		})
	}
}

func TestRealRsyncSnapshotDryRunAndTransfer(t *testing.T) {
	realRsync, err := exec.LookPath("rsync")
	if err != nil {
		t.Skip("rsync is unavailable")
	}
	src, dest, _ := snapshotFixture(t)
	for _, dry := range []bool{true, false} {
		viper.Set("cliDryRun", dry)
		b, err := BuildDaily()
		if err != nil {
			t.Fatal(err)
		}
		b.args[0] = realRsync
		if result := b.Execute(context.Background()); result.Err != nil {
			t.Fatalf("dry=%v: %v", dry, result.Err)
		}
		if dry {
			requireAbsent(t, filepath.Join(dest, "daily"))
			requireAbsent(t, filepath.Join(os.Getenv("HOME"), ".goback.db"))
		} else {
			// Preserve the existing daily layout for a source without a trailing slash.
			data, err := os.ReadFile(filepath.Join(dest, "daily", filepath.Base(src), "file"))
			if err != nil || string(data) != "data" {
				t.Fatalf("transferred file = %q, %v", data, err)
			}
		}
	}
}

func TestInterruptedSnapshotRecordsAttempt(t *testing.T) {
	_, _, log := snapshotFixture(t)
	program := filepath.Join(filepath.Dir(log), "rsync")
	if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf started > \"$GOBACK_TEST_ARGS\"\nexec sleep 30\n"), 0755); err != nil {
		t.Fatal(err)
	}
	b, err := BuildDaily()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, err := os.Stat(log); err == nil {
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	result := b.Execute(ctx)
	<-done
	if !result.Interrupted || result.NotStarted || result.ExitCode != -1 {
		t.Fatalf("result = %+v", result)
	}
	db.QueryRows("SELECT exit_code FROM backups", func(rows *sql.Rows) {
		var code int
		if !rows.Next() {
			t.Fatal("interrupted transfer not recorded")
		}
		if err := rows.Scan(&code); err != nil {
			t.Fatal(err)
		}
		if code != -1 || rows.Next() {
			t.Fatalf("expected exactly one interrupted attempt, exit = %d", code)
		}
	})
}
