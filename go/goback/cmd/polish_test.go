package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigEditUsesPreferenceAndAllowsRepair(t *testing.T) {
	t.Setenv("EDITOR", "/bin/echo fallback-editor")
	for _, tc := range []struct{ content, want string }{
		{`{"editor":"/bin/echo configured-editor"}`, "configured-editor"},
		{`{"editor":`, "fallback-editor"},
		{`{"source":"/legacy","editor":"/bin/echo legacy-editor"}`, "legacy-editor"},
	} {
		out, err := profileCommand(t, tc.content, "config", "edit")
		if err != nil || !strings.Contains(out, tc.want) || !strings.Contains(out, "fixture.json") {
			t.Fatalf("edit: %v\n%s", err, out)
		}
	}
}

func TestPreviewValidatesSettingsWithoutMountedPaths(t *testing.T) {
	for _, tc := range []struct{ profile, kind, want string }{
		{`{"source":"/Volumes/Offline/source/","destination":"/Volumes/Offline/backup","rsync":{"daily":{"archive":true}}}`, "daily", "Full command:"},
		{`{"source":"/offline/source/","destination":"/offline/backup","rsync":{"daily":{"archive":true}}}`, "daily", "Full command:"},
		{`{"destination":"/offline/backup","rsync":{"weekly":{"archive":true}}}`, "weekly", "Full command:"},
		{`{"destination":"/offline/backup","rsync":{"monthly":{"archive":false}}}`, "monthly", "Full command:"},
		{`{"source":"/offline/source/","destination":"/offline/backup"}`, "monthly", "no rsync.monthly configuration"},
		{`{"destination":"/offline/backup","rsync":{"daily":{"archive":true}}}`, "daily", "source not set"},
		{`{"source":"/offline/source/","rsync":{"daily":{"archive":true}}}`, "daily", "destination not set"},
	} {
		out, err := profileCommand(t, `{"profiles":{"default":`+tc.profile+`}}`, "preview", tc.kind)
		if (err == nil) != (tc.want == "Full command:") || !strings.Contains(out, tc.want) {
			t.Fatalf("preview %s: %v\n%s", tc.kind, err, out)
		}
	}
}

