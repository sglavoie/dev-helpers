package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
)

// companionKillGrace is how long a companion has to exit after the context is
// cancelled and it has been sent SIGTERM, before it is killed.
const companionKillGrace = 5 * time.Second

// CompanionOptions configures a single companion execution.
type CompanionOptions struct {
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
}

// CompanionResult is the outcome of a single companion execution. A companion
// never fails the backup it accompanies, so every outcome is reported here
// instead of being returned as an error.
type CompanionResult struct {
	Companion   config.Companion
	Argv        []string
	DryRun      bool
	Started     bool
	PID         int
	Start       time.Time
	Duration    time.Duration
	ExitCode    int
	Interrupted bool
	Err         error
}

// Succeeded reports whether the companion ran to completion with exit code 0.
func (r CompanionResult) Succeeded() bool {
	return r.Started && !r.Interrupted && r.Err == nil && r.ExitCode == 0
}

// CommandString renders the executed argument vector for display and history.
func (r CompanionResult) CommandString() string {
	return formatArgv(r.Argv)
}

// Status is the one-word outcome used in the combined result table. A dry run
// of a companion that declares no dryRunArgs is a skip rather than a failure,
// because nothing was meant to run.
func (r CompanionResult) Status() string {
	switch {
	case !r.Started && r.DryRun && len(r.Companion.DryRunArgs) == 0:
		return "skipped"
	case !r.Started:
		return "not started"
	case r.Interrupted:
		return "interrupted"
	case r.ExitCode != 0:
		return "failed"
	default:
		return "succeeded"
	}
}

// Summary is a one-line description of the outcome.
func (r CompanionResult) Summary() string {
	name := r.Companion.Name
	switch {
	case !r.Started:
		return fmt.Sprintf("%s: not started (%v)", name, r.Err)
	case r.Interrupted:
		return fmt.Sprintf("%s: interrupted after %s", name, r.Duration.Round(time.Millisecond))
	case r.ExitCode != 0:
		return fmt.Sprintf("%s: failed with exit code %d after %s", name, r.ExitCode, r.Duration.Round(time.Millisecond))
	default:
		return fmt.Sprintf("%s: succeeded in %s", name, r.Duration.Round(time.Millisecond))
	}
}

// RunCompanion executes a companion without a shell and returns its outcome.
// The context cancels the companion: it is sent SIGTERM and killed if it has
// not exited within companionKillGrace.
func RunCompanion(ctx context.Context, companion config.Companion, opts CompanionOptions) CompanionResult {
	result := CompanionResult{
		Companion: companion,
		Argv:      companion.Argv(opts.DryRun),
		DryRun:    opts.DryRun,
	}

	// Running the real command during a dry run would be worse than skipping
	// the companion entirely.
	if opts.DryRun && len(companion.DryRunArgs) == 0 {
		result.Err = fmt.Errorf("companion %q has no dryRunArgs, so it cannot be run as a dry run", companion.ID)
		return result
	}

	if _, err := exec.LookPath(result.Argv[0]); err != nil {
		result.Err = fmt.Errorf("%s not found in PATH: %w", result.Argv[0], err)
		return result
	}

	cmd := exec.CommandContext(ctx, result.Argv[0], result.Argv[1:]...)
	cmd.Stdout = writerOrDefault(opts.Stdout, os.Stdout)
	cmd.Stderr = writerOrDefault(opts.Stderr, os.Stderr)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = companionKillGrace

	result.Start = time.Now()
	err := cmd.Start()
	if err != nil {
		result.Duration = time.Since(result.Start)
		result.Err = fmt.Errorf("failed to start %s: %w", companion.Name, err)
		return result
	}

	result.Started = true
	result.PID = cmd.Process.Pid
	err = cmd.Wait()
	result.Duration = time.Since(result.Start)
	result.Interrupted = ctx.Err() != nil

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		result.ExitCode = 0
	case errors.As(err, &exitErr):
		result.ExitCode = exitErr.ExitCode()
		result.Err = err
	default:
		result.ExitCode = 1
		result.Err = err
	}
	return result
}

// RecordCompanion appends a completed companion run to the backup history.
// A dry run and a companion that never started record nothing, which is how
// rsync history already behaves.
func RecordCompanion(result CompanionResult, profile string) {
	if result.DryRun || !result.Started {
		return
	}

	db.RecordBackup(db.HistoryEntry{
		CreatedAt:     result.Start,
		BackupType:    db.CompanionBackupType(result.Companion.ID),
		ExecutionTime: result.Duration.String(),
		Command:       result.CommandString(),
		Profile:       profile,
		ExitCode:      result.ExitCode,
	})
}

// PrintCompanions lists the companions a daily run of the active profile would
// execute, with the exact argument vector of a real run and of a dry run.
func PrintCompanions(w io.Writer) error {
	companions, err := config.DailyCompanions()
	if err != nil {
		return err
	}
	if len(companions) == 0 {
		return nil
	}

	fmt.Fprintf(w, "\nDaily companions:\n")
	for _, companion := range companions {
		fmt.Fprintf(w, "  %s (%s): %s\n", companion.Name, companion.ID, formatArgv(companion.Argv(false)))
		if len(companion.DryRunArgs) == 0 {
			fmt.Fprintf(w, "    dry run: skipped, no dryRunArgs configured\n")
			continue
		}
		fmt.Fprintf(w, "    dry run: %s\n", formatArgv(companion.Argv(true)))
	}
	return nil
}

func writerOrDefault(w io.Writer, fallback io.Writer) io.Writer {
	if w == nil {
		return fallback
	}
	return w
}

func formatArgv(argv []string) string {
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		if strings.ContainsAny(arg, " \t\n\"'") {
			quoted = append(quoted, fmt.Sprintf("%q", arg))
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}
