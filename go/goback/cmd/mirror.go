package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/inputs"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/mirror"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/cobra"
)

// profileFlags are the root persistent flags that select profiles. The mirror
// is a single global operation described by the top-level mirror block, so
// they are meaningless here rather than silently ignored.
var profileFlags = []string{"profile", "all"}

var mirrorCmd = &cobra.Command{
	Use:         "mirror",
	Short:       "Mirror the configured source directory onto its destination",
	Long:        "Mirror the configured source directory onto its destination, deleting whatever the destination holds that the source does not.\n\nEvery run starts with the same preflight --dry-run prints, and a real mirror then asks for an explicit confirmation that defaults to No. Neither drive is ever ejected.",
	Annotations: withProfileResolution(profileNotRequired),
	Run: func(cmd *cobra.Command, args []string) {
		cobra.CheckErr(runMirror(cmd))
	},
}

func runMirror(cmd *cobra.Command) error {
	if err := rejectProfileFlags(cmd); err != nil {
		return err
	}

	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}

	settings, err := config.LoadMirror()
	if err != nil {
		return err
	}
	cfg := mirror.Config{
		Source:      settings.Source,
		Destination: settings.Destination,
		RsyncBinary: "rsync",
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	out := cmd.OutOrStdout()
	if dryRun {
		plan, err := mirror.DryRun(ctx, cfg, mirror.OSDeps())
		if err != nil {
			return err
		}
		plan.Render(out)
		return nil
	}

	// The approval is asked here whatever confirmExec says: that setting
	// governs repeatable snapshot backups, while a mirror deletes whatever
	// the destination holds that the source does not.
	execDeps := mirror.OSExecDeps(mirror.ApproverFunc(approveMirror), out, cmd.ErrOrStderr())

	result, err := mirror.Mirror(ctx, cfg, mirror.OSDeps(), execDeps, out)
	recordMirror(result)
	if result.Status != mirror.StatusSkipped {
		fmt.Fprintln(out, result.Summary())
	}
	return err
}

// recordMirror appends an attempted mirror to the backup history. Only a real
// rsync execution is an attempt, so a failed preflight, a decline, a dry run,
// an already-synchronized destination, and an interruption before rsync
// started are all left unrecorded. A history write never changes the outcome
// of the mirror: db.RecordBackup only warns when it fails.
func recordMirror(result mirror.Result) {
	if !result.Attempted() {
		return
	}

	db.RecordBackup(db.HistoryEntry{
		CreatedAt:     result.StartedAt,
		BackupType:    models.Mirror{}.String(),
		ExecutionTime: result.Duration.String(),
		Command:       result.CommandString(),
		Profile:       db.MirrorProfile,
		ExitCode:      result.ExitCode,
	})
}

// approveMirror asks the question a real mirror needs answered, with No first
// so nothing destructive happens by pressing enter.
func approveMirror(plan mirror.Plan) bool {
	return inputs.AskNoYesQuestion(mirrorApprovalQuestion(plan))
}

func mirrorApprovalQuestion(plan mirror.Plan) string {
	if plan.Changes.Deleted == 0 {
		return fmt.Sprintf("Mirror %s onto %s now?", plan.Endpoints.Source, plan.Endpoints.Destination)
	}
	return fmt.Sprintf("Mirror %s onto %s now, deleting the %d entries listed above from it?",
		plan.Endpoints.Source, plan.Endpoints.Destination, plan.Changes.Deleted)
}

// rejectProfileFlags fails when a profile-selection flag was passed. The flags
// are declared on the root command, so the mirror cannot simply not declare
// them.
func rejectProfileFlags(cmd *cobra.Command) error {
	for _, name := range profileFlags {
		if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
			return fmt.Errorf("--%s does not apply to %s: the mirror is a single global operation configured by the top-level %q block", name, cmd.CommandPath(), config.MirrorKey)
		}
	}
	return nil
}

func init() {
	mirrorCmd.Flags().Bool("dry-run", false, "Report what the mirror would change without writing anything")
	RootCmd.AddCommand(mirrorCmd)
}
