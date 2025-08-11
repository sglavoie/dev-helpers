package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

// stashCmd represents the stash command
var stashCmd = &cobra.Command{
	Use:   "stash [show|apply|clear]",
	Short: "Stash all running entries or manage stashed entries",
	Long: `Stash all currently running entries to temporarily pause them.
When no arguments are provided, stashes all active entries.
Use subcommands to manage stashed entries.

Examples:
  gt stash                           # Stash all running entries
  gt stash show                      # View and manage stashed entries
  gt stash apply                     # Unstash entries as stopped entries
  gt stash clear                     # Delete all stashed entries`,
	Args:    cobra.MaximumNArgs(1),
	RunE:    runStash,
	Aliases: []string{"s"},
}

func init() {
	rootCmd.AddCommand(stashCmd)
}

func runStash(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check for subcommands
	if len(args) > 0 {
		switch args[0] {
		case "show":
			return runStashShow(cfg, configManager)
		case "apply":
			return runStashApply(cfg, configManager)
		case "clear":
			return runStashClear(cfg, configManager)
		default:
			return fmt.Errorf("unknown subcommand: %s (available: show, apply, clear)", args[0])
		}
	}

	// Default stash behavior: stash all running entries
	return runStashDefault(cfg, configManager)
}

func runStashDefault(cfg *models.Config, configManager *config.Manager) error {
	// Get all active entries
	activeEntries := cfg.GetActiveEntries()

	if len(activeEntries) == 0 {
		fmt.Println("No entries to stash.")
		fmt.Println("Use 'gt stash show' to view existing stashes.")
		return nil
	}

	// Check if a stash already exists
	if cfg.HasActiveStash() {
		return fmt.Errorf("stash already exists, use 'stop' command first or 'stop --all' to stop all running entries\nNote: this program supports a single stash only")
	}

	// Collect entry IDs and stop all active entries
	var entryIDs []string
	var stashedEntries []string

	for i := range cfg.Entries {
		entry := &cfg.Entries[i]
		if entry.Active {
			// Stop the entry and calculate duration
			entry.Stop()
			// Mark as stashed
			entry.Stashed = true
			// Add to stash
			entryIDs = append(entryIDs, entry.ID)
			// Prepare display info
			tags := ""
			if len(entry.Tags) > 0 {
				tags = fmt.Sprintf(" %v", entry.Tags)
			}
			duration := formatDuration(entry.Duration)
			stashedEntries = append(stashedEntries, fmt.Sprintf("  • %s%s - %s", entry.Keyword, tags, duration))
		}
	}

	// Create the stash
	cfg.CreateStash(entryIDs)

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Stashed %d entries:\n", len(entryIDs))
	for _, entryDesc := range stashedEntries {
		fmt.Println(entryDesc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

func runStashShow(cfg *models.Config, configManager *config.Manager) error {
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

// runStashApply unstashes all stashed entries and makes them stopped entries
func runStashApply(cfg *models.Config, configManager *config.Manager) error {
	// Check if there are any stashed entries
	stashedEntries := cfg.GetStashedEntriesPtr()
	if len(stashedEntries) == 0 {
		fmt.Println("No stash to apply.")
		return nil
	}

	var appliedEntries []string
	appliedCount := 0

	// Apply all stashed entries (convert to stopped entries)
	for _, entry := range stashedEntries {
		// Unstash the entry by setting Stashed = false
		// Keep Active = false so it remains a stopped entry
		entry.Stashed = false
		appliedCount++

		// Prepare display info
		tags := ""
		if len(entry.Tags) > 0 {
			tags = fmt.Sprintf(" %v", entry.Tags)
		}
		duration := formatDuration(entry.Duration)
		appliedEntries = append(appliedEntries, fmt.Sprintf("  • %s%s - %s", entry.Keyword, tags, duration))
	}

	// Remove the stash since all entries are now unstashed
	if stash := cfg.GetActiveStash(); stash != nil {
		cfg.RemoveStash(stash.ID)
	}

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Applied %d stashed entries (converted to stopped entries):\n", appliedCount)
	for _, entryDesc := range appliedEntries {
		fmt.Println(entryDesc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

// runStashClear deletes all stashed entries
func runStashClear(cfg *models.Config, configManager *config.Manager) error {
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
