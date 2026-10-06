package cleanlogs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCleanupPreviewsAndRetainsNewestLogsWithoutTouchingOtherFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".goback", "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	remove := []string{filepath.Join(dir, "failure-20261001-old.log"), filepath.Join(home, ".goback-daily-old")}
	keep := []string{filepath.Join(dir, "failure-20261002-new.log"), filepath.Join(dir, "unrelated.log"), filepath.Join(home, ".goback-daily-new"), filepath.Join(home, ".goback.json"), filepath.Join(home, ".goback.db")}
	for _, path := range append(append([]string{}, remove...), keep...) {
		if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(remove[1], old, old); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "failure-20260901-symlink.log")
	if err := os.Symlink(keep[1], link); err != nil {
		t.Fatal(err)
	}
	keep = append(keep, link)
	var preview bytes.Buffer
	if err := Clean(&preview, 1, 1, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	if strings.Count(preview.String(), "Would remove") != len(remove) {
		t.Fatal(preview.String())
	}
	for _, path := range remove {
		if !strings.Contains(preview.String(), path) {
			t.Fatalf("missing candidate %s: %s", path, &preview)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal("preview deleted a log", err)
		}
	}
	var output bytes.Buffer
	if err := Clean(&output, 1, 1, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	for _, path := range remove {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("candidate retained: %s", path)
		}
	}
	for _, path := range keep {
		content, err := os.ReadFile(path)
		if err != nil || string(content) != "original" {
			t.Fatalf("changed retained file %s: %v", path, err)
		}
	}
}

func TestCleanupMissingDirectoryAndInvalidRetentionDoNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var out bytes.Buffer
	if err := Clean(&out, 0, 0, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := Clean(&out, -1, 0, 0, 0, false); err == nil {
		t.Fatal("accepted negative retention")
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cleanup wrote to empty home: %v %v", entries, err)
	}
}
