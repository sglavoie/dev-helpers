package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/cleanbackup"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/cleandb"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/cleanlogs"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/diagnostics"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Args:  cobra.NoArgs,
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
	Args:  cobra.NoArgs,
	Use:   "logs",
	Short: "Trim diagnostic and legacy logs without requiring configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, name := range []string{"profile", "all"} {
			if cmd.Flags().Changed(name) {
				return fmt.Errorf("--%s does not apply to clean logs: log retention covers all profiles", name)
			}
		}
		keep, _ := cmd.Flags().GetInt("keep")
		d, _ := cmd.Flags().GetInt("keep-daily")
		w, _ := cmd.Flags().GetInt("keep-weekly")
		m, _ := cmd.Flags().GetInt("keep-monthly")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		return cleanlogs.Clean(cmd.OutOrStdout(), keep, d, w, m, dryRun)
	},
}

var cleanBackupCmd = &cobra.Command{
	Use:       "backup [daily|weekly|monthly]",
	Short:     "Remove excluded files from backup destinations",
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"daily", "weekly", "monthly"},
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, err := cmd.Flags().GetBool("dry-run")
		cobra.CheckErr(err)
		cobra.CheckErr(forEachProfile(func() error {
			if len(args) == 0 {
				return cleanbackup.CleanAll(dryRun)
			}
			bt, err := parseBackupType(args[0])
			if err != nil {
				return err
			}
			return cleanbackup.CleanType(bt, dryRun)
		}))
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
	cleanBackupCmd.Flags().Bool("dry-run", false, "List excluded entries without deleting or asking for confirmation")
	cleanCmd.AddCommand(cleanDbCmd)
	cleanCmd.AddCommand(cleanLogsCmd)
	cleanCmd.AddCommand(cleanBackupCmd)
	RootCmd.AddCommand(cleanCmd)

	cleanLogsCmd.Flags().Int("keep", diagnostics.KeepLogs, "Number of current failure logs to keep across all profiles and backup types")
	cleanLogsCmd.Flags().Bool("dry-run", false, "List logs that would be removed without deleting them")
	cleanLogsCmd.Flags().IntP("keep-daily", "d", 14, "Number of legacy daily logs in the home directory to keep")
	cleanLogsCmd.Flags().IntP("keep-weekly", "w", 12, "Number of legacy weekly logs in the home directory to keep")
	cleanLogsCmd.Flags().IntP("keep-monthly", "m", 6, "Number of legacy monthly logs in the home directory to keep")
}
