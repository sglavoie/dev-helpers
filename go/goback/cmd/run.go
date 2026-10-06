package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
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
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		cobra.CheckErr(err)
	},
}

func init() {
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

	cobra.CheckErr(runBackupProfiles(ctx, action, func(paths []string) error {
		return eject.EjectPaths(os.Stdout, paths, eject.OSDeps())
	}))
}

func runBackupProfiles(ctx context.Context, action func(context.Context, *run.Report) error, ejectPaths func([]string) error) error {
	var destinations []string
	err := forEachProfile(func() error {
		report := &run.Report{}
		defer report.Print(os.Stdout)
		if err := action(ctx, report); err != nil {
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
	Long:  "Perform a monthly, incremental, compressed backup from the last daily backup.",
	Run: func(cmd *cobra.Command, args []string) {
		runBackups(run.MonthlyBackup)
	},
}

var allCmdRun = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "all",
	Short: "Run daily, weekly, and monthly backups in sequence",
	Run: func(cmd *cobra.Command, args []string) {
		runBackups(func(ctx context.Context, report *run.Report) error {
			if err := run.DailyBackup(ctx, report); err != nil {
				return err
			}
			if buildcmd.IsConfigured("weekly") {
				if err := run.WeeklyBackup(ctx, report); err != nil {
					return err
				}
			}
			if buildcmd.IsConfigured("monthly") {
				return run.MonthlyBackup(ctx, report)
			}
			return nil
		})
	},
}
