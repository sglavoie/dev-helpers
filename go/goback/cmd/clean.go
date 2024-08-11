package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/clean"
	"github.com/spf13/cobra"
)

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove logs",
	Run: func(cmd *cobra.Command, args []string) {
		keep, err := cmd.Flags().GetInt("keep")
		if err != nil {
			fmt.Println("Error getting keep flag:", err)
			return
		}
		clean.KeepLatest(keep)
	},
}

func init() {
	RootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().IntP("keep", "k", 10, "Number of logs to keep")
}
