package run

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
)

// helperArgs returns the arguments a companion passed to the helper process,
// or nil when the test binary is running the suite normally.
func helperArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" {
			return os.Args[i+1:]
		}
	}
	return nil
}

// TestCompanionHelperProcess is not a test: it is the program the companion
// tests execute, so that they need no external binary.
func TestCompanionHelperProcess(t *testing.T) {
	args := helperArgs()
	if len(args) == 0 {
		t.Skip("not running as a companion helper process")
	}

	switch args[0] {
	case "echo":
		fmt.Fprint(os.Stdout, strings.Join(args[1:], " "))
		fmt.Fprint(os.Stderr, "helper stderr")
		os.Exit(0)
	case "exit":
		code, err := strconv.Atoi(args[1])
		if err != nil {
			os.Exit(70)
		}
		os.Exit(code)
	case "sleep":
		ms, err := strconv.Atoi(args[1])
		if err != nil {
			os.Exit(70)
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		os.Exit(0)
	case "touch":
		if err := os.WriteFile(args[1], []byte("ran"), 0o644); err != nil {
			os.Exit(71)
		}
		os.Exit(0)
	default:
		os.Exit(72)
	}
}

func helperCompanion(t *testing.T, args ...string) config.Companion {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := append([]string{self, "-test.run=TestCompanionHelperProcess", "--"}, args...)
	return config.Companion{ID: "helper", Name: "Helper", Command: command}
}

func TestRunCompanionStreamsAndReportsSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	companion := helperCompanion(t, "echo", "hello")

	result := RunCompanion(context.Background(), companion, CompanionOptions{Stdout: &stdout, Stderr: &stderr})

	if !result.Succeeded() {
		t.Fatalf("result = %+v, want success", result)
	}
	if stdout.String() != "hello" {
		t.Fatalf("stdout = %q, want the streamed helper output", stdout.String())
	}
	if stderr.String() != "helper stderr" {
		t.Fatalf("stderr = %q, want the streamed helper error output", stderr.String())
	}
	if !result.Started || result.PID <= 0 {
		t.Fatalf("Started = %v, PID = %d, want a started process", result.Started, result.PID)
	}
	if result.Start.IsZero() || result.Duration <= 0 {
		t.Fatalf("Start = %v, Duration = %v, want both recorded", result.Start, result.Duration)
	}
	if result.Interrupted || result.Err != nil || result.ExitCode != 0 {
		t.Fatalf("result = %+v, want an uninterrupted zero exit", result)
	}
	if !strings.Contains(result.Summary(), "Helper: succeeded in ") {
		t.Fatalf("Summary() = %q", result.Summary())
	}
}

func TestRunCompanionReportsExitCodeWithoutFataling(t *testing.T) {
	companion := helperCompanion(t, "exit", "3")

	result := RunCompanion(context.Background(), companion, CompanionOptions{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})

	if result.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3", result.ExitCode)
	}
	if !result.Started || result.Succeeded() {
		t.Fatalf("result = %+v, want a started failure", result)
	}
	if result.Err == nil {
		t.Fatal("Err = nil, want the exit error carried on the result")
	}
	if !strings.Contains(result.Summary(), "failed with exit code 3") {
		t.Fatalf("Summary() = %q", result.Summary())
	}
}

func TestRunCompanionReportsMissingProgram(t *testing.T) {
	companion := config.Companion{ID: "missing", Name: "Missing", Command: []string{"goback-companion-that-does-not-exist"}}

	result := RunCompanion(context.Background(), companion, CompanionOptions{})

	if result.Started {
		t.Fatalf("result = %+v, want no started process", result)
	}
	if result.Err == nil || !strings.Contains(result.Err.Error(), "not found in PATH") {
		t.Fatalf("Err = %v, want a PATH error", result.Err)
	}
	if !strings.Contains(result.Summary(), "Missing: not started") {
		t.Fatalf("Summary() = %q", result.Summary())
	}
}

