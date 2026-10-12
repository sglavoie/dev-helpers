package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

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
