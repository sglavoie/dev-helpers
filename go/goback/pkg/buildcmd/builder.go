package buildcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/spf13/viper"
)

// ExecutionResult reports how the rsync command ended. An interruption is
// reported separately from a failure because the two lead to different
// decisions about what may run afterwards.
type ExecutionResult struct {
	Interrupted bool
	ExitCode    int
	Duration    time.Duration
	Err         error
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
	if !viper.IsSet(r.builderSettingsPrefix() + "archive") {
		return fmt.Errorf("no rsync.%s configuration found for profile %q", r.builderType.String(), config.ActiveProfileName)
	}
	r.build()
	return r.validateBeforeRun()
}

func (r *builder) Execute(ctx context.Context) ExecutionResult {
	cmd := exec.CommandContext(ctx, "bash", "-c", r.CommandString())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)
	r.executionTime = duration.String()

	if ctx.Err() != nil {
		fmt.Println("\nBackup interrupted, cleaning up...")
		return ExecutionResult{
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
	if !viper.GetBool("cliDryRun") {
		r.updateDBWithUsage()
	}
	return ExecutionResult{ExitCode: r.exitCode, Duration: duration, Err: err}
}

func (r *builder) build() {
	r.initBuilder()
	r.appendBooleanFlags()
	r.appendIncludedPatterns()
	r.appendExcludedPatterns()
	r.appendSrcDest()
}

func (r *builder) builderSettingsPrefix() string {
	return config.ActiveProfilePrefix() + "rsync." + r.builderType.String() + "."
}

func (r *builder) initBuilder() {
	r.sb = &strings.Builder{}
	r.sb.WriteString("rsync")
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
