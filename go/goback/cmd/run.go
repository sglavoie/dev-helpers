package cmd

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/run"
	"github.com/spf13/cobra"
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
	runCmd.AddCommand(dailyCmdRun)
	runCmd.AddCommand(weeklyCmdRun)
	runCmd.AddCommand(monthlyCmdRun)
	RootCmd.AddCommand(runCmd)
}

var dailyCmdRun = &cobra.Command{
	Use:   "daily",
	Short: "Perform a daily backup",
	Long:  "Perform a daily, incremental backup.",
	Run: func(cmd *cobra.Command, args []string) {
		run.DailyBackup()
	},
}

var weeklyCmdRun = &cobra.Command{
	Use:   "weekly",
	Short: "Perform a weekly backup",
	Long:  "Perform a weekly, incremental backup from the last daily backup.",
	Run: func(cmd *cobra.Command, args []string) {
		run.WeeklyBackup()
	},
}

var monthlyCmdRun = &cobra.Command{
	Use:   "monthly",
	Short: "Perform a monthly backup",
	Long:  "Perform a monthly, incremental, compressed backup from the weekly backup.",
	Run: func(cmd *cobra.Command, args []string) {
		run.MonthlyBackup()
	},
}
