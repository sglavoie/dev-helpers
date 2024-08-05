package cmd

import (
	"github.com/spf13/cobra"
	"goback/pkg/buildcmd"
)

// previewCmd simply prints the rsync command that would be executed
var previewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Print the rsync command that would be executed",
	Run: func(cmd *cobra.Command, args []string) {
		buildcmd.PrintRsyncCommand()
	},
}

func init() {
	RootCmd.AddCommand(previewCmd)
}
