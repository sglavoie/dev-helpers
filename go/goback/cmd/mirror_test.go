package cmd

import (
	"bytes"
	"database/sql"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/mirror"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// parseFlags parses a command line the way a real invocation does, which is
// also what merges the root's persistent flags into the command, and restores
// every flag afterwards so one test never leaks into the next.
func parseFlags(t *testing.T, cmd *cobra.Command, args ...string) {
	t.Helper()

	t.Cleanup(func() {
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if !flag.Changed {
				return
			}
			if err := flag.Value.Set(flag.DefValue); err != nil {
				t.Fatal(err)
			}
			flag.Changed = false
		})
	})
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
}

func TestMirrorSkipsProfileResolution(t *testing.T) {
	withAllProfiles(t, false)

	if needsProfileResolution(mirrorCmd) {
		t.Fatal("goback mirror requires profile resolution, want it to run on a machine whose hostname matches no profile")
	}
}

// The profile flags are declared on the root command, so the mirror has to
// reject them explicitly instead of silently ignoring what the user asked for.
func TestMirrorRejectsProfileSelectionFlags(t *testing.T) {
	cases := []struct {
		name string
		flag string
		args []string
	}{
		{name: "profile", flag: "profile", args: []string{"--dry-run", "--profile", "macbook"}},
		{name: "all", flag: "all", args: []string{"--dry-run", "--all"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parseFlags(t, mirrorCmd, tc.args...)

			err := rejectProfileFlags(mirrorCmd)
			if err == nil {
				t.Fatalf("--%s was accepted, want it rejected", tc.flag)
			}
			if !strings.Contains(err.Error(), "--"+tc.flag) {
				t.Fatalf("error = %q, want it to name --%s", err, tc.flag)
			}
		})
	}
}

func TestMirrorAcceptsAnUnusedProfileFlag(t *testing.T) {
	parseFlags(t, mirrorCmd, "--dry-run")

	if err := rejectProfileFlags(mirrorCmd); err != nil {
		t.Fatal(err)
	}
}

// A real mirror reads its endpoints from the configuration alone, so an
// unconfigured mirror stops before any drive is touched.
func TestMirrorWithoutConfigurationStopsBeforeTouchingAnything(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	err := runMirror(mirrorCmd)
	if err == nil {
		t.Fatal("an unconfigured mirror ran, want it refused")
	}
	if !strings.Contains(err.Error(), config.MirrorKey+".source") {
		t.Fatalf("error = %q, want it to name the missing configuration", err)
	}
}

// The question is the last thing between a reviewed plan and a destination
// losing content, so it has to say what is about to happen.
func TestMirrorApprovalQuestionNamesTheDestruction(t *testing.T) {
	plan := mirror.Plan{
		Endpoints: mirror.Endpoints{Source: "/Volumes/SanDisk/Media", Destination: "/Volumes/Elements/Media"},
		Changes:   mirror.Changes{Created: 1, Deleted: 4},
	}

	question := mirrorApprovalQuestion(plan)
	for _, want := range []string{"/Volumes/SanDisk/Media", "/Volumes/Elements/Media", "deleting the 4 entries"} {
		if !strings.Contains(question, want) {
			t.Fatalf("question = %q, want it to mention %q", question, want)
		}
	}

	plan.Changes.Deleted = 0
	if question := mirrorApprovalQuestion(plan); strings.Contains(question, "deleting") {
		t.Fatalf("question = %q, want no deletion mentioned when nothing would be deleted", question)
	}
}

type historyRow struct {
	createdAt     string
	backupType    string
	executionTime string
	command       string
	profile       string
	exitCode      int
}

func readHistory(t *testing.T) []historyRow {
	t.Helper()

	var rows []historyRow
	db.QueryRows("SELECT created_at, backup_type, execution_time, command, profile, exit_code FROM backups ORDER BY id", func(r *sql.Rows) {
		for r.Next() {
			var got historyRow
			if err := r.Scan(&got.createdAt, &got.backupType, &got.executionTime, &got.command, &got.profile, &got.exitCode); err != nil {
				t.Fatal(err)
			}
			rows = append(rows, got)
		}
	})
	return rows
}

func attemptedResult(status mirror.Status, exitCode int) mirror.Result {
	return mirror.Result{
		Status:    status,
		Command:   mirror.Command{Argv: []string{"rsync", "--archive", "/Volumes/SanDisk/Media/", "."}, Dir: "/.vol/1/2"},
		ExitCode:  exitCode,
		StartedAt: time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC),
		Duration:  90 * time.Second,
	}
}

