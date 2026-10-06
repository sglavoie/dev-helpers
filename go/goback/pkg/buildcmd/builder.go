package buildcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/diagnostics"
	"github.com/spf13/viper"
)

// ExecutionResult reports how the rsync command ended. An interruption is
// reported separately from a failure because the two lead to different
// decisions about what may run afterwards.
type ExecutionResult struct {
	NotStarted    bool // preparation or process startup failed before rsync ran
	Interrupted   bool
	ExitCode      int
	Duration      time.Duration
	Err           error
	DiagnosticLog string
}

// RequireRsync reports whether rsync can be executed at all.
func RequireRsync() error {
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("rsync not found in PATH: %w", err)
	}
	return nil
}

// IsDryRun reports whether a backup type transfers nothing, either because its
// profile configures a dry run or because --dry-run was passed.
func IsDryRun(backupType string) bool {
	return viper.GetBool(config.ActiveProfilePrefix()+"rsync."+backupType+".dryRun") || viper.GetBool("cliDryRun")
}

func (r *builder) BuildNoCheck() {
	r.build()
}

func (r *builder) BuildCheck() error {
	if err := r.validateSettings(); err != nil {
		return err
	}
	r.build()
	return r.validateBeforeRun()
}

func (r *builder) Execute(ctx context.Context) (result ExecutionResult) {
	if ctx.Err() != nil {
		return ExecutionResult{NotStarted: true, Interrupted: true, ExitCode: -1, Err: fmt.Errorf("backup interrupted")}
	}
	ctx, release, err := r.Lock(ctx)
	if err != nil {
		return ExecutionResult{NotStarted: true, ExitCode: 1, Err: err}
	}
	defer release()
	if err := r.validateBeforeRun(); err != nil {
		return ExecutionResult{NotStarted: true, ExitCode: 1, Err: err}
	}
	if !r.dryRun {
		if err := os.MkdirAll(r.updatedDestDir, 0755); err != nil {
			return ExecutionResult{NotStarted: true, ExitCode: 1, Err: fmt.Errorf("create destination: %w", err)}
		}
	}
	cmd := exec.CommandContext(ctx, r.args[0], r.args[1:]...)
	// Capturing output uses pipes. A descendant retaining a pipe must not keep
	// cancellation (and the destination lock) waiting indefinitely.
	cmd.WaitDelay = 10 * time.Second
	r.exitCode = 0
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !r.dryRun {
		tail := &diagnostics.Tail{}
		cmd.Stdout = io.MultiWriter(os.Stdout, tail)
		cmd.Stderr = io.MultiWriter(os.Stderr, tail)
		defer func() {
			if result.Err == nil {
				return
			}
			fmt.Fprintf(tail, "\ngoback: %v\n", result.Err)
			path, err := tail.Save(config.ActiveProfileName, r.builderType.String(), r.CommandString(), result.ExitCode)
			result.DiagnosticLog = path
			if err != nil {
				log.Printf("warning: could not save or trim diagnostic logs: %v", err)
			}
		}()
	}

	start := time.Now()
	err = cmd.Start()
	started := err == nil
	if started {
		err = cmd.Wait()
	}
	duration := time.Since(start)
	r.executionTime = duration.String()

	if ctx.Err() != nil {
		fmt.Println("\nBackup interrupted.")
		r.exitCode = -1
		if started && !r.dryRun {
			r.updateDBWithUsage()
		}
		return ExecutionResult{
			NotStarted:  !started,
			Interrupted: true,
			ExitCode:    -1,
			Duration:    duration,
			Err:         fmt.Errorf("backup interrupted"),
		}
	}

	if err != nil {
		fmt.Println("Error running rsync command: ", err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			r.exitCode = exitErr.ExitCode()
		} else {
			r.exitCode = 1
		}
	}
	if started && !r.dryRun {
		r.updateDBWithUsage()
	}
	return ExecutionResult{NotStarted: !started, ExitCode: r.exitCode, Duration: duration, Err: err}
}

func (r *builder) build() {
	r.args = []string{"rsync"}
	r.dryRun = IsDryRun(r.builderType.String())
	r.appendBooleanFlags()
	r.args = append(r.args, FilterArgs(r.builderType)...)
	r.appendSrcDest()
}

func (r *builder) builderSettingsPrefix() string {
	return config.ActiveProfilePrefix() + "rsync." + r.builderType.String() + "."
}

func (r *builder) updateDBWithUsage() {
	db.RecordBackup(db.HistoryEntry{
		CreatedAt:     time.Now(),
		BackupType:    r.builderType.String(),
		ExecutionTime: r.executionTime,
		Command:       r.CommandString(),
		Profile:       config.ActiveProfileName,
		ExitCode:      r.exitCode,
	})
}
