package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var profilesCmd = &cobra.Command{
	Use: "profiles", Args: cobra.NoArgs,
	Short:       "List profiles and show which an unqualified run would select",
	Annotations: withProfileResolution(profileNotRequired),
	RunE:        func(cmd *cobra.Command, args []string) error { return printProfiles(cmd.OutOrStdout()) },
}

var statusCmd = &cobra.Command{
	Use: "status", Args: cobra.NoArgs,
	Short:       "Show configured backups, including those never recorded in history",
	Long:        "Show configured snapshot backups, daily companions, and the global mirror, including backups with no retained history. By default all profiles are shown; --profile narrows the report. Select one of --daily, --weekly, --monthly, --mirror, or --companions to filter backup types. --older-than marks last successes older than a duration such as 48h or 168h. --check exits nonzero for unhealthy or empty results; without --older-than it assesses outcomes only, not age. This command does not change configuration or history and does not require mounted drives.",
	Annotations: withProfileResolution(profileNotRequired),
	RunE: func(cmd *cobra.Command, args []string) error {
		older, _ := cmd.Flags().GetDuration("older-than")
		if older < 0 || (cmd.Flags().Changed("older-than") && older == 0) {
			return fmt.Errorf("--older-than must be a positive duration, such as 48h")
		}
		kind := ""
		for _, name := range statusSelectors {
			selected, _ := cmd.Flags().GetBool(name)
			if selected {
				kind = name
			}
		}
		names, err := overviewProfiles()
		if err != nil {
			return err
		}
		history, err := db.ReadSummary()
		if err != nil {
			return fmt.Errorf("read backup history: %w", err)
		}
		rows, err := configuredStatus(names, history, time.Now(), older)
		if err != nil {
			return err
		}
		rows = filterStatus(rows, kind)
		asJSON, _ := cmd.Flags().GetBool("json")
		if asJSON {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(rows); err != nil {
				return err
			}
		} else {
			printStatus(cmd.OutOrStdout(), rows)
		}
		check, _ := cmd.Flags().GetBool("check")
		if check {
			// Keep stdout valid JSON even when a health check fails.
			cmd.SilenceUsage = true
			return checkStatus(rows)
		}
		return nil
	},
}

var statusSelectors = []string{"daily", "weekly", "monthly", "mirror", "companions"}

func init() {
	statusCmd.Flags().Duration("older-than", 0, "Mark last successes older than this duration (e.g. 48h, 168h)")
	statusCmd.Flags().Bool("json", false, "Output configured backup status as JSON")
	statusCmd.Flags().Bool("check", false, "Exit nonzero for failed, interrupted, stale, never-successful, or empty results")
	for _, name := range statusSelectors {
		statusCmd.Flags().Bool(name, false, "Show only "+name+" backups")
	}
	statusCmd.MarkFlagsMutuallyExclusive(statusSelectors...)
	RootCmd.AddCommand(profilesCmd, statusCmd)
}

func overviewProfiles() ([]string, error) {
	if config.ProfileFlag != "" {
		return config.SelectProfiles()
	}
	return config.ProfileNames(), nil
}

