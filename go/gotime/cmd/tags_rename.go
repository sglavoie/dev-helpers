package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/spf13/cobra"
)

// tagsRenameCmd represents the tags rename command
var tagsRenameCmd = &cobra.Command{
	Use:   "rename <old-tag> <new-tag>",
	Short: "Rename a tag across all entries",
	Long: `Rename a tag across all time tracking entries.

This command will find all entries that contain the specified old tag
and replace it with the new tag. The operation affects all entries
that have the old tag.

Examples:
  gt tags rename work office        # Rename 'work' tag to 'office' in all entries
  gt tags rename old-project proj1  # Rename 'old-project' to 'proj1'
  gt tags rename "old tag" new-tag  # Rename tag with spaces (use quotes)`,
	Args: cobra.ExactArgs(2),
	RunE: runTagsRename,
}

func init() {
	tagsCmd.AddCommand(tagsRenameCmd)
}

func runTagsRename(cmd *cobra.Command, args []string) error {
	oldTag := args[0]
	newTag := args[1]

	// Validate arguments
	if oldTag == "" {
		return fmt.Errorf("old tag cannot be empty")
	}
	if newTag == "" {
		return fmt.Errorf("new tag cannot be empty")
	}
	if oldTag == newTag {
		return fmt.Errorf("old tag and new tag cannot be the same")
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

	// Process each entry
	for i := range cfg.Entries {
		entry := &cfg.Entries[i]
		modified := false

		// Look for the old tag in this entry
		for j, tag := range entry.Tags {
			if tag == oldTag {
				entry.Tags[j] = newTag
				modified = true
				totalOccurrences++
			}
		}

		if modified {
			entriesModified++
		}
	}

	// Save changes if any were made
	if entriesModified > 0 {
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Successfully renamed tag '%s' to '%s'\n", oldTag, newTag)
		fmt.Printf("Modified %d entries (%d total occurrences)\n", entriesModified, totalOccurrences)

		if IsVerbose() {
			fmt.Printf("Config saved to: %s\n", configManager.GetConfigPath())
		}
	} else {
		fmt.Printf("No entries found with tag '%s'\n", oldTag)
	}

	return nil
}
