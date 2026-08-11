package run

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/spf13/viper"
)

// installPhotosBackup writes an executable named photos-backup into a directory
// prepended to PATH, so a companion configured as the bare console script name
// resolves exactly as it does after an editable install. It records the path it
// was resolved to and the arguments it received, then exits with exitCode.
func installPhotosBackup(t *testing.T, exitCode int) string {
	t.Helper()

	dir := t.TempDir()
	log := filepath.Join(dir, "invocation")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$0\" \"$@\" > %q\nexit %d\n", log, exitCode)
	if err := os.WriteFile(filepath.Join(dir, "photos-backup"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// invocation returns the resolved program path and the arguments the installed
// CLI fixture was called with.
func invocation(t *testing.T, log string) (string, []string) {
	t.Helper()

	content, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the installed photos-backup was never invoked: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	return lines[0], lines[1:]
}

// photosBackupEntry is the companion configuration this plan deploys, spelled
// exactly as it appears in ~/.goback.json.
func photosBackupEntry() map[string]any {
	return map[string]any{
		"id":         "apple-photos",
		"name":       "Apple Photos",
		"command":    []any{"photos-backup", "daily"},
		"dryRunArgs": []any{"--dry-run"},
	}
}

func TestInstalledPhotosBackupCompanionPreservesMainStatus(t *testing.T) {
	failedRsync := buildcmd.ExecutionResult{ExitCode: 23, Duration: time.Second, Err: errors.New("exit status 23")}
	succeededRsync := buildcmd.ExecutionResult{Duration: time.Second}

	cases := []struct {
		name          string
		execution     buildcmd.ExecutionResult
		companionExit int
		wantStatus    MainStatus
		wantErr       bool
	}{
		{"clean run", succeededRsync, 0, MainSucceeded, false},
		{"companion needs a person", succeededRsync, 3, MainSucceeded, false},
		{"companion failed", succeededRsync, 1, MainSucceeded, false},
		{"rsync failed, companion clean", failedRsync, 0, MainFailed, true},
		{"rsync failed, companion needs a person", failedRsync, 3, MainFailed, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := installPhotosBackup(t, tc.companionExit)
			useCompanions(t, photosBackupEntry())

			steps := &fakeSteps{main: &fakeMain{confirm: true, execution: tc.execution}}
			report := &Report{}
			err := runBackup(context.Background(), report, steps.steps(), true)

			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if report.Mains[0].Status != tc.wantStatus {
				t.Fatalf("main = %+v, want %s", report.Mains[0], tc.wantStatus)
			}

			program, args := invocation(t, log)
			if filepath.Base(program) != "photos-backup" || !filepath.IsAbs(program) {
				t.Fatalf("program = %q, want the installed photos-backup resolved through PATH", program)
			}
			if len(args) != 1 || args[0] != "daily" {
				t.Fatalf("args = %v, want the daily subcommand alone", args)
			}

			companion := report.Companions[0]
			if companion.ExitCode != tc.companionExit {
				t.Fatalf("companion exit code = %d, want %d", companion.ExitCode, tc.companionExit)
			}
			if companion.PID <= 0 || companion.PID == os.Getpid() {
				t.Fatalf("companion PID = %d, want a separate process", companion.PID)
			}
			if companion.CommandString() != "photos-backup daily" {
				t.Fatalf("CommandString() = %q, want the configured argument vector", companion.CommandString())
			}
		})
	}
}

func TestInstalledPhotosBackupCompanionIsRecordedInHistory(t *testing.T) {
	installPhotosBackup(t, 3)
	useCompanions(t, photosBackupEntry())

	steps := &fakeSteps{main: &fakeMain{confirm: true}}
	if err := runBackup(context.Background(), &Report{}, steps.steps(), true); err != nil {
		t.Fatal(err)
	}

	var backupType, command, profile string
	var exitCode int
	db.QueryRows("SELECT backup_type, command, profile, exit_code FROM backups", func(rows *sql.Rows) {
		if !rows.Next() {
			t.Fatal("no history row recorded for the companion")
		}
		if err := rows.Scan(&backupType, &command, &profile, &exitCode); err != nil {
			t.Fatal(err)
		}
		if rows.Next() {
			t.Fatal("more than one history row recorded")
		}
	})

	if backupType != "companion/apple-photos" || command != "photos-backup daily" {
		t.Fatalf("backup_type = %q, command = %q", backupType, command)
	}
	if profile != "test" || exitCode != 3 {
		t.Fatalf("profile = %q, exit_code = %d, want test and 3", profile, exitCode)
	}
}

func TestInstalledPhotosBackupDryRunPassesTheFlagAndRecordsNothing(t *testing.T) {
	log := installPhotosBackup(t, 0)
	useCompanions(t, photosBackupEntry())
	viper.Set("cliDryRun", true)

	steps := &fakeSteps{main: &fakeMain{confirm: true}}
	report := &Report{}
	if err := runBackup(context.Background(), report, steps.steps(), true); err != nil {
		t.Fatal(err)
	}

	_, args := invocation(t, log)
	if strings.Join(args, " ") != "daily --dry-run" {
		t.Fatalf("args = %v, want the dry-run flag appended to daily", args)
	}
	if !report.Companions[0].Succeeded() {
		t.Fatalf("companion = %+v, want a successful dry run", report.Companions[0])
	}

	var count int
	db.QueryRows("SELECT COUNT(*) FROM backups", func(rows *sql.Rows) {
		if !rows.Next() {
			t.Fatal("no count returned")
		}
		if err := rows.Scan(&count); err != nil {
			t.Fatal(err)
		}
	})
	if count != 0 {
		t.Fatalf("recorded %d rows, want none for a dry run", count)
	}
}
