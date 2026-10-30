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

	"github.com/sglavoie/dev-helpers/go/goback/pkg/destinationlock"
)

// Drive the installed command tree with stdin closed and a disposable HOME,
// including error exits, configuration loading, locks, history, and companions.
func unattendedProcess(t *testing.T, home, configPath string, env []string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	processArgs := append([]string{"-test.run=^TestProfileCommandProcess$", "--", "--config", configPath}, args...)
	command := exec.CommandContext(ctx, os.Args[0], processArgs...)
	command.Env = append(os.Environ(), "GOBACK_TEST_COMMAND=1", "HOME="+home)
	command.Env = append(command.Env, env...)
	out, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("command did not finish unattended: %v\n%s", ctx.Err(), out)
	}
	return string(out), err
}

func TestUnattendedConfigErrorsNeverPromptOrWrite(t *testing.T) {
	for _, content := range []string{"", `{"profiles":`, `{"confirmExec":"false"}`} {
		t.Run(content, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "config.json")
			if content != "" {
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			out, err := unattendedProcess(t, home, path, nil, "run", "daily", "--non-interactive")
			if err == nil || !strings.Contains(out, "configuration") || strings.Contains(out, "create one") || strings.Contains(out, "recreate") {
				t.Fatalf("configuration error: %v\n%s", err, out)
			}
			entries, err := os.ReadDir(home)
			want := 1
			if content == "" {
				want = 0
			} else if data, err := os.ReadFile(path); err != nil || string(data) != content {
				t.Fatalf("config changed: %v, %s", err, data)
			}
			if err != nil || len(entries) != want {
				t.Fatalf("unexpected writes: %v, %v", entries, err)
			}
		})
	}
}

func unattendedFixture(t *testing.T, companion, source, destination string) (home, path, bin string) {
	t.Helper()
	home = t.TempDir()
	bin = filepath.Join(home, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{source, destination} {
		if err := os.MkdirAll(endpoint, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "note.txt"), []byte("backup fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HOME/rsync-args\"\nexit \"${GOBACK_RSYNC_EXIT:-0}\"\n"
	if err := os.WriteFile(filepath.Join(bin, "rsync"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	profile := map[string]any{"source": source + "/", "destination": destination, "rsync": map[string]any{"daily": map[string]any{"archive": true}}}
	if companion != "" {
		profile["dailyCompanions"] = []any{map[string]any{"id": "fixture", "command": []string{companion}}}
	}
	data, err := json.Marshal(map[string]any{"confirmExec": true, "showProgress": true, "profiles": map[string]any{"test": profile}})
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(home, "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return home, path, bin
}

func TestUnattendedRunOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, rsyncExit, companion   string
		dryRun, wantErr, wantHistory bool
	}{
		{name: "success", wantHistory: true},
		{name: "vanished source", rsyncExit: "24", wantHistory: true},
		{name: "failed rsync", rsyncExit: "23", wantErr: true, wantHistory: true},
		{name: "failed companion", companion: "/usr/bin/false", wantErr: true, wantHistory: true},
		{name: "missing companion", companion: "goback-test-missing-companion", wantErr: true, wantHistory: true},
		{name: "dry run skips companion", companion: "/usr/bin/false", dryRun: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dest := filepath.Join(root, "backup")
			home, path, bin := unattendedFixture(t, tc.companion, filepath.Join(root, "source"), dest)
			args := []string{"run", "daily", "--non-interactive"}
			if tc.dryRun {
				args = append(args, "--dry-run")
			}
			out, err := unattendedProcess(t, home, path, []string{"PATH=" + bin + ":/usr/bin:/bin", "GOBACK_RSYNC_EXIT=" + tc.rsyncExit}, args...)
			if (err != nil) != tc.wantErr || strings.Contains(out, "wish to proceed") || !strings.Contains(out, "goback: finished") {
				t.Fatalf("run: %v\n%s", err, out)
			}
			argv, err := os.ReadFile(filepath.Join(home, "rsync-args"))
			if err != nil || strings.Contains(string(argv), "--progress") {
				t.Fatalf("rsync did not run without progress: %v, %s", err, argv)
			}
			_, historyErr := os.Stat(filepath.Join(home, ".goback.db"))
			if (historyErr == nil) != tc.wantHistory {
				t.Fatalf("history: %v, want history %v", historyErr, tc.wantHistory)
			}
			if tc.dryRun {
				if _, err := os.Stat(filepath.Join(dest, "daily")); !os.IsNotExist(err) {
					t.Fatalf("dry run created destination: %v", err)
				}
			}
			if tc.companion != "" && tc.wantErr && !strings.Contains(out, `companion "fixture"`) {
				t.Fatalf("missing companion failure in command error:\n%s", out)
			}
		})
	}
}

func TestUnattendedRunRefusesBusyOrMissingDestination(t *testing.T) {
	for _, busy := range []bool{true, false} {
		root := t.TempDir()
		dest := filepath.Join(root, "backup")
		home, path, bin := unattendedFixture(t, "", filepath.Join(root, "source"), dest)
		if busy {
			_, release, err := destinationlock.Acquire(context.Background(), dest)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
		} else if err := os.Remove(dest); err != nil {
			t.Fatal(err)
		}
		out, err := unattendedProcess(t, home, path, []string{"PATH=" + bin}, "run", "daily", "--non-interactive")
		if err == nil || (busy && !strings.Contains(out, "another goback operation")) || (!busy && !strings.Contains(out, dest)) {
			t.Fatalf("busy=%v: %v\n%s", busy, err, out)
		}
		if _, err := os.Stat(filepath.Join(home, "rsync-args")); !os.IsNotExist(err) {
			t.Fatal("rsync started despite refused destination")
		}
		if _, err := os.Stat(filepath.Join(home, ".goback.db")); !os.IsNotExist(err) {
			t.Fatal("refused run created history")
		}
	}
}
