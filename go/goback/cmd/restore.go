package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/restore"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var restoreCmd = &cobra.Command{
	Use:   "restore daily|weekly|monthly FILE --to DIRECTORY",
	Short: "Recover one file into a separate directory and verify its contents",
	Long: `Recover one regular file from daily, weekly, or monthly into an existing
recovery directory. FILE is relative to that backup directory; the recovered
file keeps its filename, without its parent directories. Use --profile when
more than one profile would be selected. --all is not supported.

--dry-run prints the exact source and destination without writing anything.
A real restore preserves ordinary Unix permissions and modification time,
checks the recovered content against the backup using SHA-256, and publishes
the file only after verification. Existing entries are never overwritten,
including symlinks. Symlinks inside the backup and directory restores are not
supported. Ownership, ACLs, and extended attributes are not restored. The
recovery directory must not overlap the profile's backup destination and must
be on a filesystem supporting hard links.

Restore does not need the original source, prompt, run companions, record backup
history, or eject drives. Verification establishes a match with the stored copy;
it cannot establish whether that copy was already corrupted.`,
	Args: cobra.ExactArgs(2),
	RunE: runRestore,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var kinds []string
		for _, kind := range []string{"daily", "weekly", "monthly"} {
			if strings.HasPrefix(kind, toComplete) {
				kinds = append(kinds, kind)
			}
		}
		return kinds, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	restoreCmd.Flags().String("to", "", "Existing recovery directory (required; the file keeps its filename)")
	restoreCmd.Flags().Bool("dry-run", false, "Preview the exact recovery paths without writing anything")
	if err := restoreCmd.MarkFlagRequired("to"); err != nil {
		panic(err)
	}
	if err := restoreCmd.MarkFlagDirname("to"); err != nil {
		panic(err)
	}
	RootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("all") {
		return fmt.Errorf("--all does not apply to restore; select one profile with --profile")
	}
	names, err := config.SelectProfiles()
	if err != nil {
		return err
	}
	if len(names) != 1 {
		return fmt.Errorf("restore requires one profile; use --profile to select one of %v", names)
	}
	to, _ := cmd.Flags().GetString("to")
	plan, err := restore.Prepare(restore.Request{
		BackupRoot:  viper.GetString("profiles." + names[0] + ".destination"),
		BackupType:  args[0],
		Path:        args[1],
		Destination: to,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Profile: %s\nSource: %q\nDestination: %q\nSize: %d bytes\nPermissions: %04o\n",
		names[0], plan.Source, plan.Destination, plan.Size, plan.Mode)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		fmt.Fprintln(out, "Dry run: no files written; content verification runs during a real restore.")
		return nil
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := restore.Restore(ctx, plan); err != nil {
		return err
	}
	fmt.Fprintf(out, "Restored %q (SHA-256 content verified).\n", plan.Destination)
	return nil
}
