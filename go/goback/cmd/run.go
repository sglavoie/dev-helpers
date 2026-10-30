package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/run"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "run",
	Short: "Run the backup command",
	Long: `Run configured snapshot backups and daily companions.
--non-interactive explicitly approves the configured snapshot operations even
when confirmExec is true. It never prompts to create or repair configuration,
suppresses progress output, and exits nonzero if a backup or companion fails.
Companion programs must themselves support unattended execution.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		cobra.CheckErr(err)
	},
}

func init() {
	runCmd.PersistentFlags().Bool("non-interactive", false, "Approve configured backups without prompts; fail if a backup or companion fails")
	if err := viper.BindPFlag("cliNonInteractive", runCmd.PersistentFlags().Lookup("non-interactive")); err != nil {
		panic(err)
	}
	runCmd.PersistentFlags().Bool("dry-run", false, "Ask rsync what would be transferred without changing backups or history")
	if err := viper.BindPFlag("cliDryRun", runCmd.PersistentFlags().Lookup("dry-run")); err != nil {
		panic(err)
	}
	runCmd.AddCommand(dailyCmdRun)
	runCmd.AddCommand(weeklyCmdRun)
	runCmd.AddCommand(monthlyCmdRun)
	runCmd.AddCommand(allCmdRun)
	RootCmd.AddCommand(runCmd)
}

// runBackups runs one backup action per profile under a single signal context,
// so one interruption cancels rsync and its companions together, and prints the
// combined results of each profile once its companions have finished.
func runBackups(action func(context.Context, *run.Report) error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if viper.GetBool("cliNonInteractive") {
		fmt.Fprintf(os.Stdout, "goback: started %s\n", time.Now().Format(time.RFC3339))
	}
	err := runBackupProfiles(ctx, action, func(paths []string) error {
		return eject.EjectPaths(os.Stdout, paths, eject.OSDeps())
	})
	if viper.GetBool("cliNonInteractive") {
		outcome := "succeeded"
		if err != nil {
			outcome = "failed"
		}
		fmt.Fprintf(os.Stdout, "goback: finished %s (%s)\n", time.Now().Format(time.RFC3339), outcome)
	}
	cobra.CheckErr(err)
}

func runBackupProfiles(ctx context.Context, action func(context.Context, *run.Report) error, ejectPaths func([]string) error) error {
	var destinations []string
	err := forEachProfile(func() error {
		report := &run.Report{}
		defer report.Print(os.Stdout)
		err := action(ctx, report)
		if viper.GetBool("cliNonInteractive") {
			err = errors.Join(err, report.CompanionError(), ctx.Err())
		}
		if err != nil {
			return err
		}
		if viper.GetBool("ejectOnExit") && report.CompletedBackup() {
			if dest := viper.GetString(config.ActiveProfilePrefix() + "destination"); dest != "" {
				destinations = append(destinations, dest)
			}
		}
		return nil
	})
	if len(destinations) > 0 && ctx.Err() == nil {
		err = errors.Join(err, ejectPaths(destinations))
	}
	return err
}

var dailyCmdRun = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "daily",
	Short: "Perform a daily backup",
	Long:  "Perform a daily, incremental backup.",
	Run: func(cmd *cobra.Command, args []string) {
		runBackups(run.DailyBackup)
	},
}

var weeklyCmdRun = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "weekly",
	Short: "Perform a weekly backup",
	Long:  "Perform a weekly, incremental backup from the last daily backup.",
	Run: func(cmd *cobra.Command, args []string) {
		runBackups(run.WeeklyBackup)
	},
}

var monthlyCmdRun = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "monthly",
	Short: "Perform a monthly backup",
	Long:  "Perform a monthly, incremental backup from the last daily backup.",
	Run: func(cmd *cobra.Command, args []string) {
		runBackups(run.MonthlyBackup)
	},
}

var allCmdRun = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "all",
	Short: "Run daily, weekly, and monthly backups in sequence",
	Run: func(cmd *cobra.Command, args []string) {
		runBackups(run.AllBackups)
	},
}
