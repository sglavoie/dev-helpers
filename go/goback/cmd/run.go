package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/run"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the backup command",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		cobra.CheckErr(err)
	},
}

func init() {
	runCmd.PersistentFlags().Bool("dry-run", false, "Show what would be transferred without actually running rsync")
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

	forEachProfile(func() error {
		report := &run.Report{}
		defer report.Print(os.Stdout)
		return action(ctx, report)
	})
}

var dailyCmdRun = &cobra.Command{
	Use:   "daily",
	Short: "Perform a daily backup",
	Long:  "Perform a daily, incremental backup.",
	Run: func(cmd *cobra.Command, args []string) {
		runBackups(run.DailyBackup)
	},
}

var weeklyCmdRun = &cobra.Command{
	Use:   "weekly",
	Short: "Perform a weekly backup",
	Long:  "Perform a weekly, incremental backup from the last daily backup.",
	Run: func(cmd *cobra.Command, args []string) {
		runBackups(run.WeeklyBackup)
	},
}

var monthlyCmdRun = &cobra.Command{
	Use:   "monthly",
	Short: "Perform a monthly backup",
	Long:  "Perform a monthly, incremental, compressed backup from the last daily backup.",
	Run: func(cmd *cobra.Command, args []string) {
		runBackups(run.MonthlyBackup)
	},
}

var allCmdRun = &cobra.Command{
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
