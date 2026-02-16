package cmd

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/spf13/cobra"
)

// previewCmd simply prints the rsync command that would be executed
var previewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Print the rsync command that would be executed",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		cobra.CheckErr(err)
	},
}

func init() {
	previewCmd.AddCommand(dailyCmdPreview)
	previewCmd.AddCommand(weeklyCmdPreview)
	previewCmd.AddCommand(monthlyCmdPreview)
	RootCmd.AddCommand(previewCmd)
}

var dailyCmdPreview = &cobra.Command{
	Use:   "daily",
	Short: "Preview command for daily backup",
	Run: func(cmd *cobra.Command, args []string) {
		forEachProfile(func() {
			buildcmd.PrintCommandDaily()
		})
	},
}

var weeklyCmdPreview = &cobra.Command{
	Use:   "weekly",
	Short: "Preview command for weekly backup",
	Run: func(cmd *cobra.Command, args []string) {
		forEachProfile(func() {
			buildcmd.PrintCommandWeekly()
		})
	},
}

var monthlyCmdPreview = &cobra.Command{
	Use:   "monthly",
	Short: "Preview command for monthly backup",
	Run: func(cmd *cobra.Command, args []string) {
		forEachProfile(func() {
			buildcmd.PrintCommandMonthly()
		})
	},
}