// Every outcome that actually started rsync is one history row, whatever rsync
// then did.
func TestRecordMirrorRecordsEveryAttempt(t *testing.T) {
	cases := []struct {
		name     string
		result   mirror.Result
		exitCode int
	}{
		{name: "succeeded", result: attemptedResult(mirror.StatusSucceeded, 0), exitCode: 0},
		{name: "failed", result: attemptedResult(mirror.StatusFailed, 23), exitCode: 23},
		{name: "interrupted", result: attemptedResult(mirror.StatusInterrupted, mirror.InterruptedExitCode), exitCode: mirror.InterruptedExitCode},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			recordMirror(tc.result)

			rows := readHistory(t)
			want := historyRow{
				createdAt:     "2026-08-11 09:30:00",
				backupType:    "mirror",
				executionTime: "1m30s",
				command:       tc.result.CommandString(),
				profile:       "global",
				exitCode:      tc.exitCode,
			}
			if len(rows) != 1 {
				t.Fatalf("recorded %d rows, want exactly 1", len(rows))
			}
			if rows[0] != want {
				t.Fatalf("row = %+v, want %+v", rows[0], want)
			}
		})
	}
}

// Nothing that never ran rsync may leave a trace in history: a failed
// preflight, an unreviewed change, a decline, an already-matching destination,
// and an interruption before the transfer all record nothing. A dry run cannot
// even reach this function, since runMirror returns before it.
func TestRecordMirrorIgnoresEveryNonAttempt(t *testing.T) {
	cases := []struct {
		name   string
		result mirror.Result
	}{
		{name: "skipped", result: mirror.Result{Status: mirror.StatusSkipped}},
		{name: "up to date", result: mirror.Result{Status: mirror.StatusUpToDate}},
		{name: "declined", result: mirror.Result{Status: mirror.StatusDeclined}},
		{name: "interrupted before the transfer", result: mirror.Result{Status: mirror.StatusInterrupted, Err: mirror.ErrInterrupted}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			recordMirror(tc.result)

			if rows := readHistory(t); len(rows) != 0 {
				t.Fatalf("recorded %d rows, want none: %+v", len(rows), rows)
			}
		})
	}
}

// A dry run returns before the recorder exists, and a preflight that fails
// never reaches it either, so neither leaves a row behind.
func TestMirrorDryRunAndFailedPreflightRecordNothing(t *testing.T) {
	for _, args := range [][]string{{"--dry-run"}, nil} {
		t.Run(strings.Join(append([]string{"mirror"}, args...), " "), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.Set(config.MirrorKey+".source", home+"/absent-source")
			viper.Set(config.MirrorKey+".destination", home+"/absent-destination")

			parseFlags(t, mirrorCmd, args...)

			if err := runMirror(mirrorCmd); err == nil {
				t.Fatal("the mirror ran against a missing source, want it refused")
			}
			if rows := readHistory(t); len(rows) != 0 {
				t.Fatalf("recorded %d rows, want none: %+v", len(rows), rows)
			}
		})
	}
}

// A history write is bookkeeping: a database that cannot take the row warns and
// leaves the mirror's own outcome alone.
func TestRecordMirrorSurvivesAnUnwritableHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	unusable, err := sql.Open("sqlite3", home+"/.goback.db")
	if err != nil {
		t.Fatal(err)
	}
	// An extra column no migration adds makes the insert fail without
	// breaking the connection itself.
	if _, err := unusable.Exec("CREATE TABLE backups (id INTEGER PRIMARY KEY, created_at TEXT, backup_type TEXT, execution_time TEXT, command TEXT, profile TEXT, exit_code INTEGER, unexpected TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err := unusable.Close(); err != nil {
		t.Fatal(err)
	}

	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	recordMirror(attemptedResult(mirror.StatusSucceeded, 0))

	if !strings.Contains(logged.String(), "failed to record backup in history") {
		t.Fatalf("log = %q, want a history warning", logged.String())
	}
	if rows := readHistory(t); len(rows) != 0 {
		t.Fatalf("recorded %d rows on an unusable database, want none", len(rows))
	}
}

// run's --dry-run is bound to viper as cliDryRun, which every snapshot builder
// reads. The mirror's own flag must stay local so it cannot turn a snapshot
// into a dry run or be turned on by one.
func TestMirrorDryRunFlagIsNotBoundToViper(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	parseFlags(t, mirrorCmd, "--dry-run")

	if viper.GetBool("cliDryRun") {
		t.Fatal("cliDryRun = true, want the mirror's --dry-run to stay local to the mirror")
	}
}
