package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/diagnostics"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/mirror"
)

func TestStatusChecksFilterBackupsAndKeepJSONOnFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	content := `{"profiles":{"test":{"source":"/offline/source","destination":"/offline/backup","rsync":{"daily":{"archive":true},"weekly":{"archive":true},"monthly":{"archive":true}},"dailyCompanions":[{"id":"photos","command":["photos-backup"]},{"id":"new","command":["new-backup"]}]}},"mirror":{"source":"/offline/a","destination":"/offline/b"}}`
	path := filepath.Join(home, "config.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, row := range []db.HistoryEntry{
		{Profile: "test", BackupType: "daily", CreatedAt: now.Add(-time.Hour)},
		{Profile: "test", BackupType: "weekly", CreatedAt: now.Add(-10 * 24 * time.Hour)},
		{Profile: "test", BackupType: "monthly", CreatedAt: now.Add(-time.Hour), ExitCode: -1},
		{Profile: "test", BackupType: "companion/photos", CreatedAt: now.Add(-2 * time.Hour)},
		{Profile: "test", BackupType: "companion/photos", CreatedAt: now.Add(-time.Hour), ExitCode: 3},
		{Profile: "retired", BackupType: "daily", CreatedAt: now},
		{Profile: "test", BackupType: "companion/retired", CreatedAt: now},
	} {
		db.RecordBackup(row)
	}
	for _, tc := range []struct {
		name  string
		flags []string
		fail  bool
		count int
	}{
		{"ordinary status", nil, false, 6},
		{"healthy daily", []string{"--daily", "--older-than", "48h", "--check"}, false, 1},
		{"stale informational", []string{"--weekly", "--older-than", "48h"}, false, 1},
		{"stale check", []string{"--weekly", "--older-than", "48h", "--check"}, true, 1},
		{"no age assessment", []string{"--weekly", "--check"}, false, 1},
		{"interrupted", []string{"--monthly", "--check"}, true, 1},
		{"never run", []string{"--mirror", "--check"}, true, 1},
		{"companion failure and unrun", []string{"--companions", "--check"}, true, 2},
		{"empty check", []string{"--mirror", "--profile", "test", "--check"}, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			args := append([]string{"-test.run=^TestProfileCommandProcess$", "--", "--config", path, "status", "--json"}, tc.flags...)
			command := exec.CommandContext(ctx, os.Args[0], args...)
			command.Env = append(os.Environ(), "GOBACK_TEST_COMMAND=1")
			var stderr bytes.Buffer
			command.Stderr = &stderr
			output, err := command.Output()
			if (err != nil) != tc.fail {
				t.Fatalf("exit %v: %s", err, &stderr)
			}
			var report statusReport
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, output)
			}
			rows := report.Backups
			if len(rows) != tc.count {
				t.Fatalf("rows: %s", output)
			}
			for _, row := range rows {
				if row.Profile == "retired" || row.BackupType == "companion/retired" {
					t.Fatalf("retired configuration included: %+v", row)
				}
				if row.BackupType == "companion/photos" && (row.LastSuccess == nil || row.ExitCode == nil || *row.ExitCode != 3) {
					t.Fatalf("companion failure hid success: %+v", row)
				}
				if row.BackupType == "companion/new" && (row.LatestAttempt != nil || row.Result != "never run") {
					t.Fatalf("unrun companion: %+v", row)
				}
			}
		})
	}
	out, err := profileCommand(t, content, "status", "--daily", "--weekly")
	if err == nil || !strings.Contains(out, "none of the others") {
		t.Fatalf("conflicting selectors: %v, %s", err, out)
	}
}

func TestLogCleanupWorksWithoutConfigurationAndPreviewsCurrentLogs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tail := &diagnostics.Tail{}
	path, err := tail.Save("test", "daily", "rsync", 23)
	if err != nil {
		t.Fatal(err)
	}
	out := globalCommand(t, home, "clean", "logs", "--keep", "0", "--dry-run")
	if !strings.Contains(out, "Would remove") || !strings.Contains(out, path) {
		t.Fatal(out)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	// A broken backup configuration must not prevent log maintenance.
	if err := os.WriteFile(filepath.Join(home, ".goback.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	out = globalCommand(t, home, "clean", "logs", "--keep", "0")
	if !strings.Contains(out, "Removed") {
		t.Fatal(out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("log was not removed")
	}
	for _, flags := range [][]string{{"--keep", "-1"}, {"--profile", "test"}, {"--all"}} {
		if out, err := profileCommand(t, `{}`, append([]string{"clean", "logs"}, flags...)...); err == nil {
			t.Fatalf("accepted %v: %s", flags, out)
		}
	}
}

func TestMirrorDiagnosticsCaptureTransferStreamsAndKeepOutcome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	tail := &diagnostics.Tail{}
	deps := mirror.OSExecDeps(nil, io.MultiWriter(&stdout, tail), io.MultiWriter(&stderr, tail))
	command := mirror.Command{Argv: []string{"/bin/sh", "-c", "echo transfer-output; echo transfer-error >&2; exit 23"}, Dir: home}
	code, err := deps.Streamer.Stream(context.Background(), command)
	if err != nil || code != 23 {
		t.Fatalf("stream: %d, %v", code, err)
	}
	result := mirror.Result{Status: mirror.StatusFailed, Command: command, ExitCode: code, Err: fmt.Errorf("transfer failed")}
	saveMirrorDiagnostics(result, tail, &stdout, &stderr)
	paths, err := diagnostics.LogPaths()
	if err != nil || len(paths) != 1 {
		t.Fatalf("logs: %v, %v", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"transfer-output", "transfer-error", `Profile: "global"`, "Backup type: mirror", "Exit code: 23", command.String()} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %q in %s", want, data)
		}
	}
	if !strings.Contains(stdout.String(), paths[0]) {
		t.Fatal("log path not reported")
	}
	for _, status := range []mirror.Status{mirror.StatusSkipped, mirror.StatusUpToDate, mirror.StatusDeclined, mirror.StatusSucceeded, mirror.StatusInterrupted} {
		saveMirrorDiagnostics(mirror.Result{Status: status}, tail, &stdout, &stderr)
	}
	paths, err = diagnostics.LogPaths()
	if err != nil || len(paths) != 1 {
		t.Fatalf("unattempted outcomes produced logs: %v, %v", paths, err)
	}
	// A diagnostic write error must only warn, preserving the transfer result.
	if err := os.RemoveAll(filepath.Join(home, ".goback")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".goback"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	saveMirrorDiagnostics(result, tail, &stdout, &stderr)
	if result.ExitCode != 23 || !strings.Contains(stderr.String(), "warning:") {
		t.Fatal("logging changed outcome or failed silently")
	}
}

func TestDerivedPreviewShowsDailyHistoryOffline(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "config.json")
	content := `{"profiles":{"test":{"destination":"/offline/backup","rsync":{"weekly":{"archive":true},"monthly":{"archive":true}}}}}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	db.RecordBackup(db.HistoryEntry{Profile: "test", BackupType: "daily", CreatedAt: time.Now().Add(-9*24*time.Hour - time.Minute)})
	db.RecordBackup(db.HistoryEntry{Profile: "test", BackupType: "daily", CreatedAt: time.Now(), ExitCode: 23})
	for _, kind := range []string{"weekly", "monthly"} {
		out := globalCommand(t, home, "--config", path, "preview", kind)
		for _, want := range []string{"Copying the existing daily backup", "9 days ago", "failed (exit 23)", "partially updated", "Full command:"} {
			if !strings.Contains(out, want) {
				t.Fatalf("%s missing %q: %s", kind, want, out)
			}
		}
	}
}
