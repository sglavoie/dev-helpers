package cmd

import (
	"fmt"
	"strconv"
	"strings"

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

// runDirectFieldSet applies one field=value pair given on the command line. Only
// the fields that can be expressed as a single argument pair are offered here;
// the timestamps belong to the interactive editors.
func runDirectFieldSet(entry *models.Entry, args []string, configManager *config.Manager, cfg *models.Config) error {
	if len(args) < 2 {
		return fmt.Errorf("must provide both field and value")
	}

	field := strings.ToLower(args[0])
	value := args[1]
	if field == "tags" {
		// Tags may arrive as several arguments when the commas are spaced out.
		value = strings.Join(args[1:], " ")
	}

	switch field {
	case "keyword", "duration", "tags":
	default:
		return fmt.Errorf("unsupported field: %s (supported: keyword, duration, tags)", field)
	}

	if err := setFieldValue(entry, field, value); err != nil {
		return err
	}

	switch field {
	case "keyword":
		fmt.Printf("Updated keyword to: %s\n", entry.Keyword)
	case "duration":
		// An active entry keeps its duration in its start time, so the value
		// that was asked for is what gets reported.
		seconds, _ := strconv.Atoi(value)
		fmt.Printf("Updated duration to: %s\n", formatDuration(seconds))
	case "tags":
		fmt.Printf("Updated tags to: %v\n", entry.Tags)
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
