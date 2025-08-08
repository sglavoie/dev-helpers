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
	Short: "Set or update entry fields",
	Long: `Set or update fields for a time tracking entry.
When called without arguments, displays an interactive table to select an entry to edit.
When called without field arguments, opens an interactive field editor showing all available fields.
You can also directly set specific fields by providing field and value arguments.
If multiple entries exist for a keyword, you'll be prompted to choose which one to edit.

Examples:
  gt set                             # Interactive entry selection and field editor
  gt set coding                      # Interactive field editor for "coding"
  gt set 3                           # Interactive field editor for entry ID 3
  gt set coding duration 3600        # Set duration to 3600 seconds (1 hour)
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
		entries := cfg.GetEntriesPtrsByKeyword(keyword)
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
	// Create selector items from all entries
	var items []tui.SelectorItem
	for _, entry := range cfg.Entries {
		status := "completed"
		duration := formatDuration(entry.Duration)
		if entry.Active {
			status = "running"
			duration = formatDuration(entry.GetCurrentDuration())
		}

		displayText := fmt.Sprintf("ID:%d | %s %v | %s | %s",
			entry.ShortID,
			entry.Keyword,
			entry.Tags,
			status,
			duration,
		)

		items = append(items, tui.SelectorItem{
			ID:          entry.ID,
			DisplayText: displayText,
			Data:        &entry,
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
	selected, err := tui.RunSelector("Select entry to edit:", items)
	if err != nil {
		return err
	}

	// Get the selected entry
	selectedID := selected.Data.(*models.Entry).ID
	var targetEntry *models.Entry

	for i := range cfg.Entries {
		if cfg.Entries[i].ID == selectedID {
			targetEntry = &cfg.Entries[i]
			break
		}
	}

	if targetEntry == nil {
		return fmt.Errorf("selected entry not found")
	}

	// Open interactive field editor
	return runInteractiveFieldEditor(targetEntry, configManager, cfg)
}

func runInteractiveEntrySelectionFromList(entries []*models.Entry, keyword string) (*models.Entry, error) {
	// Create selector items from the provided entries
	var items []tui.SelectorItem
	for _, entry := range entries {
		status := "completed"
		duration := formatDuration(entry.Duration)
		if entry.Active {
			status = "running"
			duration = formatDuration(entry.GetCurrentDuration())
		}

		displayText := fmt.Sprintf("ID:%d | %s %v | %s | %s",
			entry.ShortID,
			entry.Keyword,
			entry.Tags,
			status,
			duration,
		)

		items = append(items, tui.SelectorItem{
			ID:          entry.ID,
			DisplayText: displayText,
			Data:        entry,
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