func configuredKinds(name string) []string {
	var kinds []string
	for _, kind := range []string{"daily", "weekly", "monthly"} {
		if viper.IsSet("profiles." + name + ".rsync." + kind + ".archive") {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

func printProfiles(w io.Writer) error {
	names, err := overviewProfiles()
	if err != nil {
		return err
	}
	automatic, selectionErr := config.DefaultProfiles()
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Profile", "Auto-selected", "Hostname", "Host match", "Source", "Destination", "Backups"})
	for _, name := range names {
		prefix := "profiles." + name + "."
		host := viper.GetString(prefix + "hostname")
		match, selected := "-", "no"
		if host != "" {
			match = "no"
			if host == hostname {
				match = "yes"
			}
		}
		if slices.Contains(automatic, name) {
			selected = "yes"
		}
		t.AppendRow(table.Row{name, selected, host, match, viper.GetString(prefix + "source"), viper.GetString(prefix + "destination"), strings.Join(configuredKinds(name), ", ")})
	}
	if len(names) == 0 {
		fmt.Fprintln(w, "No backup profiles configured.")
		return nil
	}
	fmt.Fprintln(w, t.Render())
	if selectionErr != nil {
		fmt.Fprintf(w, "Automatic selection unavailable: %v\n", selectionErr)
	}
	return nil
}

type statusRow struct {
	Profile       string  `json:"profile"`
	BackupType    string  `json:"backup_type"`
	LastSuccess   *string `json:"last_success"`
	LatestAttempt *string `json:"latest_attempt"`
	ExitCode      *int    `json:"exit_code"`
	Result        string  `json:"result"`
	Freshness     string  `json:"freshness"`
}

func configuredStatus(names []string, history []db.SummaryRow, now time.Time, older time.Duration) ([]statusRow, error) {
	type key struct{ profile, kind string }
	indexed := make(map[key]db.SummaryRow)
	for _, row := range history {
		indexed[key{row.Profile, row.BackupType}] = row
	}
	rows := []statusRow{}
	appendStatus := func(profile, kind string) error {
		row := statusRow{Profile: profile, BackupType: kind, Result: "never run", Freshness: "no recorded success"}
		if entry, ok := indexed[key{profile, kind}]; ok {
			row.LatestAttempt, row.ExitCode, row.LastSuccess = &entry.LatestAttempt, &entry.ExitCode, entry.LastSuccess
			row.Result = "succeeded"
			if entry.ExitCode == -1 {
				row.Result = "interrupted"
			} else if entry.ExitCode != 0 {
				row.Result = fmt.Sprintf("failed (exit %d)", entry.ExitCode)
			}
			if entry.LastSuccess != nil {
				row.Freshness = "not assessed"
				if older > 0 {
					success, err := time.ParseInLocation("2006-01-02 15:04:05", *entry.LastSuccess, time.Local)
					if err != nil {
						return fmt.Errorf("invalid success timestamp for %s/%s: %w", profile, kind, err)
					}
					row.Freshness = "current"
					if now.Sub(success) > older {
						row.Freshness = "stale"
					}
				}
			}
		}
		rows = append(rows, row)
		return nil
	}
	for _, name := range names {
		for _, kind := range configuredKinds(name) {
			if err := appendStatus(name, kind); err != nil {
				return nil, err
			}
		}
		companions, err := config.ProfileCompanions(name)
		if err != nil {
			return nil, err
		}
		slices.SortFunc(companions, func(a, b config.Companion) int { return strings.Compare(a.ID, b.ID) })
		for _, companion := range companions {
			if err := appendStatus(name, db.CompanionBackupType(companion.ID)); err != nil {
				return nil, err
			}
		}
	}
	if config.ProfileFlag == "" && viper.IsSet("mirror") {
		if err := appendStatus(db.MirrorProfile, "mirror"); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func filterStatus(rows []statusRow, kind string) []statusRow {
	if kind == "" {
		return rows
	}
	filtered := []statusRow{}
	for _, row := range rows {
		if row.BackupType == kind || (kind == "companions" && strings.HasPrefix(row.BackupType, "companion/")) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func checkStatus(rows []statusRow) error {
	if len(rows) == 0 {
		return fmt.Errorf("no configured backups match the requested status check")
	}
	var unhealthy []string
	for _, row := range rows {
		if row.LastSuccess == nil || row.ExitCode == nil || *row.ExitCode != 0 || row.Freshness == "stale" {
			unhealthy = append(unhealthy, row.Profile+"/"+row.BackupType)
		}
	}
	if len(unhealthy) > 0 {
		return fmt.Errorf("backup status check failed: %s", strings.Join(unhealthy, ", "))
	}
	return nil
}

func printStatus(w io.Writer, rows []statusRow) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No configured backups.")
		return
	}
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Profile", "Backup type", "Last success", "Latest attempt", "Result", "Freshness"})
	for _, row := range rows {
		success, attempt := "Never recorded", "Never recorded"
		if row.LastSuccess != nil {
			success = printer.TimestampWithAge(*row.LastSuccess)
		}
		if row.LatestAttempt != nil {
			attempt = printer.TimestampWithAge(*row.LatestAttempt)
		}
		t.AppendRow(table.Row{row.Profile, row.BackupType, success, attempt, row.Result, row.Freshness})
	}
	fmt.Fprintln(w, t.Render())
}
