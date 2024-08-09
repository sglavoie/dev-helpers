package cmd

import (
	"github.com/spf13/cobra"
	"goback/pkg/run"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the rsync command",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			return
		}
	},
}

func init() {
	runCmd.AddCommand(dailyCmd)
	runCmd.AddCommand(weeklyCmd)
	runCmd.AddCommand(monthlyCmd)
	RootCmd.AddCommand(runCmd)
}

var dailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Perform a daily backup",
	Long:  "Perform a daily, incremental backup.",
	Run: func(cmd *cobra.Command, args []string) {
		run.ExecDailyBackup()
	},
}

var weeklyCmd = &cobra.Command{
	Use:   "weekly",
	Short: "Perform a weekly backup",
	Long:  "Perform a weekly, incremental backup from the last daily backup.",
	Run: func(cmd *cobra.Command, args []string) {
		run.ExecWeeklyBackup()
	},
}

var monthlyCmd = &cobra.Command{
	Use:   "monthly",
	Short: "Perform a monthly backup",
	Long:  "Perform a monthly, incremental, compressed backup from the weekly backup.",
	Run: func(cmd *cobra.Command, args []string) {
		run.ExecMonthlyBackup()
	},
}
