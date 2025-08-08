package cmd

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new time tracking entry retroactively",
	Long: `Add a new time tracking entry retroactively using an interactive form.
This allows you to create entries for work that was done in the past but wasn't tracked.

The form will allow you to set:
- Keyword: The primary categorization for the entry
- Tags: Additional categorization tags
- Start Time: When the work started
- End Time: When the work ended (leave empty for active entry)
- Duration: Total time spent (calculated automatically from start/end times)

Examples:
  gt add                            # Interactive form to add a new entry`,
	Args: cobra.NoArgs,
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create a blank entry with default values for retroactive entry
	// Default to 1 hour completed entry ending now
	shortID := getNextShortID(cfg)
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	newEntry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   shortID,
		Keyword:   "",
		Tags:      []string{},
		StartTime: oneHourAgo, // Default to 1 hour ago
		EndTime:   &now,       // Default to completed entry
		Duration:  3600,       // 1 hour in seconds
		Active:    false,      // Default to completed for retroactive entries
	}

	// Run field editor TUI
	if err := tui.RunFieldEditor(newEntry); err != nil {
		return fmt.Errorf("entry creation cancelled or failed: %w", err)
	}

	// Validate that required fields are set
	if newEntry.Keyword == "" {
		return fmt.Errorf("keyword is required")
	}

	// Add to configuration
	cfg.AddEntry(newEntry)

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Format output
	var statusStr string
	var durationStr string

	if newEntry.Active {
		statusStr = "active"
		durationStr = formatDuration(newEntry.GetCurrentDuration())
	} else {
		statusStr = "completed"
		durationStr = formatDuration(newEntry.Duration)
	}

	if len(newEntry.Tags) > 0 {
		fmt.Printf("Added %s entry: %s %v - %s\n", statusStr, newEntry.Keyword, newEntry.Tags, durationStr)
	} else {
		fmt.Printf("Added %s entry: %s - %s\n", statusStr, newEntry.Keyword, durationStr)
	}

	if IsVerbose() {
		fmt.Printf("Entry ID: %s (Short ID: %d)\n", newEntry.ID, newEntry.ShortID)
		fmt.Printf("Start time: %s\n", newEntry.StartTime.Format("Jan 2, 2006 3:04:05 PM"))
		if newEntry.EndTime != nil {
			fmt.Printf("End time: %s\n", newEntry.EndTime.Format("Jan 2, 2006 3:04:05 PM"))
		}
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}
