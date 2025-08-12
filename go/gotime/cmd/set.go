package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/tui"
	"github.com/spf13/cobra"
)

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set [keyword | ID] [field value]",
	Short: "Set or update entry fields (supports bulk editing)",
	Long: `Set or update fields for one or multiple time tracking entries.
When called without arguments, displays an interactive multi-selection table to select entries to edit.
When called without field arguments, opens an interactive field editor showing all available fields.
You can also directly set specific fields by providing field and value arguments.
If multiple entries exist for a keyword, you'll be prompted to choose which ones to edit.
If called on running entries, they will be stopped to avoid ambiguity with the end time.

Examples:
  gt set                             # Interactive multi-selection and field editor
  gt set coding                      # Interactive field editor for "coding" entries
  gt set 3                           # Interactive field editor for entry ID 3
  gt set coding duration 3600        # Set duration to 3600 seconds (1 hour) for all "coding" entries
  gt set 3 keyword development       # Change keyword to "development"`,
	Args: cobra.ArbitraryArgs,
	RunE: runSet,
}

func init() {
	rootCmd.AddCommand(setCmd)
}

func runSet(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Entries) == 0 {
		return fmt.Errorf("no entries to set")
	}

	// Interactive selection if no arguments provided
	if len(args) == 0 {
		return runInteractiveEntrySelection(cfg, configManager)
	}

	// Find target entry using keyword/ID parsing
	var targetEntry *models.Entry

	parsedArg, err := ParseKeywordOrID(args[0], cfg)
	if err != nil {
		return err
	}

	if parsedArg.Type == ArgumentTypeID {
		// Set by ID
		targetEntry = parsedArg.Entry
	} else {
		// Set by keyword
		keyword := parsedArg.Keyword
		entries := cfg.GetNonStashedEntriesPtrsByKeyword(keyword)
		if len(entries) == 0 {
			return fmt.Errorf("no entries found for keyword '%s'", keyword)
		}

		if len(entries) == 1 {
			// Only one entry, use it directly
			targetEntry = entries[0]
		} else {
			// Multiple entries, prompt user to choose using bubble tea
			selectedEntry, err := runInteractiveEntrySelectionFromList(entries, keyword)
			if err != nil {
				return fmt.Errorf("failed to select entry: %w", err)
			}
			targetEntry = selectedEntry
		}
	}

	// Determine operation mode
	var fieldArgs []string
	if parsedArg.Type == ArgumentTypeID {
		fieldArgs = args[1:] // Skip ID argument
	} else {
		fieldArgs = args[1:] // Skip keyword argument
	}

	if len(fieldArgs) < 2 {
		// Interactive field editor mode
		return runInteractiveFieldEditor(targetEntry, configManager, cfg)
	} else {
		// Direct field setting mode
		return runDirectFieldSet(targetEntry, fieldArgs, configManager, cfg)
	}
}

func runInteractiveFieldEditor(entry *models.Entry, configManager *config.Manager, cfg *models.Config) error {
	// Make a copy of the entry to pass to the editor
	entryCopy := *entry

	// Run field editor TUI
	if err := tui.RunFieldEditor(&entryCopy); err != nil {
		return fmt.Errorf("field editing cancelled or failed: %w", err)
	}

	// Update the original entry with changes
	*entry = entryCopy

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Updated entry: %s %v\n", entry.Keyword, entry.Tags)

	if IsVerbose() {
		fmt.Printf("Entry ID: %s (Short ID: %d)\n", entry.ID, entry.ShortID)
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

func runDirectFieldSet(entry *models.Entry, args []string, configManager *config.Manager, cfg *models.Config) error {
	if len(args) < 2 {
		return fmt.Errorf("must provide both field and value")
	}

	field := strings.ToLower(args[0])
	value := args[1]

	switch field {
	case "keyword":
		entry.Keyword = value
		fmt.Printf("Updated keyword to: %s\n", value)

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

		fmt.Printf("Updated duration to: %s\n", formatDuration(duration))

	case "tags":
		// Parse comma-separated tags
		tagsStr := strings.Join(args[1:], " ") // Join all remaining args for tags
		tags := strings.Split(tagsStr, ",")
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
		fmt.Printf("Updated tags to: %v\n", cleanTags)

	default:
		return fmt.Errorf("unsupported field: %s (supported: keyword, duration, tags)", field)
	}

	// Save configuration
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if IsVerbose() {
		fmt.Printf("Entry ID: %s (Short ID: %d)\n", entry.ID, entry.ShortID)
		fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
	}

	return nil
}

func runInteractiveEntrySelection(cfg *models.Config, configManager *config.Manager) error {
	// Get all non-stashed entries and sort by StartTime descending (most recent first)
	entries := cfg.GetNonStashedEntries()
	SortEntriesByStartTimeDesc(entries)

	// Create selector items from sorted entries
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
			Data: &entry,
			Columns: []string{
				fmt.Sprintf("%d", entry.ShortID),
				entry.Keyword,
				fmt.Sprintf("%v", entry.Tags),
				status,
				duration,
			},
		})
	}

	// Show multi-selector for bulk editing
	selectedItems, err := tui.RunMultiSelector("Select entries to edit (supports bulk editing):", items)
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
	SortEntriesPtrsByStartTimeDesc(entries)

	// Create selector items from the sorted entries
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

	// Show selector
	title := fmt.Sprintf("Select entry for keyword '%s':", keyword)
	selected, err := tui.RunSelector(title, items)
	if err != nil {
		return nil, err
	}

	// Return the selected entry
	return selected.Data.(*models.Entry), nil
}

// runBulkFieldEditor handles bulk editing of multiple entries
func runBulkFieldEditor(entries []*models.Entry, configManager *config.Manager, cfg *models.Config) error {
	fmt.Printf("Bulk editing %d entries:\n", len(entries))
	for i, entry := range entries {
		fmt.Printf("  %d. %s %v (ID: %d)\n", i+1, entry.Keyword, entry.Tags, entry.ShortID)
	}
	fmt.Println()

	// Ask which field to bulk edit
	fmt.Println("Available fields for bulk editing:")
	fmt.Println("  1. keyword    - Change keyword for all selected entries")
	fmt.Println("  2. tags       - Replace tags for all selected entries")
	fmt.Println("  3. duration   - Set duration (in seconds) for all selected entries")
	fmt.Println("  4. starttime  - Set start time (YYYY-MM-DD HH:MM:SS) for all selected entries")
	fmt.Println()

	// Map digit to field name
	fieldMap := map[string]string{
		"1": "keyword",
		"2": "tags",
		"3": "duration",
		"4": "starttime",
	}
	var fieldInput string
	var field string

	// Prompt for a valid field selection (1-4) in a loop
	for {
		fmt.Print("Enter field name to edit (1-4): ")
		fmt.Scanln(&fieldInput)
		fieldInput = strings.TrimSpace(fieldInput)
		if fieldInput == "" {
			fmt.Println("Operation cancelled.")
			return nil
		}
		if mapped, ok := fieldMap[fieldInput]; ok {
			field = mapped
			break
		}
		// If not a digit, assume user entered the field name directly
		field = strings.ToLower(fieldInput)
		if field == "keyword" || field == "tags" || field == "duration" || field == "starttime" {
			break
		}
		fmt.Println("Invalid selection. Please enter a number between 1 and 4.")
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
	if modifiedCount > 0 {
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
	} else {
		fmt.Println("No entries were modified.")
	}

	return nil
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

	default:
		return fmt.Errorf("unsupported field: %s (supported: keyword, duration, tags, starttime)", field)
	}

	return nil
}
