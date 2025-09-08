package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

// stashClearCmd represents the stash clear command
var stashClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all stashed entries",
	Long: `Delete all stashed entries permanently after confirmation.
This operation cannot be undone, but provides an undo record for recovery.

Examples:
  gt stash clear                   # Delete all stashed entries with confirmation`,
	RunE: runStashClear,
}

func init() {
	stashCmd.AddCommand(stashClearCmd)
}

func runStashClear(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there are any stashed entries
	stashedEntries := cfg.GetStashedEntries()
	if len(stashedEntries) == 0 {
		fmt.Println("No stash to clear.")
		return nil
	}

	// Build confirmation message
	var confirmMessage strings.Builder
	confirmMessage.WriteString(fmt.Sprintf("Are you sure you want to delete all %d stashed entries?\n\n", len(stashedEntries)))

	for i, entry := range stashedEntries {
		duration := formatDuration(entry.Duration)
		confirmMessage.WriteString(fmt.Sprintf("%d. %s %v (ID: %d) - %s\n",
			i+1, entry.Keyword, entry.Tags, entry.ShortID, duration))
	}

	confirmed, err := tui.RunConfirm(confirmMessage.String())
	if err != nil {
		return fmt.Errorf("confirmation failed: %w", err)
	}
	if !confirmed {
		fmt.Println("Clear operation cancelled.")
		return nil
	}

	// Record undo information before deletion
	var currentStashes []models.Stash
	if stash := cfg.GetActiveStash(); stash != nil {
		currentStashes = append(currentStashes, *stash)
	}

	undoData := map[string]interface{}{
		"entries": stashedEntries,
		"stashes": currentStashes,
	}

	// Delete all stashed entries
	deletedCount := 0
	var deletedEntries []string

	for _, entry := range stashedEntries {
		// Capture display info before removal
		keyword := entry.Keyword
		tags := entry.Tags
		shortID := entry.ShortID
		duration := formatDuration(entry.Duration)

		if cfg.RemoveEntry(entry.ID) {
			deletedCount++
			deletedEntries = append(deletedEntries,
				fmt.Sprintf("%s %v (ID: %d) - %s", keyword, tags, shortID, duration))
		}
	}

	// Remove the stash
	if stash := cfg.GetActiveStash(); stash != nil {
		cfg.RemoveStash(stash.ID)
	}

	// Add undo record if any entries were deleted
	if deletedCount > 0 {
		description := fmt.Sprintf("Cleared %d stashed entries", deletedCount)
		cfg.AddUndoRecord(models.UndoOperationClear, description, undoData)
	}

	// Display results
	fmt.Printf("Deleted %d stashed entries. Use 'gt undo' to restore.\n", deletedCount)
	for _, entryDesc := range deletedEntries {
		fmt.Printf("  • %s\n", entryDesc)
	}

	// Save configuration
	if deletedCount > 0 {
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		if IsVerbose() {
			fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
		}
	}

	return nil
}