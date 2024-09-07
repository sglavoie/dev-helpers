package cmd

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/clean"
	"github.com/spf13/cobra"
)

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove logs",
	Run: func(cmd *cobra.Command, args []string) {
		d, err := cmd.Flags().GetInt("keep-daily")
		cobra.CheckErr(err)
		if d < 0 {
			cobra.CheckErr("Number of daily logs to keep must be greater than or equal to 0")
		}
		w, err := cmd.Flags().GetInt("keep-weekly")
		cobra.CheckErr(err)
		if w < 0 {
			cobra.CheckErr("Number of weekly logs to keep must be greater than or equal to 0")
		}
		m, err := cmd.Flags().GetInt("keep-monthly")
		cobra.CheckErr(err)
		if m < 0 {
			cobra.CheckErr("Number of monthly logs to keep must be greater than or equal to 0")
		}
		clean.KeepLatestOf(d, "daily")
		clean.KeepLatestOf(w, "weekly")
		clean.KeepLatestOf(m, "monthly")
	},
}

func init() {
	RootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().IntP("keep-daily", "d", 14, "Number of daily logs to keep")
	cleanCmd.Flags().IntP("keep-weekly", "w", 12, "Number of weekly logs to keep")
	cleanCmd.Flags().IntP("keep-monthly", "m", 6, "Number of monthly logs to keep")
}
