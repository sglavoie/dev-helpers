package run

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/viper"
)

// fakeMain stands in for the rsync builder so every orchestration path can be
// driven without running rsync.
type fakeMain struct {
	confirm   bool
	execution buildcmd.ExecutionResult
	executed  bool
}

func (f *fakeMain) PrintCommandToRunWithConfirmation() bool { return f.confirm }

func (f *fakeMain) Execute(context.Context) buildcmd.ExecutionResult {
	f.executed = true
	return f.execution
}

type fakeSteps struct {
	main     *fakeMain
	buildErr error
	rsyncErr error
	built    bool
}

func (f *fakeSteps) steps() backupSteps {
	return backupSteps{
		backupType:   "daily",
		requireRsync: func() error { return f.rsyncErr },
		build: func() (mainCommand, error) {
			f.built = true
			if f.buildErr != nil {
				return nil, f.buildErr
			}
			return f.main, nil
		},
	}
}

// useCompanions configures the companions of the active profile and points the
// history database at a temporary home so no test writes to the real one.
func useCompanions(t *testing.T, entries ...map[string]any) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	raw := make([]any, 0, len(entries))
	for _, entry := range entries {
		raw = append(raw, entry)
	}
	viper.Set("profiles.test.dailyCompanions", raw)
	config.ActiveProfileName = "test"
	t.Cleanup(func() {
		viper.Reset()
		config.ActiveProfileName = ""
	})
}

// helperEntry is the configuration of a companion that re-executes the test
// binary through TestCompanionHelperProcess.
func helperEntry(t *testing.T, args ...string) map[string]any {
	t.Helper()
	companion := helperCompanion(t, args...)
	return map[string]any{"id": companion.ID, "name": companion.Name, "command": companion.Command}
}

func TestDailyOrchestrationMatrix(t *testing.T) {
	cases := []struct {
		name          string
		buildErr      error
		rsyncErr      error
		confirm       bool
		execution     buildcmd.ExecutionResult
		wantStatus    MainStatus
		wantErr       bool
		wantCompanion bool
	}{
		{
			name:       "build failure",
			buildErr:   errors.New("no rsync.daily configuration found"),
			wantStatus: MainSkipped,
			wantErr:    true,
		},
		{
			name:       "rsync unavailable before start",
			rsyncErr:   errors.New("rsync not found in PATH"),
			confirm:    true,
			wantStatus: MainSkipped,
			wantErr:    true,
		},
		{
			name:       "declined confirmation",
			wantStatus: MainDeclined,
		},
		{
			name:       "interrupted rsync",
			confirm:    true,
			execution:  buildcmd.ExecutionResult{Interrupted: true, ExitCode: -1, Duration: time.Second, Err: errors.New("backup interrupted")},
			wantStatus: MainInterrupted,
			wantErr:    true,
		},
		{
			name:          "failed rsync",
			confirm:       true,
			execution:     buildcmd.ExecutionResult{ExitCode: 23, Duration: time.Second, Err: errors.New("exit status 23")},
			wantStatus:    MainFailed,
			wantErr:       true,
			wantCompanion: true,
		},
		{
			name:          "successful rsync",
			confirm:       true,
			execution:     buildcmd.ExecutionResult{Duration: time.Second},
			wantStatus:    MainSucceeded,
			wantCompanion: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "companion-ran")
			useCompanions(t, helperEntry(t, "touch", marker))

			steps := &fakeSteps{main: &fakeMain{confirm: tc.confirm, execution: tc.execution}, buildErr: tc.buildErr, rsyncErr: tc.rsyncErr}
			report := &Report{}
			err := runBackup(context.Background(), report, steps.steps(), true)

			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if len(report.Mains) != 1 || report.Mains[0].Status != tc.wantStatus {
				t.Fatalf("mains = %+v, want a single %s result", report.Mains, tc.wantStatus)
			}
			_, statErr := os.Stat(marker)
			if ran := statErr == nil; ran != tc.wantCompanion {
				t.Fatalf("companion ran = %v, want %v", ran, tc.wantCompanion)
			}
			wantCompanions := 0
			if tc.wantCompanion {
				wantCompanions = 1
			}
			if len(report.Companions) != wantCompanions {
				t.Fatalf("companions = %+v, want %d recorded", report.Companions, wantCompanions)
			}
		})
	}
}

func TestCompanionFailureDoesNotFailTheBackup(t *testing.T) {
	useCompanions(t, helperEntry(t, "exit", "3"))

	steps := &fakeSteps{main: &fakeMain{confirm: true}}
	report := &Report{}
	err := runBackup(context.Background(), report, steps.steps(), true)

	if err != nil {
		t.Fatalf("err = %v, want a successful backup despite the companion", err)
	}
	if report.Mains[0].Status != MainSucceeded {
		t.Fatalf("main = %+v, want a success", report.Mains[0])
	}
	if len(report.Companions) != 1 || report.Companions[0].ExitCode != 3 {
		t.Fatalf("companions = %+v, want one exit code 3", report.Companions)
	}
	if report.Companions[0].Status() != "failed" {
		t.Fatalf("Status() = %q, want failed", report.Companions[0].Status())
	}
}

