package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

// undoCmd represents the undo command
var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo the last destructive operation or list available operations",
	Long: `Undo destructive operations such as delete, bulk edit, or stash clear.
Displays a table showing all undoable operations with their affected entries.
You can select multiple operations to restore at once.
  
Note: You can restore operations in any order. Undo history is maintained for the last 10 operations.`,
	Args:    cobra.NoArgs,
	RunE:    runUndo,
	Aliases: []string{"u"},
}

func init() {
	rootCmd.AddCommand(undoCmd)

	// No flags needed for the simplified undo command
}

func runUndo(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there's anything to undo
	if !cfg.HasUndoHistory() {
		fmt.Println("No operations to undo.")
		return nil
	}

	// Run interactive table view for undo selection
	return runInteractiveUndo(cfg, configManager)
}

// undoDelete restores deleted entries
func undoDelete(cfg *models.Config, configManager *config.Manager, record *models.UndoRecord) error {
	// Extract the deleted entries from the undo data
	entriesData, ok := record.Data["entries"]
	if !ok {
		return fmt.Errorf("invalid undo data: missing entries")
	}

	// Convert the data back to Entry objects
	var deletedEntries []models.Entry
	entriesBytes, err := json.Marshal(entriesData)
	if err != nil {
		return fmt.Errorf("failed to process undo data: %w", err)
	}

	if err := json.Unmarshal(entriesBytes, &deletedEntries); err != nil {
		return fmt.Errorf("failed to restore deleted entries: %w", err)
	}

	// Restore the entries
	restoredCount := 0
	var restoredDescriptions []string

	for _, entry := range deletedEntries {
		// Add the entry back
		cfg.Entries = append(cfg.Entries, entry)
		restoredCount++

		// Prepare display info
		tags := ""
		if len(entry.Tags) > 0 {
			tags = fmt.Sprintf(" %v", entry.Tags)
		}
		duration := formatDuration(entry.Duration)
		status := "stopped"
		if entry.Active {
			status = "active"
		} else if entry.Stashed {
			status = "stashed"
		}
		restoredDescriptions = append(restoredDescriptions,
			fmt.Sprintf("  • %s%s (ID: %d) - %s - %s", entry.Keyword, tags, entry.ShortID, duration, status))
	}

	// Update short IDs
	cfg.UpdateShortIDs()

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Undone: %s\n", record.Description)
	fmt.Printf("Restored %d entries:\n", restoredCount)
	for _, desc := range restoredDescriptions {
		fmt.Println(desc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

// undoBulkEdit restores entries to their previous state before bulk editing
func undoBulkEdit(cfg *models.Config, configManager *config.Manager, record *models.UndoRecord) error {
	// Extract the original entries from the undo data
	originalData, ok := record.Data["original_entries"]
	if !ok {
		return fmt.Errorf("invalid undo data: missing original entries")
	}

	// Convert the data back to Entry objects
	var originalEntries []models.Entry
	entriesBytes, err := json.Marshal(originalData)
	if err != nil {
		return fmt.Errorf("failed to process undo data: %w", err)
	}

	if err := json.Unmarshal(entriesBytes, &originalEntries); err != nil {
		return fmt.Errorf("failed to restore original entries: %w", err)
	}

	// Restore the original state of the entries
	restoredCount := 0
	var restoredDescriptions []string

	for _, originalEntry := range originalEntries {
		// Find and replace the current entry with the original
		for i := range cfg.Entries {
			if cfg.Entries[i].ID == originalEntry.ID {
				cfg.Entries[i] = originalEntry
				restoredCount++

				// Prepare display info
				tags := ""
				if len(originalEntry.Tags) > 0 {
					tags = fmt.Sprintf(" %v", originalEntry.Tags)
				}
				duration := formatDuration(originalEntry.Duration)
				status := "stopped"
				if originalEntry.Active {
					status = "active"
				} else if originalEntry.Stashed {
					status = "stashed"
				}
				restoredDescriptions = append(restoredDescriptions,
					fmt.Sprintf("  • %s%s (ID: %d) - %s - %s", originalEntry.Keyword, tags, originalEntry.ShortID, duration, status))
				break
			}
		}
	}

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Undone: %s\n", record.Description)
	fmt.Printf("Restored %d entries to their original state:\n", restoredCount)
	for _, desc := range restoredDescriptions {
		fmt.Println(desc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

// undoClear restores entries that were cleared/deleted in bulk
func undoClear(cfg *models.Config, configManager *config.Manager, record *models.UndoRecord) error {
	// Extract the cleared entries from the undo data
	entriesData, ok := record.Data["entries"]
	if !ok {
		return fmt.Errorf("invalid undo data: missing entries")
	}

	// Also restore any stashes if they existed
	stashesData, hasStashes := record.Data["stashes"]

	// Convert the entries data back to Entry objects
	var clearedEntries []models.Entry
	entriesBytes, err := json.Marshal(entriesData)
	if err != nil {
		return fmt.Errorf("failed to process undo data: %w", err)
	}

	if err := json.Unmarshal(entriesBytes, &clearedEntries); err != nil {
		return fmt.Errorf("failed to restore cleared entries: %w", err)
	}

	// Restore stashes if they existed
	if hasStashes {
		var clearedStashes []models.Stash
		stashesBytes, err := json.Marshal(stashesData)
		if err != nil {
			return fmt.Errorf("failed to process stash undo data: %w", err)
		}

		if err := json.Unmarshal(stashesBytes, &clearedStashes); err != nil {
			return fmt.Errorf("failed to restore stashes: %w", err)
		}

		// Restore the stashes
		cfg.Stashes = append(cfg.Stashes, clearedStashes...)
	}

	// Restore the entries
	restoredCount := 0
	var restoredDescriptions []string

	for _, entry := range clearedEntries {
		// Add the entry back
		cfg.Entries = append(cfg.Entries, entry)
		restoredCount++

		// Prepare display info
		tags := ""
		if len(entry.Tags) > 0 {
			tags = fmt.Sprintf(" %v", entry.Tags)
		}
		duration := formatDuration(entry.Duration)
		status := "stopped"
		if entry.Active {
			status = "active"
		} else if entry.Stashed {
			status = "stashed"
		}
		restoredDescriptions = append(restoredDescriptions,
			fmt.Sprintf("  • %s%s (ID: %d) - %s - %s", entry.Keyword, tags, entry.ShortID, duration, status))
	}

	// Update short IDs
	cfg.UpdateShortIDs()

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display results
	fmt.Printf("Undone: %s\n", record.Description)
	fmt.Printf("Restored %d entries:\n", restoredCount)
	for _, desc := range restoredDescriptions {
		fmt.Println(desc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

// extractEntriesFromUndoRecord extracts entries from an UndoRecord's data
func extractEntriesFromUndoRecord(record *models.UndoRecord) ([]models.Entry, error) {
	var entries []models.Entry

	// Handle different operation types
	var entriesData any
	var ok bool

	switch record.Operation {
	case models.UndoOperationDelete, models.UndoOperationClear:
		entriesData, ok = record.Data["entries"]
	case models.UndoOperationBulkEdit:
		entriesData, ok = record.Data["original_entries"]
	default:
		return nil, fmt.Errorf("unsupported undo operation: %s", record.Operation)
	}

	if !ok {
		return nil, fmt.Errorf("invalid undo data: missing entries")
	}

	// Convert the data back to Entry objects
	entriesBytes, err := json.Marshal(entriesData)
	if err != nil {
		return nil, fmt.Errorf("failed to process undo data: %w", err)
	}

	if err := json.Unmarshal(entriesBytes, &entries); err != nil {
		return nil, fmt.Errorf("failed to restore entries: %w", err)
	}

	return entries, nil
}

// runUndoList function removed - now using interactive table display only

// runInteractiveUndo displays available undo operations as individual entry rows
func runInteractiveUndo(cfg *models.Config, configManager *config.Manager) error {
	if len(cfg.UndoHistory) == 0 {
		fmt.Println("No operations available to undo.")
		return nil
	}

	// Create selector items from undo history (reverse order to show most recent first)
	var items []tui.SelectorItem

	for i := len(cfg.UndoHistory) - 1; i >= 0; i-- {
		record := cfg.UndoHistory[i]

		// Extract entries from this operation
		entries, err := extractEntriesFromUndoRecord(&record)
		if err != nil {
			// If we can't extract entries, show operation summary instead
			fmt.Printf("Warning: Could not extract entries from operation %s: %v\n", record.Description, err)
			continue
		}

		// Calculate operation number and relative time
		operationNumber := len(cfg.UndoHistory) - i
		timeSince := time.Since(record.Timestamp)
		var relativeTime string
		if timeSince < time.Hour {
			relativeTime = fmt.Sprintf("%dm ago", int(timeSince.Minutes()))
		} else if timeSince < 24*time.Hour {
			relativeTime = fmt.Sprintf("%dh ago", int(timeSince.Hours()))
		} else {
			days := int(timeSince.Hours() / 24)
			if days == 1 {
				relativeTime = "1 day ago"
			} else {
				relativeTime = fmt.Sprintf("%d days ago", days)
			}
		}

		// Add each entry as a selectable item
		for entryIdx, entry := range entries {
			// Format entry details
			tags := ""
			if len(entry.Tags) > 0 {
				tags = fmt.Sprintf("%v", entry.Tags)
			}

			duration := formatDuration(entry.Duration)

			// Create unique ID for this entry within the operation
			entryItemID := fmt.Sprintf("%s_entry_%d", record.ID, entryIdx)

			items = append(items, tui.SelectorItem{
				ID:   entryItemID,
				Data: &record, // Store the operation record for restoration
				Columns: []string{
					fmt.Sprintf("#%d", operationNumber), // Operation number
					string(record.Operation),            // Operation type
					entry.Keyword,                       // Entry keyword
					tags,                                // Entry tags
					duration,                            // Entry duration
					relativeTime,                        // Time ago
				},
			})
		}
	}

	// Show multi-selector for choosing which entries to restore
	selectedItems, err := tui.RunMultiSelector("Select entries to restore:", items)
	if err != nil {
		return err
	}

	if len(selectedItems) == 0 {
		fmt.Println("No entries selected for restoration.")
		return nil
	}

	// Extract unique operations from selected items
	selectedOperations := make(map[string]*models.UndoRecord)
	for _, item := range selectedItems {
		record := item.Data.(*models.UndoRecord)
		selectedOperations[record.ID] = record
	}

	// Show confirmation prompt
	return confirmAndExecuteRestore(cfg, configManager, selectedOperations)
}

// confirmAndExecuteRestore shows confirmation and executes the restore operations
func confirmAndExecuteRestore(cfg *models.Config, configManager *config.Manager, selectedOperations map[string]*models.UndoRecord) error {
	// Build confirmation message
	var confirmationBuilder strings.Builder
	confirmationBuilder.WriteString(fmt.Sprintf("Are you sure you want to restore %d operation(s)?\n\n", len(selectedOperations)))

	totalEntries := 0
	for _, record := range selectedOperations {
		entries, err := extractEntriesFromUndoRecord(record)
		if err != nil {
			confirmationBuilder.WriteString(fmt.Sprintf("• %s (entries could not be counted: %v)\n", record.Description, err))
		} else {
			totalEntries += len(entries)
			confirmationBuilder.WriteString(fmt.Sprintf("• %s (%d entries)\n", record.Description, len(entries)))
		}
	}

	confirmationBuilder.WriteString(fmt.Sprintf("\nTotal entries to restore: %d", totalEntries))

	// Show confirmation dialog
	confirmed, err := tui.RunConfirm(confirmationBuilder.String())
	if err != nil {
		return fmt.Errorf("confirmation failed: %w", err)
	}

	if !confirmed {
		fmt.Println("Restoration cancelled.")
		return nil
	}

	// Execute the restore operations
	return executeRestore(cfg, configManager, selectedOperations)
}

// executeRestore executes the selected restore operations
func executeRestore(cfg *models.Config, configManager *config.Manager, selectedOperations map[string]*models.UndoRecord) error {
	var restoredOperations []string
	var totalRestoredEntries int

	// Execute each restore operation
	for _, record := range selectedOperations {
		var undoErr error
		switch record.Operation {
		case models.UndoOperationDelete:
			undoErr = undoDelete(cfg, configManager, record)
		case models.UndoOperationBulkEdit:
			undoErr = undoBulkEdit(cfg, configManager, record)
		case models.UndoOperationClear:
			undoErr = undoClear(cfg, configManager, record)
		default:
			return fmt.Errorf("unsupported undo operation: %s", record.Operation)
		}

		if undoErr != nil {
			return fmt.Errorf("failed to restore operation '%s': %w", record.Description, undoErr)
		}

		// Count restored entries for summary
		entries, err := extractEntriesFromUndoRecord(record)
		if err == nil {
			totalRestoredEntries += len(entries)
		}
		restoredOperations = append(restoredOperations, record.Description)
	}

	// Remove all restored operations from undo history
	var remainingHistory []models.UndoRecord
	for _, historyRecord := range cfg.UndoHistory {
		if _, wasRestored := selectedOperations[historyRecord.ID]; !wasRestored {
			remainingHistory = append(remainingHistory, historyRecord)
		}
	}
	cfg.UndoHistory = remainingHistory

	// Save configuration to persist the changes and updated history
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config after restoration: %w", err)
	}

	// Display success message
	fmt.Printf("Successfully restored %d operations (%d entries total):\n",
		len(restoredOperations), totalRestoredEntries)
	for _, desc := range restoredOperations {
		fmt.Printf("  ✓ %s\n", desc)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}
