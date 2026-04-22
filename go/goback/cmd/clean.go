package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/cleanbackup"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/cleandb"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/cleanlogs"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean unwanted data",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		cobra.CheckErr(err)
	},
}

// cleanDbCmd represents the command to clean the database
var cleanDbCmd = &cobra.Command{
	Use:     "db",
	Short:   "Remove a database entry by ID",
	Example: "goback clean db 1",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cleandb.Remove(args[0])
	},
}

// cleanLogsCmd represents the command to clean logs
var cleanLogsCmd = &cobra.Command{
	Use:   "logs",
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
		cleanlogs.KeepLatestOf(d, "daily")
		cleanlogs.KeepLatestOf(w, "weekly")
		cleanlogs.KeepLatestOf(m, "monthly")
	},
}

var cleanBackupCmd = &cobra.Command{
	Use:       "backup [daily|weekly|monthly]",
	Short:     "Remove excluded files from backup destinations",
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"daily", "weekly", "monthly"},
	Run: func(cmd *cobra.Command, args []string) {
		forEachProfile(func() error {
			if len(args) == 0 {
				return cleanbackup.CleanAll()
			}
			bt, err := parseBackupType(args[0])
			if err != nil {
				return err
			}
			return cleanbackup.CleanType(bt)
		})
	},
}

func parseBackupType(s string) (models.BackupTypes, error) {
	switch s {
	case "daily":
		return models.Daily{}, nil
	case "weekly":
		return models.Weekly{}, nil
	case "monthly":
		return models.Monthly{}, nil
	default:
		return nil, fmt.Errorf("invalid backup type: %s (must be daily, weekly, or monthly)", s)
	}
}

func init() {
	cleanCmd.AddCommand(cleanDbCmd)
	cleanCmd.AddCommand(cleanLogsCmd)
	cleanCmd.AddCommand(cleanBackupCmd)
	RootCmd.AddCommand(cleanCmd)

	cleanLogsCmd.Flags().IntP("keep-daily", "d", 14, "Number of daily logs to keep")
	cleanLogsCmd.Flags().IntP("keep-weekly", "w", 12, "Number of weekly logs to keep")
	cleanLogsCmd.Flags().IntP("keep-monthly", "m", 6, "Number of monthly logs to keep")
}
