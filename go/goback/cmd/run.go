package cmd

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/run"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/usage/last"
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

var dailyCmdRun = &cobra.Command{
	Use:   "daily",
	Short: "Perform a daily backup",
	Long:  "Perform a daily, incremental backup.",
	Run: func(cmd *cobra.Command, args []string) {
		forEachProfile(func() error {
			if err := run.DailyBackup(); err != nil {
				return err
			}
			last.SummaryWithLineBreak()
			return nil
		})
	},
}

var weeklyCmdRun = &cobra.Command{
	Use:   "weekly",
	Short: "Perform a weekly backup",
	Long:  "Perform a weekly, incremental backup from the last daily backup.",
	Run: func(cmd *cobra.Command, args []string) {
		forEachProfile(func() error {
			if err := run.WeeklyBackup(); err != nil {
				return err
			}
			last.SummaryWithLineBreak()
			return nil
		})
	},
}

var monthlyCmdRun = &cobra.Command{
	Use:   "monthly",
	Short: "Perform a monthly backup",
	Long:  "Perform a monthly, incremental, compressed backup from the last daily backup.",
	Run: func(cmd *cobra.Command, args []string) {
		forEachProfile(func() error {
			if err := run.MonthlyBackup(); err != nil {
				return err
			}
			last.SummaryWithLineBreak()
			return nil
		})
	},
}

var allCmdRun = &cobra.Command{
	Use:   "all",
	Short: "Run daily, weekly, and monthly backups in sequence",
	Run: func(cmd *cobra.Command, args []string) {
		forEachProfile(func() error {
			if err := run.DailyBackup(); err != nil {
				return err
			}
			if buildcmd.IsConfigured("weekly") {
				if err := run.WeeklyBackup(); err != nil {
					return err
				}
			}
			if buildcmd.IsConfigured("monthly") {
				if err := run.MonthlyBackup(); err != nil {
					return err
				}
			}
			last.SummaryWithLineBreak()
			return nil
		})
	},
}
