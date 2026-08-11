package run

import (
	"context"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
)

// MainStatus classifies the outcome of the main rsync command of a backup.
// Companions only run when the main command was actually attempted, so this
// distinction is what keeps every path deterministic.
type MainStatus int

const (
	// MainSkipped means the command was never reached: the build failed, its
	// paths did not validate, or rsync is not installed.
	MainSkipped MainStatus = iota
	// MainDeclined means the user answered no at the confirmation prompt.
	MainDeclined
	// MainInterrupted means the run was cancelled by a signal.
	MainInterrupted
	// MainFailed means rsync ran and exited non-zero.
	MainFailed
	// MainSucceeded means rsync ran and exited zero.
	MainSucceeded
)

func (s MainStatus) String() string {
	switch s {
	case MainDeclined:
		return "declined"
	case MainInterrupted:
		return "interrupted"
	case MainFailed:
		return "failed"
	case MainSucceeded:
		return "succeeded"
	default:
		return "skipped"
	}
}

// MainResult is the outcome of one main backup command.
type MainResult struct {
	BackupType string
	Status     MainStatus
	ExitCode   int
	Duration   time.Duration
	Err        error
}

// Attempted reports whether rsync actually ran, whatever it exited with.
func (m MainResult) Attempted() bool {
	return m.Status == MainSucceeded || m.Status == MainFailed
}

// mainCommand is the part of the rsync builder the orchestration drives, so
// the orchestration can be exercised without running rsync.
type mainCommand interface {
	PrintCommandToRunWithConfirmation() bool
	Execute(ctx context.Context) buildcmd.ExecutionResult
}

// backupSteps is one main backup expressed as its replaceable steps.
type backupSteps struct {
	backupType   string
	requireRsync func() error
	build        func() (mainCommand, error)
}

// DailyBackup runs the daily rsync and then, only if rsync was actually
// attempted, the companions configured for the active profile. Companion
// outcomes are added to the report but never change the returned error, which
// is the main backup's status alone.
func DailyBackup(ctx context.Context, report *Report) error {
	return runBackup(ctx, report, stepsFor(models.Daily{}, buildcmd.BuildDaily), true)
}

// WeeklyBackup runs the weekly rsync. Companions are a daily concern.
func WeeklyBackup(ctx context.Context, report *Report) error {
	return runBackup(ctx, report, stepsFor(models.Weekly{}, buildcmd.BuildWeekly), false)
}

// MonthlyBackup runs the monthly rsync. Companions are a daily concern.
func MonthlyBackup(ctx context.Context, report *Report) error {
	return runBackup(ctx, report, stepsFor(models.Monthly{}, buildcmd.BuildMonthly), false)
}

func stepsFor(backupType models.BackupTypes, build func() (*buildcmd.RsyncBuilder, error)) backupSteps {
	return backupSteps{
		backupType:   backupType.String(),
		requireRsync: buildcmd.RequireRsync,
		build: func() (mainCommand, error) {
			c, err := build()
			if err != nil {
				return nil, err
			}
			return c, nil
		},
	}
}

func runBackup(ctx context.Context, report *Report, steps backupSteps, withCompanions bool) error {
	var companions []config.Companion
	if withCompanions {
		// A companion list that cannot be read means the user asked for
		// something this run would silently not do, so it fails the profile
		// before rsync starts rather than after it.
		var err error
		companions, err = config.DailyCompanions()
		if err != nil {
			result := MainResult{BackupType: steps.backupType, Status: MainSkipped, Err: err}
			report.Mains = append(report.Mains, result)
			return err
		}
	}

	main := runMain(ctx, steps)
	report.Mains = append(report.Mains, main)
	if main.Attempted() {
		runCompanions(ctx, report, companions, buildcmd.IsDryRun(steps.backupType))
	}
	return main.Err
}

func runMain(ctx context.Context, steps backupSteps) MainResult {
	result := MainResult{BackupType: steps.backupType}

	if ctx.Err() != nil {
		result.Status = MainInterrupted
		result.Err = fmt.Errorf("backup interrupted")
		return result
	}

	c, err := steps.build()
	if err != nil {
		result.Status = MainSkipped
		result.Err = err
		return result
	}

	if err := steps.requireRsync(); err != nil {
		result.Status = MainSkipped
		result.Err = err
		return result
	}

	if !c.PrintCommandToRunWithConfirmation() {
		result.Status = MainDeclined
		return result
	}

	execution := c.Execute(ctx)
	result.ExitCode = execution.ExitCode
	result.Duration = execution.Duration
	result.Err = execution.Err
	switch {
	case execution.Interrupted:
		result.Status = MainInterrupted
	case execution.Err != nil:
		result.Status = MainFailed
	default:
		result.Status = MainSucceeded
	}
	return result
}

func runCompanions(ctx context.Context, report *Report, companions []config.Companion, dryRun bool) {
	for _, companion := range companions {
		result := RunCompanion(ctx, companion, CompanionOptions{DryRun: dryRun})
		RecordCompanion(result, config.ActiveProfileName)
		report.Companions = append(report.Companions, result)
		if ctx.Err() != nil {
			return
		}
	}
}
