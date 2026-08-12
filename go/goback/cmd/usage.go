package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/usage/last"
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

var lastUsageCmd = &cobra.Command{
	Use:   "last",
	Short: "Show last goback's usage for each backup type",
	Run: func(cmd *cobra.Command, args []string) {
		e, err := cmd.Flags().GetInt("entries")
		cobra.CheckErr(err)
		if e < 1 {
			cobra.CheckErr("Latest entries to show must be greater than 0")
		}

		s, err := cmd.Flags().GetBool("summary")
		cobra.CheckErr(err)
		if s {
			last.Summary()
			return
		}
		last.Last(e)
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
		view.View(e, parseBuilderTypeFlags(cmd))
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
				cobra.CheckErr("Number of entries to keep must be greater than or equal to 0")
			}
			toKeep = k
		}

		reset.Reset(toKeep, parseBuilderTypeFlags(cmd))
	},
}

func init() {
	usageCmd.AddCommand(lastUsageCmd)
	usageCmd.AddCommand(viewUsageCmd)
	usageCmd.AddCommand(resetUsageCmd)
	RootCmd.AddCommand(usageCmd)

	lastUsageCmd.Flags().IntP("entries", "e", 3, "Number of entries to show for each backup type")
	lastUsageCmd.Flags().BoolP("summary", "s", false, "Show when the last backup by type was done")

	resetUsageCmd.Flags().BoolP("all", "a", false, "Reset all usage (set --keep=0)")
	resetUsageCmd.Flags().IntP("keep", "k", 20, "Number of entries to keep")
	resetUsageCmd.Flags().BoolP("daily", "d", false, "Remove by daily usage")
	resetUsageCmd.Flags().BoolP("weekly", "w", false, "Remove by weekly usage")
	resetUsageCmd.Flags().BoolP("monthly", "m", false, "Remove by monthly usage")
	resetUsageCmd.Flags().Bool("mirror", false, "Remove by mirror usage")

	viewUsageCmd.Flags().IntP("entries", "e", 20, "Number of entries to display")
	viewUsageCmd.Flags().BoolP("daily", "d", false, "Display by daily usage")
	viewUsageCmd.Flags().BoolP("weekly", "w", false, "Display by weekly usage")
	viewUsageCmd.Flags().BoolP("monthly", "m", false, "Display by monthly usage")
	viewUsageCmd.Flags().Bool("mirror", false, "Display by mirror usage")
}

func parseBuilderTypeFlags(cmd *cobra.Command) models.BackupTypes {
	selected := make(map[string]bool, len(backupTypeSelectors))
	for _, selector := range backupTypeSelectors {
		set, err := cmd.Flags().GetBool(selector.flag)
		cobra.CheckErr(err)
		selected[selector.flag] = set
	}

	backupType, err := selectBackupType(selected)
	cobra.CheckErr(err)
	return backupType
}

// backupTypeSelectors are the mutually exclusive flags that narrow a usage
// command to one backup type, in the order they are reported.
var backupTypeSelectors = []struct {
	flag       string
	backupType models.BackupTypes
}{
	{"daily", models.Daily{}},
	{"weekly", models.Weekly{}},
	{"monthly", models.Monthly{}},
	{"mirror", models.Mirror{}},
}

func selectBackupType(selected map[string]bool) (models.BackupTypes, error) {
	var chosen []string
	backupType := models.BackupTypes(models.NoBackupType{})
	for _, selector := range backupTypeSelectors {
		if !selected[selector.flag] {
			continue
		}
		chosen = append(chosen, selector.flag)
		backupType = selector.backupType
	}

	if len(chosen) > 1 {
		return models.NoBackupType{}, fmt.Errorf("only one of daily, weekly, monthly, or mirror can be set, but %s were", strings.Join(chosen, ", "))
	}
	return backupType, nil
}
