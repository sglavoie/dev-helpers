package cmd

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
)

func globalCommand(t *testing.T, home string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	processArgs := append([]string{"-test.run=^TestProfileCommandProcess$", "--"}, args...)
	command := exec.CommandContext(ctx, os.Args[0], processArgs...)
	command.Env = append(os.Environ(), "GOBACK_TEST_COMMAND=1", "HOME="+home)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestCompletionWithoutConfiguration(t *testing.T) {
	home := t.TempDir()
	out := globalCommand(t, home, "completion", "zsh")
	if !strings.Contains(out, "#compdef goback") {
		t.Fatalf("not a completion script: %s", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".goback.json")); !os.IsNotExist(err) {
		t.Fatal("completion created config")
	}
}

func TestHistoryWithoutConfigurationAndWithRedirectedOutput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	db.RecordBackup(db.HistoryEntry{CreatedAt: time.Now().Add(-2 * time.Hour), BackupType: "daily", Profile: "retired", ExecutionTime: "1s", Command: "rsync", ExitCode: 23})
	for _, flags := range [][]string{nil, {"--no-pager"}} {
		args := append([]string{"usage", "view", "--profile", "retired"}, flags...)
		out := globalCommand(t, home, args...)
		if strings.Count(out, "retired") != 1 || strings.Contains(out, "Initializing") || strings.Contains(out, "\x1b[") {
			t.Fatalf("expected one plain table: %s", out)
		}
	}
	out := globalCommand(t, home, "usage", "last", "--summary")
	for _, want := range []string{"retired", "hours ago", "partial"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	globalCommand(t, home, "usage", "reset", "--profile", "retired", "--keep", "0")
	if out := globalCommand(t, home, "usage", "view"); !strings.Contains(out, "No backups data found") {
		t.Fatal(out)
	}
}

func TestGlobalCommandsIgnoreUnmatchedProfiles(t *testing.T) {
	for _, args := range [][]string{{"completion", "zsh"}, {"usage", "last", "--summary"}} {
		out, err := profileCommand(t, `{"profiles":{"alpha":{"hostname":"no-alpha"},"beta":{"hostname":"no-beta"}}}`, args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
}

func TestCleanupDryRunThroughCLI(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
	home := t.TempDir()
	daily := filepath.Join(home, "backups", "daily")
	if err := os.MkdirAll(daily, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"keep.txt", "two  spaces.txt"} {
		if err := os.WriteFile(filepath.Join(daily, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	content, err := json.Marshal(map[string]any{"profiles": map[string]any{"default": map[string]any{
		"destination": filepath.Dir(daily), "rsync": map[string]any{"daily": map[string]any{
			"archive": true, "includedPatterns": []string{"keep.txt"}, "excludedPatterns": []string{"*.txt"},
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, "config.json")
	if err := os.WriteFile(configPath, content, 0600); err != nil {
		t.Fatal(err)
	}
	out := globalCommand(t, home, "--config", configPath, "clean", "backup", "daily", "--dry-run")
	for _, want := range []string{`"two  spaces.txt"`, "would delete 1", "deleted 0, failed 0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "keep.txt") {
		t.Fatal("included file proposed for deletion")
	}
	entries, err := os.ReadDir(daily)
	if err != nil || len(entries) != 2 {
		t.Fatalf("dry run changed files: %v, %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".goback.db")); !os.IsNotExist(err) {
		t.Fatal("cleanup wrote history")
	}
}
