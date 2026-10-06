package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/logic"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <UUID | short ID>",
		Short: "Update an entry in place without interactive prompts",
		Long: `Update any combination of fields in one save, preserving the entry's UUID.
Use permanent UUIDs in scripts: short IDs change when entries are added or moved.
Timestamps must include a time zone (RFC3339, e.g. 2026-10-06T09:00:00-06:00).
Setting --end stops the timer; --end active leaves it running or resumes it.
Omitted fields keep their values. Setting --duration adjusts the end of a completed
entry or the start of a running timer. It cannot be combined with --start or --end.
Stashed entries must be restored before editing. Use 'gt undo' to restore an edit.`,
		Args: cobra.ExactArgs(1),
		RunE: runUpdate,
	}
	cmd.Flags().String("keyword", "", "Replacement keyword")
	cmd.Flags().String("tags", "", "Comma-separated tags; an empty string clears them")
	cmd.Flags().String("start", "", "Start timestamp (RFC3339)")
	cmd.Flags().String("end", "", "End timestamp (RFC3339), or active for a running timer")
	cmd.Flags().Int("duration", 0, "Duration in seconds")
	return cmd
}

func init() { rootCmd.AddCommand(newUpdateCmd()) }

func runUpdate(cmd *cobra.Command, args []string) error {
	hasChange := false
	for _, field := range []string{"keyword", "tags", "start", "end", "duration"} {
		hasChange = hasChange || cmd.Flags().Changed(field)
	}
	if !hasChange {
		return fmt.Errorf("provide at least one of --keyword, --tags, --start, --end, or --duration")
	}
	if cmd.Flags().Changed("duration") && (cmd.Flags().Changed("start") || cmd.Flags().Changed("end")) {
		return fmt.Errorf("--duration cannot be combined with --start or --end")
	}
	manager := config.NewManager(GetConfigPath())
	cfg, err := manager.Load()
	if err != nil {
		return err
	}
	parsed, err := ParseKeywordOrID(args[0], cfg)
	if err != nil {
		return err
	}
	if parsed.Type != ArgumentTypeID {
		return fmt.Errorf("update requires a UUID or short ID, not a keyword")
	}
	original := *parsed.Entry
	if original.Stashed {
		return fmt.Errorf("restore the stashed entry before editing it")
	}
	updated := original
	now := time.Now()
	if cmd.Flags().Changed("keyword") {
		updated.Keyword, _ = cmd.Flags().GetString("keyword")
		updated.Keyword = strings.TrimSpace(updated.Keyword)
		if updated.Keyword == "" || logic.IsReservedKeyword(updated.Keyword) {
			return fmt.Errorf("keyword must be nonempty and cannot be a number")
		}
	}
	if cmd.Flags().Changed("tags") {
		value, _ := cmd.Flags().GetString("tags")
		if err := setFieldValue(&updated, "tags", value); err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("start") {
		value, _ := cmd.Flags().GetString("start")
		updated.StartTime, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return fmt.Errorf("--start requires an RFC3339 timestamp with a time zone")
		}
	}
	if cmd.Flags().Changed("end") {
		value, _ := cmd.Flags().GetString("end")
		if value == "active" {
			updated.Active, updated.EndTime = true, nil
		} else {
			end, parseErr := time.Parse(time.RFC3339Nano, value)
			if parseErr != nil {
				return fmt.Errorf("--end requires an RFC3339 timestamp with a time zone, or active")
			}
			updated.Active, updated.EndTime = false, &end
		}
	}
	if cmd.Flags().Changed("duration") {
		seconds, _ := cmd.Flags().GetInt("duration")
		if seconds < 0 || int64(seconds) > int64((1<<63-1)/time.Second) {
			return fmt.Errorf("duration must be nonnegative and fit in a time interval")
		}
		duration := time.Duration(seconds) * time.Second
		if updated.Active {
			updated.StartTime = now.Add(-duration)
		} else {
			end := updated.StartTime.Add(duration)
			updated.EndTime = &end
		}
	}
	if updated.StartTime.After(now) {
		return fmt.Errorf("start time cannot be in the future")
	}
	if updated.Active {
		updated.EndTime, updated.Duration = nil, 0
		for _, other := range cfg.Entries {
			if other.ID != updated.ID && other.Active && other.Keyword == updated.Keyword {
				return fmt.Errorf("an active timer for keyword %q already exists", updated.Keyword)
			}
		}
	} else if updated.EndTime != nil {
		if updated.EndTime.Before(updated.StartTime) {
			return fmt.Errorf("end time cannot be before start time")
		}
		if updated.EndTime.After(now) {
			return fmt.Errorf("end time cannot be in the future")
		}
		if cmd.Flags().Changed("start") || cmd.Flags().Changed("end") || cmd.Flags().Changed("duration") {
			updated.Duration = int(updated.EndTime.Sub(updated.StartTime).Seconds())
		}
	} else if cmd.Flags().Changed("start") {
		return fmt.Errorf("this completed entry has no end time; provide --end too")
	}
	*parsed.Entry = updated
	cfg.UpdateShortIDs()
	cfg.AddUndoRecord(models.UndoOperationBulkEdit, "Updated entry: "+updated.Keyword,
		map[string]interface{}{"original_entries": []models.Entry{original}})
	if err := manager.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Updated entry: %s (%s)\n", updated.Keyword, updated.ID)
	return nil
}
