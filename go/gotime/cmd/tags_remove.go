package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/spf13/cobra"
)

var (
	removeTagID      int
	removeTagKeyword string
)

// tagsRemoveCmd represents the tags remove command
var tagsRemoveCmd = &cobra.Command{
	Use:   "remove <tag> [keyword]",
	Short: "Remove a tag from entries",
	Long: `Remove a tag from time tracking entries.

Without any flags or additional arguments, removes the tag from ALL entries.
With --id flag, removes the tag only from the specified entry.
With a keyword argument, removes the tag only from entries with that keyword.

Examples:
  gt tags remove deprecated          # Remove 'deprecated' tag from all entries
  gt tags remove work --id 5         # Remove 'work' tag only from entry ID 5
  gt tags remove meeting coding      # Remove 'meeting' tag only from entries with 'coding' keyword`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTagsRemove,
}

func init() {
	tagsCmd.AddCommand(tagsRemoveCmd)

	tagsRemoveCmd.Flags().IntVar(&removeTagID, "id", 0, "remove tag only from entry with specified short ID")
	tagsRemoveCmd.Flags().StringVar(&removeTagKeyword, "keyword", "", "remove tag only from entries with specified keyword (alternative to positional keyword argument)")
}

func runTagsRemove(cmd *cobra.Command, args []string) error {
	tagToRemove := args[0]
	var keyword string

	// Handle keyword from either positional argument or flag
	if len(args) > 1 {
		keyword = args[1]
	}
	if removeTagKeyword != "" {
		if keyword != "" {
			return fmt.Errorf("cannot specify keyword both as argument and flag")
		}
		keyword = removeTagKeyword
	}

	// Validate arguments
	if tagToRemove == "" {
		return fmt.Errorf("tag to remove cannot be empty")
	}

	if removeTagID > 0 && keyword != "" {
		return fmt.Errorf("cannot specify both --id and keyword")
	}

	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Entries) == 0 {
		fmt.Println("No entries found")
		return nil
	}

	// Track changes
	entriesModified := 0
	totalOccurrences := 0

	// Process entries based on the specified criteria
	for i := range cfg.Entries {
		entry := &cfg.Entries[i]

		// Skip stashed entries
		if entry.Stashed {
			continue
		}

		// Check if this entry should be processed
		shouldProcess := false

		if removeTagID > 0 {
			// Only process the specific entry with this ID
			shouldProcess = (entry.ShortID == removeTagID)
		} else if keyword != "" {
			// Only process entries with the specified keyword
			shouldProcess = (entry.Keyword == keyword)
		} else {
			// Process all entries (no filter)
			shouldProcess = true
		}

		if !shouldProcess {
			continue
		}

		// Look for and remove the tag from this entry
		modified := false
		newTags := make([]string, 0, len(entry.Tags))

		for _, tag := range entry.Tags {
			if tag == tagToRemove {
				modified = true
				totalOccurrences++
			} else {
				newTags = append(newTags, tag)
			}
		}

		if modified {
			entry.Tags = newTags
			entriesModified++
		}
	}

	// Handle case where specific entry ID was not found
	if removeTagID > 0 && entriesModified == 0 && totalOccurrences == 0 {
		return fmt.Errorf("no entry found with short ID %d", removeTagID)
	}

	// Save changes if any were made
	if entriesModified > 0 {
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Successfully removed tag '%s'\n", tagToRemove)

		if removeTagID > 0 {
			fmt.Printf("Removed from entry ID %d (%d occurrences)\n", removeTagID, totalOccurrences)
		} else if keyword != "" {
			fmt.Printf("Removed from %d entries with keyword '%s' (%d total occurrences)\n", entriesModified, keyword, totalOccurrences)
		} else {
			fmt.Printf("Removed from %d entries (%d total occurrences)\n", entriesModified, totalOccurrences)
		}

		if IsVerbose() {
			fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
		}
	} else {
		if removeTagID > 0 {
			fmt.Printf("Entry ID %d does not have tag '%s'\n", removeTagID, tagToRemove)
		} else if keyword != "" {
			fmt.Printf("No entries with keyword '%s' have tag '%s'\n", keyword, tagToRemove)
		} else {
			fmt.Printf("No entries found with tag '%s'\n", tagToRemove)
		}
	}

	return nil
}
