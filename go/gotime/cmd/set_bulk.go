package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
)

// bulkFields maps the digit shown in the bulk menu to the field it stands for.
var bulkFields = map[string]string{
	"1": "keyword",
	"2": "tags",
	"3": "duration",
	"4": "starttime",
	"5": "endtime",
}

// runBulkFieldEditor handles bulk editing of multiple entries
func runBulkFieldEditor(entries []*models.Entry, configManager *config.Manager, cfg *models.Config) error {
	fmt.Printf("Bulk editing %d entries:\n", len(entries))
	for i, entry := range entries {
		fmt.Printf("  %d. %s %v (ID: %d)\n", i+1, entry.Keyword, entry.Tags, entry.ShortID)
	}
	fmt.Println()

	field, ok := promptBulkField()
	if !ok {
		fmt.Println("Operation cancelled.")
		return nil
	}

	fmt.Printf("Enter new value for '%s': ", field)
	var value string
	fmt.Scanln(&value)

	if value == "" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// Build confirmation message
	var confirmMessage strings.Builder
	confirmMessage.WriteString(fmt.Sprintf("Are you sure you want to set '%s' to '%s' for the following %d entries?\n\n",
		field, value, len(entries)))

	for i, entry := range entries {
		confirmMessage.WriteString(fmt.Sprintf("%d. %s %v (ID: %d)\n",
			i+1, entry.Keyword, entry.Tags, entry.ShortID))
	}

	confirmed, err := tui.RunConfirm(confirmMessage.String())
	if err != nil {
		return fmt.Errorf("confirmation failed: %w", err)
	}
	if !confirmed {
		fmt.Println("Bulk edit cancelled.")
		return nil
	}

	// Record original state for undo before making changes
	var originalEntries []models.Entry
	for _, entry := range entries {
		originalEntries = append(originalEntries, *entry) // Create snapshot
	}

	undoData := map[string]interface{}{
		"original_entries": originalEntries,
	}

	// Apply the field change to all selected entries
	modifiedCount := 0
	var modifiedEntries []string

	for _, entry := range entries {
		// Stop active entries before modifying them
		wasActive := entry.Active
		if wasActive {
			entry.Stop()
		}

		// Apply the field change
		err := setFieldValue(entry, field, value)
		if err != nil {
			fmt.Printf("Warning: Failed to set %s for entry %d: %v\n", field, entry.ShortID, err)
			continue
		}

		modifiedCount++
		status := "completed"
		if wasActive {
			status = "was running (now stopped)"
		}
		modifiedEntries = append(modifiedEntries,
			fmt.Sprintf("%s %v (ID: %d) - %s", entry.Keyword, entry.Tags, entry.ShortID, status))
	}

	// Display results
	if modifiedCount == 0 {
		fmt.Println("No entries were modified.")
		return nil
	}

	// Add undo record for bulk edit
	description := fmt.Sprintf("Bulk edited %d entries (field: %s)", modifiedCount, field)
	cfg.AddUndoRecord(models.UndoOperationBulkEdit, description, undoData)

	fmt.Printf("Successfully modified %d entries. Use 'gt undo' to restore.\n", modifiedCount)
	for _, entryDesc := range modifiedEntries {
		fmt.Printf("  • %s\n", entryDesc)
	}

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if IsVerbose() {
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

// promptBulkField asks which field to bulk edit until the answer names one,
// accepting either its menu digit or its name. An empty answer cancels.
func promptBulkField() (string, bool) {
	fmt.Println("Available fields for bulk editing:")
	fmt.Println("  1. keyword    - Change keyword for all selected entries")
	fmt.Println("  2. tags       - Replace tags for all selected entries")
	fmt.Println("  3. duration   - Set duration (in seconds) for all selected entries")
	fmt.Println("  4. starttime  - Set start time (YYYY-MM-DD HH:MM:SS) for all selected entries")
	fmt.Println("  5. endtime    - Set end time (YYYY-MM-DD HH:MM:SS) for all selected entries")
	fmt.Println()

	var fieldInput string
	for {
		fmt.Print("Enter field name to edit (1-5): ")
		fmt.Scanln(&fieldInput)
		fieldInput = strings.TrimSpace(fieldInput)
		if fieldInput == "" {
			return "", false
		}
		if mapped, ok := bulkFields[fieldInput]; ok {
			return mapped, true
		}
		// If not a digit, assume user entered the field name directly
		field := strings.ToLower(fieldInput)
		for _, known := range bulkFields {
			if field == known {
				return field, true
			}
		}
		fmt.Println("Invalid selection. Please enter a number between 1 and 5.")
	}
}

// setFieldValue applies a field change to a single entry
func setFieldValue(entry *models.Entry, field, value string) error {
	field = strings.ToLower(field)

	switch field {
	case "keyword":
		entry.Keyword = value

	case "duration":
		duration, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid duration value: %s", value)
		}
		if duration < 0 {
			return fmt.Errorf("duration cannot be negative")
		}

		if entry.Active {
			// For active entries, adjust the start time
			now := time.Now()
			newStartTime := now.Add(-time.Duration(duration) * time.Second)
			entry.StartTime = newStartTime
		} else {
			entry.Duration = duration
		}

	case "tags":
		// Parse comma-separated tags
		tags := strings.Split(value, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}

		// Remove empty tags
		var cleanTags []string
		for _, tag := range tags {
			if tag != "" {
				cleanTags = append(cleanTags, tag)
			}
		}

		entry.Tags = cleanTags

	case "starttime":
		// Parse start time in YYYY-MM-DD HH:MM:SS format
		startTime, err := time.Parse("2006-01-02 15:04:05", value)
		if err != nil {
			return fmt.Errorf("invalid start time format (expected YYYY-MM-DD HH:MM:SS): %s", value)
		}
		entry.StartTime = startTime

	case "endtime":
		// Parse end time in YYYY-MM-DD HH:MM:SS format
		endTime, err := time.Parse("2006-01-02 15:04:05", value)
		if err != nil {
			return fmt.Errorf("invalid end time format (expected YYYY-MM-DD HH:MM:SS): %s", value)
		}
		entry.EndTime = &endTime
		entry.Duration = int(endTime.Sub(entry.StartTime).Seconds())

	default:
		return fmt.Errorf("unsupported field: %s (supported: keyword, duration, tags, starttime, endtime)", field)
	}

	return nil
}
