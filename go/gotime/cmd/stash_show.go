package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

// stashShowCmd represents the stash show command
var stashShowCmd = &cobra.Command{
	Use:   "show",
	Short: "View and manage stashed entries",
	Long: `View all stashed entries in an interactive interface where you can select
specific entries to delete. If no stashed entries exist, displays a message
indicating that no stash is found.

Examples:
  gt stash show                    # View and manage stashed entries`,
	RunE: runStashShow,
}

func init() {
	stashCmd.AddCommand(stashShowCmd)
}

func runStashShow(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there are any stashed entries
	stashedEntries := cfg.GetStashedEntries()
	if len(stashedEntries) == 0 {
		fmt.Println("No stash found.")
		return nil
	}

	// Create selector items from stashed entries
	var items []tui.SelectorItem
	for _, entry := range stashedEntries {
		duration := formatDuration(entry.Duration)

		items = append(items, tui.SelectorItem{
			ID:   entry.ID,
			Data: &entry,
			Columns: []string{
				fmt.Sprintf("%d", entry.ShortID),
				entry.Keyword,
				fmt.Sprintf("%v", entry.Tags),
				"stashed",
				duration,
			},
		})
	}

	// Show multi-selector for deletion
	selectedItems, err := tui.RunMultiSelector("Select stashed entries to delete:", items)
	if err != nil {
		return err
	}

	if len(selectedItems) == 0 {
		fmt.Println("No entries selected for deletion.")
		return nil
	}

	// Build confirmation message
	var confirmMessage strings.Builder
	confirmMessage.WriteString("Are you sure you want to delete the following stashed entries?\n\n")

	for i, item := range selectedItems {
		entry := item.Data.(*models.Entry)
		duration := formatDuration(entry.Duration)
		confirmMessage.WriteString(fmt.Sprintf("%d. %s %v (ID: %d) - %s\n",
			i+1, entry.Keyword, entry.Tags, entry.ShortID, duration))
	}

	confirmed, err := tui.RunConfirm(confirmMessage.String())
	if err != nil {
		return fmt.Errorf("confirmation failed: %w", err)
	}
	if !confirmed {
		fmt.Println("Deletion cancelled.")
		return nil
	}

	// Delete all selected entries
	deletedCount := 0
	var deletedEntries []string

	for _, item := range selectedItems {
		entry := item.Data.(*models.Entry)

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

	// Display results
	fmt.Printf("Deleted %d stashed entries:\n", deletedCount)
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