func TestRunCompanionIsCancelledByContext(t *testing.T) {
	companion := helperCompanion(t, "sleep", "30000")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result := RunCompanion(ctx, companion, CompanionOptions{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	elapsed := time.Since(start)

	if !result.Interrupted {
		t.Fatalf("result = %+v, want an interrupted companion", result)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("cancellation took %s, want the companion to be signalled promptly", elapsed)
	}
	if result.Succeeded() {
		t.Fatal("Succeeded() = true, want false for an interrupted companion")
	}
	if !strings.Contains(result.Summary(), "Helper: interrupted after ") {
		t.Fatalf("Summary() = %q", result.Summary())
	}
}

func TestRunCompanionAppendsDryRunArgs(t *testing.T) {
	var stdout bytes.Buffer
	companion := helperCompanion(t, "echo", "daily")
	companion.DryRunArgs = []string{"--dry-run"}

	result := RunCompanion(context.Background(), companion, CompanionOptions{DryRun: true, Stdout: &stdout, Stderr: &bytes.Buffer{}})

	if !result.Succeeded() {
		t.Fatalf("result = %+v, want success", result)
	}
	if stdout.String() != "daily --dry-run" {
		t.Fatalf("stdout = %q, want the dry-run argument appended", stdout.String())
	}
}

func TestRunCompanionRefusesDryRunWithoutDryRunArgs(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	companion := helperCompanion(t, "touch", marker)

	result := RunCompanion(context.Background(), companion, CompanionOptions{DryRun: true})

	if result.Started {
		t.Fatalf("result = %+v, want the companion skipped", result)
	}
	if result.Err == nil || !strings.Contains(result.Err.Error(), "dryRunArgs") {
		t.Fatalf("Err = %v, want an error naming dryRunArgs", result.Err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("the companion ran for real during a dry run")
	}
}

func TestCommandStringQuotesArgumentsWithSpaces(t *testing.T) {
	result := CompanionResult{Argv: []string{"photos-backup", "daily", "--archive", "/Volumes/SanDisk/Apple Photos"}}

	want := `photos-backup daily --archive "/Volumes/SanDisk/Apple Photos"`
	if result.CommandString() != want {
		t.Fatalf("CommandString() = %q, want %q", result.CommandString(), want)
	}
}

func TestRecordCompanionWritesHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	result := CompanionResult{
		Companion: config.Companion{ID: "apple-photos", Name: "Apple Photos"},
		Argv:      []string{"photos-backup", "daily"},
		Started:   true,
		Start:     time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC),
		Duration:  2 * time.Second,
		ExitCode:  3,
	}
	RecordCompanion(result, "macbook")

	var backupType, executionTime, command, profile string
	var exitCode int
	db.QueryRows("SELECT backup_type, execution_time, command, profile, exit_code FROM backups", func(rows *sql.Rows) {
		if !rows.Next() {
			t.Fatal("no history row recorded")
		}
		if err := rows.Scan(&backupType, &executionTime, &command, &profile, &exitCode); err != nil {
			t.Fatal(err)
		}
		if rows.Next() {
			t.Fatal("more than one history row recorded")
		}
	})

	if backupType != "companion/apple-photos" {
		t.Fatalf("backup_type = %q, want companion/apple-photos", backupType)
	}
	if executionTime != "2s" {
		t.Fatalf("execution_time = %q, want 2s", executionTime)
	}
	if command != "photos-backup daily" {
		t.Fatalf("command = %q, want the executed argument vector", command)
	}
	if profile != "macbook" || exitCode != 3 {
		t.Fatalf("profile = %q, exit_code = %d, want macbook and 3", profile, exitCode)
	}
}

func TestRecordCompanionSkipsDryRunAndUnstartedCompanions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := CompanionResult{
		Companion: config.Companion{ID: "apple-photos"},
		Argv:      []string{"photos-backup", "daily"},
		Start:     time.Now(),
	}

	dryRun := base
	dryRun.Started = true
	dryRun.DryRun = true
	RecordCompanion(dryRun, "macbook")
	RecordCompanion(base, "macbook")

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
		t.Fatalf("recorded %d rows, want none", count)
	}
}
