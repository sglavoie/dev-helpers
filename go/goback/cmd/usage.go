package cmd

import (
	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/usage/reset"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/usage/view"
	"github.com/spf13/cobra"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Manage goback's usage",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		cobra.CheckErr(err)
	},
}

var viewUsageCmd = &cobra.Command{
	Use:   "view",
	Short: "View goback's usage",
	Run: func(cmd *cobra.Command, args []string) {
		e, err := cmd.Flags().GetInt("entries")
		cobra.CheckErr(err)
		if e < 1 {
			cobra.CheckErr("Number of entries to view must be greater than 0, right?")
		}
		builderType := parseBuilderTypeFlags(cmd)
		if builderType == "" {
			view.View(e, "")
		} else {
			view.View(e, builderType)
		}
	},
}

var resetUsageCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset goback's usage",
	Run: func(cmd *cobra.Command, args []string) {
		a, err := cmd.Flags().GetBool("all")
		cobra.CheckErr(err)
		var toKeep int
		if a {
			toKeep = 0
		} else {
			k, err := cmd.Flags().GetInt("keep")
			cobra.CheckErr(err)
			if k < 0 {
				cobra.CheckErr("Number of entries to keep must be greater than or equal to 0, right?")
			}
			toKeep = k
		}

		builderType := parseBuilderTypeFlags(cmd)
		if builderType == "" {
			reset.Reset(toKeep, "")
		} else {
			reset.Reset(toKeep, builderType)
		}
	},
}

func init() {
	usageCmd.AddCommand(viewUsageCmd)
	usageCmd.AddCommand(resetUsageCmd)
	RootCmd.AddCommand(usageCmd)

	resetUsageCmd.Flags().BoolP("all", "a", false, "Reset all usage (set --keep=0)")
	resetUsageCmd.Flags().IntP("keep", "k", 20, "Number of entries to keep")
	resetUsageCmd.Flags().BoolP("daily", "d", false, "Remove by daily usage")
	resetUsageCmd.Flags().BoolP("weekly", "w", false, "Remove by weekly usage")
	resetUsageCmd.Flags().BoolP("monthly", "m", false, "Remove by monthly usage")

	viewUsageCmd.Flags().IntP("entries", "e", 20, "Number of entries to display")
	viewUsageCmd.Flags().BoolP("daily", "d", false, "Display by daily usage")
	viewUsageCmd.Flags().BoolP("weekly", "w", false, "Display by weekly usage")
	viewUsageCmd.Flags().BoolP("monthly", "m", false, "Display by monthly usage")
}

func parseBuilderTypeFlags(cmd *cobra.Command) (builderType string) {
	d, err := cmd.Flags().GetBool("daily")
	cobra.CheckErr(err)
	w, err := cmd.Flags().GetBool("weekly")
	cobra.CheckErr(err)
	m, err := cmd.Flags().GetBool("monthly")
	cobra.CheckErr(err)

	flagCount := 0
	if d {
		flagCount++
	}
	if w {
		flagCount++
	}
	if m {
		flagCount++
	}
	if flagCount > 1 {
		cobra.CheckErr("Only one of daily, weekly, or monthly can be set")
	}

	if d {
		return buildcmd.DailyBuilderType()
	}
	if w {
		return buildcmd.WeeklyBuilderType()
	}
	if m {
		return buildcmd.MonthlyBuilderType()
	}
	return ""
}