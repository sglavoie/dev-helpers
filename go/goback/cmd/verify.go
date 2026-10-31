package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify daily",
	Short: "Compare daily backup contents with the current live source using checksums",
	Long: `Compare an existing daily backup with the current live source using SHA-256
checksums, the daily include/exclude filters, and the source's trailing-slash
layout. Recursively check file contents, directory presence, entry types, and
symlink targets. Permission/timestamp changes and destination-only retained
entries are ignored. Configured pruneEmptyDirs still applies.

Use --profile when more than one profile would be selected. --all is not supported.
The live source and backup must be available. A difference may be a legitimate
source edit since the last backup; verification cannot identify which copy is
correct. This is a live comparison, not a point-in-time filesystem snapshot.

No backup files, configuration, history, or diagnostic logs are changed. Only
coordination lock files are created, outside the backup, to exclude other goback
operations. No prompts, companions, or ejection run. Other programs can still
modify files during the comparison. Verification exits nonzero for differences,
unreadable or changing files, an incomplete scan, or interruption (including
rsync exit 24).
--json keeps the report on stdout even when verification fails; errors go to stderr.`,
	Args:         cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	ValidArgs:    []string{"daily"},
	SilenceUsage: true,
	RunE:         runVerify,
}

func init() {
	verifyCmd.Flags().Bool("json", false, "Output the verification report as JSON, including partial results on scan failures")
	RootCmd.AddCommand(verifyCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("all") {
		return fmt.Errorf("--all does not apply to verify; select one profile with --profile")
	}
	names, err := config.SelectProfiles()
	if err != nil {
		return err
	}
	if len(names) != 1 {
		return fmt.Errorf("verify requires one profile; use --profile to select one of %v", names)
	}
	previous := config.ActiveProfileName
	config.ActiveProfileName = names[0]
	defer func() { config.ActiveProfileName = previous }()
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report, scanErr := buildcmd.VerifyDaily(ctx)
	out := cmd.OutOrStdout()
	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(out, "Profile: %s\nSource: %q\nDaily backup: %q\n", report.Profile, report.Source, report.Destination)
		for _, difference := range report.Differences {
			fmt.Fprintf(out, "%s: %q\n", difference.Status, difference.Path)
		}
		if report.Completed {
			if len(report.Differences) == 0 {
				fmt.Fprintln(out, "Verification complete: selected backup contents match the current source.")
			} else {
				fmt.Fprintf(out, "Verification complete: %d difference(s).\n", len(report.Differences))
			}
		} else {
			fmt.Fprintln(out, "Verification incomplete; a content match could not be established.")
		}
	}
	if scanErr != nil {
		return scanErr
	}
	if len(report.Differences) != 0 {
		return fmt.Errorf("backup verification found %d difference(s)", len(report.Differences))
	}
	return nil
}
