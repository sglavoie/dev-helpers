package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/spf13/cobra"
)

var (
	tagsListCount bool
	tagsListUsage bool
)

// tagsListCmd represents the tags list command
var tagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags in use",
	Long: `List all unique tags that are currently being used across all entries.

By default, shows a simple list of all unique tags in alphabetical order.
Use flags to get additional information about tag usage.

Examples:
  gt tags list                     # List all unique tags
  gt tags list --count             # Show tag usage count
  gt tags list --usage             # Show detailed usage per tag`,
	RunE: runTagsList,
}

func init() {
	tagsCmd.AddCommand(tagsListCmd)

	tagsListCmd.Flags().BoolVar(&tagsListCount, "count", false, "show usage count for each tag")
	tagsListCmd.Flags().BoolVar(&tagsListUsage, "usage", false, "show detailed usage information per tag")
}

func runTagsList(cmd *cobra.Command, args []string) error {
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

	// Collect all tags with their usage information
	tagUsage := make(map[string]*TagInfo)

	for _, entry := range cfg.Entries {
		for _, tag := range entry.Tags {
			if tagUsage[tag] == nil {
				tagUsage[tag] = &TagInfo{
					Name:    tag,
					Count:   0,
					Entries: []EntryInfo{},
				}
			}

			tagUsage[tag].Count++
			tagUsage[tag].Entries = append(tagUsage[tag].Entries, EntryInfo{
				ShortID:  entry.ShortID,
				Keyword:  entry.Keyword,
				Active:   entry.Active,
				Duration: entry.GetCurrentDuration(),
			})
		}
	}

	// Check if no tags found
	if len(tagUsage) == 0 {
		fmt.Println("No tags found")
		return nil
	}

	// Sort tags alphabetically
	var tagNames []string
	for tagName := range tagUsage {
		tagNames = append(tagNames, tagName)
	}
	sort.Strings(tagNames)

	// Output based on requested format
	if tagsListUsage {
		return outputDetailedUsage(tagNames, tagUsage)
	} else if tagsListCount {
		return outputWithCount(tagNames, tagUsage)
	} else {
		return outputSimpleList(tagNames, tagUsage)
	}
}

// TagInfo holds information about a tag's usage
type TagInfo struct {
	Name    string
	Count   int
	Entries []EntryInfo
}

// EntryInfo holds basic information about an entry using a tag
type EntryInfo struct {
	ShortID  int
	Keyword  string
	Active   bool
	Duration int
}

func outputSimpleList(tagNames []string, _ map[string]*TagInfo) error {
	fmt.Printf("Found %d unique tags:\n\n", len(tagNames))

	for _, tagName := range tagNames {
		fmt.Printf("  %s\n", tagName)
	}

	return nil
}

func outputWithCount(tagNames []string, tagUsage map[string]*TagInfo) error {
	fmt.Printf("Found %d unique tags:\n\n", len(tagNames))

	// Calculate the maximum tag name length for alignment
	maxLen := 0
	for _, tagName := range tagNames {
		if len(tagName) > maxLen {
			maxLen = len(tagName)
		}
	}

	for _, tagName := range tagNames {
		info := tagUsage[tagName]
		padding := strings.Repeat(" ", maxLen-len(tagName))

		pluralEntries := "entries"
		if info.Count == 1 {
			pluralEntries = "entry"
		}

		fmt.Printf("  %s%s  (%d %s)\n", tagName, padding, info.Count, pluralEntries)
	}

	return nil
}

func outputDetailedUsage(tagNames []string, tagUsage map[string]*TagInfo) error {
	fmt.Printf("Found %d unique tags:\n\n", len(tagNames))

	for i, tagName := range tagNames {
		info := tagUsage[tagName]

		pluralEntries := "entries"
		if info.Count == 1 {
			pluralEntries = "entry"
		}

		fmt.Printf("📝 %s (%d %s):\n", tagName, info.Count, pluralEntries)

		// Sort entries by ShortID for consistent output
		sort.Slice(info.Entries, func(a, b int) bool {
			return info.Entries[a].ShortID < info.Entries[b].ShortID
		})

		for _, entryInfo := range info.Entries {
			status := "⏹️ "
			duration := formatDuration(entryInfo.Duration)

			if entryInfo.Active {
				status = "🟢 "
			}

			fmt.Printf("  %sID:%d %s (%s)\n", status, entryInfo.ShortID, entryInfo.Keyword, duration)
		}

		// Add spacing between tags (except for the last one)
		if i < len(tagNames)-1 {
			fmt.Println()
		}
	}

	return nil
}
