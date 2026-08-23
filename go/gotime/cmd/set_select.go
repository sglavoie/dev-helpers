package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
)

// entrySelectorItems renders entries as selector rows: short ID, keyword, tags,
// whether the entry is still running, and the time it has accumulated.
func entrySelectorItems(entries []*models.Entry) []tui.SelectorItem {
	var items []tui.SelectorItem
	for _, entry := range entries {
		status := "completed"
		duration := formatDuration(entry.Duration)
		if entry.Active {
			status = "running"
			duration = formatDuration(entry.GetCurrentDuration())
		}

		items = append(items, tui.SelectorItem{
			ID:   entry.ID,
			Data: entry,
			Columns: []string{
				fmt.Sprintf("%d", entry.ShortID),
				entry.Keyword,
				fmt.Sprintf("%v", entry.Tags),
				status,
				duration,
			},
		})
	}
	return items
}

func runInteractiveEntrySelection(cfg *models.Config, configManager *config.Manager) error {
	// Get all non-stashed entries and sort by StartTime descending (most recent first)
	entries := cfg.GetNonStashedEntries()
	SortEntries(entries, ByStartTime, Descending)

	selectable := make([]*models.Entry, 0, len(entries))
	for i := range entries {
		selectable = append(selectable, &entries[i])
	}

	// Show multi-selector for bulk editing
	selectedItems, err := tui.RunMultiSelector("Select entries to edit (supports bulk editing):", entrySelectorItems(selectable))
	if err != nil {
		return err
	}

	if len(selectedItems) == 0 {
		fmt.Println("No entries selected for editing.")
		return nil
	}

	// Get the selected entries
	var targetEntries []*models.Entry
	for _, item := range selectedItems {
		entry := item.Data.(*models.Entry)
		// Find the actual entry in the config
		for i := range cfg.Entries {
			if cfg.Entries[i].ID == entry.ID {
				targetEntries = append(targetEntries, &cfg.Entries[i])
				break
			}
		}
	}

	if len(targetEntries) == 0 {
		return fmt.Errorf("no valid entries found")
	}

	// Handle single vs bulk editing
	if len(targetEntries) == 1 {
		// Single entry - use existing field editor
		return runInteractiveFieldEditor(targetEntries[0], configManager, cfg)
	} else {
		// Multiple entries - bulk edit mode
		return runBulkFieldEditor(targetEntries, configManager, cfg)
	}
}

func runInteractiveEntrySelectionFromList(entries []*models.Entry, keyword string) (*models.Entry, error) {
	// Sort entries by StartTime descending (most recent first)
	SortEntries(entries, ByStartTime, Descending)

	// Show selector
	title := fmt.Sprintf("Select entry for keyword '%s':", keyword)
	selected, err := tui.RunSelector(title, entrySelectorItems(entries))
	if err != nil {
		return nil, err
	}

	// Return the selected entry
	return selected.Data.(*models.Entry), nil
}
