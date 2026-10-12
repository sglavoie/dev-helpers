package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
)

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

	// Define explicit column configuration for undo entries
	tableConfig := tui.TableConfig{
		Columns: []table.Column{
			{Title: "#", Width: 4},         // Operation number
			{Title: "Type", Width: 10},     // Operation type
			{Title: "Keyword", Width: 15},  // Entry keyword
			{Title: "Tags", Width: 26},     // Entry tags
			{Title: "Duration", Width: 12}, // Entry duration
			{Title: "Time", Width: 16},     // Time ago
		},
	}

	// Show multi-selector for choosing which entries to restore
	selectedItems, err := tui.RunMultiSelectorWithConfig("Select entries to restore:", items, tableConfig)
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