func TestPatternPreviewsPreservePartiallyExcludedDirectories(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Documents"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"keep.txt", "scratch.tmp"} {
		if err := os.WriteFile(filepath.Join(root, "Documents", name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	content, err := json.Marshal(map[string]any{"profiles": map[string]any{"test": map[string]any{
		"source": root + "/", "destination": "/offline/backup",
		"rsync": map[string]any{"daily": map[string]any{"archive": true, "excludedPatterns": []string{"/Documents/*.tmp"}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"--test-pattern", "/Documents/*.tmp"}, {"--excluded"}, {"--test-pattern", "/Documents/*.tmp", "--subdir", "Documents"}, {"--excluded", "--subdir", "Documents"}} {
		args := append([]string{"preview", "daily", "--no-pager"}, flags...)
		out, err := profileCommand(t, string(content), args...)
		if err != nil || !strings.Contains(out, "\nDocuments/scratch.tmp\n") || strings.Contains(out, "\nDocuments/\n") || strings.Contains(out, "keep.txt") {
			t.Fatalf("%v: %v\n%s", flags, err, out)
		}
	}
}

func TestPreviewRejectsNegativeDepth(t *testing.T) {
	content := `{"profiles":{"test":{"source":"/offline/source","destination":"/offline/backup","rsync":{"daily":{"archive":true}}}}}`
	for _, flags := range [][]string{nil, {"--excluded"}, {"--test-pattern", "*.tmp"}} {
		args := append([]string{"preview", "daily", "--depth", "-1"}, flags...)
		out, err := profileCommand(t, content, args...)
		if err == nil || !strings.Contains(out, "--depth must be greater than or equal to 0") {
			t.Fatalf("%v: %v\n%s", flags, err, out)
		}
	}
}

func TestCommandsRejectUnexpectedArgumentsBeforeConfiguration(t *testing.T) {
	for _, args := range [][]string{
		{"preview", "daily", "typo"}, {"run", "daily", "typo"}, {"run", "all", "typo"},
		{"mirror", "typo"}, {"eject", "typo"}, {"config", "edit", "typo"},
		{"usage", "reset", "typo"}, {"clean", "logs", "typo"},
	} {
		out, err := profileCommand(t, `{}`, args...)
		if err == nil || !strings.Contains(out, "unknown command") || strings.Contains(out, "no backup profiles") {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
}

func TestProfileCompletionReadsCustomConfigWithoutHostnameResolution(t *testing.T) {
	out, err := profileCommand(t, `{"profiles":{"media":{},"macbook":{"hostname":"unmatched"},"other":{}}}`, "__complete", "run", "daily", "--profile", "m")
	if err != nil || !strings.Contains(out, "macbook\nmedia\n:4") || strings.Contains(out, "other") {
		t.Fatalf("completion: %v\n%s", err, out)
	}
	for _, content := range []string{`{}`, `{"profiles":`} {
		out, err := profileCommand(t, content, "__complete", "run", "daily", "--profile", "")
		if err != nil || !strings.Contains(out, ":4") || strings.Contains(out, "recreate") {
			t.Fatalf("completion of invalid/empty config: %v\n%s", err, out)
		}
	}
	home := t.TempDir()
	out = globalCommand(t, home, "__complete", "run", "daily", "--profile", "")
	if !strings.Contains(out, ":4") {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(home, ".goback.json")); !os.IsNotExist(err) {
		t.Fatal("completion created config")
	}
}

func TestDryRunListsFilesAndPrintsPlainReport(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
	for _, configured := range []bool{false, true} {
		root := t.TempDir()
		src, dest := filepath.Join(root, "source"), filepath.Join(root, "destination")
		for _, dir := range []string{src, dest} {
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(src, "new-file.txt"), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
		content, err := json.Marshal(map[string]any{"confirmExec": false, "profiles": map[string]any{"default": map[string]any{
			"source": src + "/", "destination": dest, "rsync": map[string]any{"daily": map[string]any{"archive": true, "dryRun": configured}},
			"dailyCompanions": []any{
				map[string]any{"id": "check", "command": []string{"/usr/bin/true"}, "dryRunArgs": []string{"--dry-run"}},
				map[string]any{"id": "skipped", "command": []string{"/usr/bin/false"}},
			},
		}}})
		if err != nil {
			t.Fatal(err)
		}
		args := []string{"run", "daily"}
		if !configured {
			args = append(args, "--dry-run")
		}
		out, err := profileCommand(t, string(content), args...)
		if err != nil || !strings.Contains(out, "new-file.txt") || strings.Count(out, "dry run: succeeded") != 2 || !strings.Contains(out, "dry run: skipped") || strings.Contains(out, "\x1b[") {
			t.Fatalf("configured=%v: %v\n%s", configured, err, out)
		}
		if _, err := os.Stat(filepath.Join(dest, "daily")); !os.IsNotExist(err) {
			t.Fatal("dry run created destination")
		}
	}
}

func TestHistoryFailureDoesNotStopCompanionsOrReport(t *testing.T) {
	root := t.TempDir()
	src, dest := filepath.Join(root, "source"), filepath.Join(root, "destination")
	for _, dir := range []string{src, dest, filepath.Join(root, ".goback.db")} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "data"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "rsync"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	content, err := json.Marshal(map[string]any{"confirmExec": false, "profiles": map[string]any{"default": map[string]any{
		"source": src + "/", "destination": dest, "rsync": map[string]any{"daily": map[string]any{"archive": true}},
		"dailyCompanions": []any{map[string]any{"id": "check", "command": []string{"/bin/echo", "companion-completed"}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.json")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	out := globalCommand(t, root, "--config", path, "run", "daily")
	for _, want := range []string{"history unavailable", "companion-completed", "succeeded", "companion/check"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
}