func TestUnreadableCompanionsFailBeforeTheBackupStarts(t *testing.T) {
	useCompanions(t, map[string]any{"id": "apple-photos", "command": "photos-backup daily"})

	steps := &fakeSteps{main: &fakeMain{confirm: true}}
	report := &Report{}
	err := runBackup(context.Background(), report, steps.steps(), true)

	if err == nil || !strings.Contains(err.Error(), "command") {
		t.Fatalf("err = %v, want the companion configuration error", err)
	}
	if steps.built || steps.main.executed {
		t.Fatal("the backup was built or executed despite an unreadable companion list")
	}
	if len(report.Mains) != 1 || report.Mains[0].Status != MainSkipped {
		t.Fatalf("mains = %+v, want a single skipped result", report.Mains)
	}
}

func TestCancelledContextSkipsEverything(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "companion-ran")
	useCompanions(t, helperEntry(t, "touch", marker))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	steps := &fakeSteps{main: &fakeMain{confirm: true}}
	report := &Report{}
	err := runBackup(ctx, report, steps.steps(), true)

	if err == nil {
		t.Fatal("err = nil, want an interrupted backup")
	}
	if steps.built {
		t.Fatal("the backup was built after cancellation")
	}
	if report.Mains[0].Status != MainInterrupted || len(report.Companions) != 0 {
		t.Fatalf("report = %+v, want an interrupted main and no companion", report)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("a companion ran after cancellation")
	}
}

func TestDryRunAppendsDryRunArgsToCompanions(t *testing.T) {
	entry := helperEntry(t, "echo", "daily")
	entry["dryRunArgs"] = []string{"--dry-run"}
	useCompanions(t, entry)
	viper.Set("cliDryRun", true)

	steps := &fakeSteps{main: &fakeMain{confirm: true}}
	report := &Report{}
	if err := runBackup(context.Background(), report, steps.steps(), true); err != nil {
		t.Fatal(err)
	}

	result := report.Companions[0]
	if !result.DryRun || !result.Succeeded() {
		t.Fatalf("result = %+v, want a successful dry run", result)
	}
	if !strings.HasSuffix(result.CommandString(), "--dry-run") {
		t.Fatalf("CommandString() = %q, want the dry-run argument appended", result.CommandString())
	}
}

func TestDryRunSkipsCompanionsWithoutDryRunArgs(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "companion-ran")
	useCompanions(t, helperEntry(t, "touch", marker))
	viper.Set("cliDryRun", true)

	steps := &fakeSteps{main: &fakeMain{confirm: true}}
	report := &Report{}
	if err := runBackup(context.Background(), report, steps.steps(), true); err != nil {
		t.Fatal(err)
	}

	if report.Companions[0].Status() != "skipped" {
		t.Fatalf("Status() = %q, want skipped", report.Companions[0].Status())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("the companion ran for real during a dry run")
	}
}

func TestProfileDryRunConfigurationAlsoAppliesToCompanions(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "companion-ran")
	useCompanions(t, helperEntry(t, "touch", marker))
	viper.Set("profiles.test.rsync.daily.dryRun", true)

	steps := &fakeSteps{main: &fakeMain{confirm: true}}
	report := &Report{}
	if err := runBackup(context.Background(), report, steps.steps(), true); err != nil {
		t.Fatal(err)
	}

	if !report.Companions[0].DryRun {
		t.Fatalf("result = %+v, want a dry run driven by the profile configuration", report.Companions[0])
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("the companion ran for real while rsync was configured as a dry run")
	}
}

func TestBackupsWithoutCompanionsRunNone(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "companion-ran")
	useCompanions(t, helperEntry(t, "touch", marker))

	steps := &fakeSteps{main: &fakeMain{confirm: true}}
	report := &Report{}
	if err := runBackup(context.Background(), report, steps.steps(), false); err != nil {
		t.Fatal(err)
	}

	if len(report.Companions) != 0 {
		t.Fatalf("companions = %+v, want none for a weekly or monthly backup", report.Companions)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("a companion ran for a backup type that has none")
	}
}

func TestReportPrintsMainAndCompanionResults(t *testing.T) {
	report := &Report{
		Mains: []MainResult{{BackupType: "daily", Status: MainSucceeded, Duration: 1500 * time.Millisecond}},
		Companions: []CompanionResult{{
			Companion: config.Companion{ID: "apple-photos", Name: "Apple Photos"},
			Argv:      []string{"photos-backup", "daily"},
			Started:   true,
			Duration:  2 * time.Second,
			ExitCode:  3,
			Err:       errors.New("exit status 3"),
		}},
	}

	var out bytes.Buffer
	report.Print(&out)

	for _, want := range []string{"daily", "succeeded", "1.5s", "companion/apple-photos", "failed", "exit code 3"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("table does not contain %q:\n%s", want, out.String())
		}
	}
}

func TestEmptyReportPrintsNothing(t *testing.T) {
	var out bytes.Buffer
	(&Report{}).Print(&out)

	if out.Len() != 0 {
		t.Fatalf("output = %q, want nothing for a run with no result", out.String())
	}
}
