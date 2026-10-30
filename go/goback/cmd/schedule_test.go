package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const scheduleConfig = `{"confirmExec":true,"profiles":{"test":{"source":"/offline/source/","destination":"/offline/backup","rsync":{"daily":{"archive":true}}}}}`

func TestScheduleDailyReadsOfflineConfigWithoutWriting(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "config & backup.json")
	if err := os.WriteFile(path, []byte(scheduleConfig), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := unattendedProcess(t, home, path, nil, "schedule", "daily", "--profile", "test", "--at", "07:35", "--exec-path", "/usr/bin:/bin")
	if err != nil || !strings.HasPrefix(out, "<?xml") || !strings.Contains(out, "config &amp; backup.json") || !strings.Contains(out, "<integer>35</integer>") || !strings.Contains(out, "--non-interactive") {
		t.Fatalf("schedule: %v\n%s", err, out)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != scheduleConfig {
		t.Fatalf("configuration changed: %v\n%s", err, data)
	}
	if entries, err := os.ReadDir(home); err != nil || len(entries) != 1 {
		t.Fatalf("schedule created files: %v, %v", entries, err)
	}
}

func TestScheduleDailyRefusesUnusableJobs(t *testing.T) {
	for _, tc := range []struct {
		name, config, want string
		flags              []string
	}{
		{"all", scheduleConfig, "does not support --all", []string{"--all"}},
		{"time", scheduleConfig, "HH:MM", []string{"--at", "25:00"}},
		{"unknown profile", scheduleConfig, "not found", []string{"--profile", "missing"}},
		{"relative source", strings.Replace(scheduleConfig, "/offline/source/", "source/", 1), "absolute path", nil},
		{"relative destination", strings.Replace(scheduleConfig, "/offline/backup", "backup", 1), "absolute path", nil},
		{"no daily", strings.Replace(scheduleConfig, `"daily"`, `"weekly"`, 1), "no daily backup", nil},
		{"dry run", strings.Replace(scheduleConfig, `"archive":true`, `"archive":true,"dryRun":true`, 1), "dry run", nil},
		{"config error", `{"profiles":`, "configuration", nil},
		{"binary missing", scheduleConfig, "goback executable", []string{"--binary", "/nonexistent/goback"}},
		{"relative binary", scheduleConfig, "absolute path", []string{"--binary", "../go.mod"}},
		{"relative companion", strings.Replace(scheduleConfig, `"rsync":`, `"dailyCompanions":[{"id":"x","command":["./backup"]}],"rsync":`, 1), "executable must be", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := profileCommand(t, tc.config, append([]string{"schedule", "daily", "--exec-path", "/usr/bin:/bin"}, tc.flags...)...)
			if err == nil || !strings.Contains(out, tc.want) || strings.Contains(out, "<?xml") || strings.Contains(out, "recreate") {
				t.Fatalf("schedule: %v\n%s", err, out)
			}
		})
	}
}
