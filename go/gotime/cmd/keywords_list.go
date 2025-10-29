package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/spf13/cobra"
)

var (
	keywordsListCount bool
	keywordsListUsage bool
	keywordsListJSON  bool
)

// keywordsListCmd represents the keywords list command
var keywordsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all keywords in use",
	Long: `List all unique keywords that are currently being used across all entries.

By default, shows a simple list of all unique keywords in alphabetical order.
Use flags to get additional information about keyword usage.

Examples:
  gt keywords list                     # List all unique keywords
  gt keywords list --json              # List keywords as JSON (for programmatic use)
  gt keywords list --count             # Show keyword usage count
  gt keywords list --usage             # Show detailed usage per keyword`,
	RunE: runKeywordsList,
}

func init() {
	keywordsCmd.AddCommand(keywordsListCmd)

	keywordsListCmd.Flags().BoolVar(&keywordsListCount, "count", false, "show usage count for each keyword")
	keywordsListCmd.Flags().BoolVar(&keywordsListUsage, "usage", false, "show detailed usage information per keyword")
	keywordsListCmd.Flags().BoolVar(&keywordsListJSON, "json", false, "output keywords as JSON")
}

func runKeywordsList(cmd *cobra.Command, args []string) error {
	// Load configuration
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Entries) == 0 {
		if keywordsListJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No entries found")
		}
		return nil
	}

	// Collect all keywords with their usage information
	keywordUsage := make(map[string]*KeywordInfo)

	for _, entry := range cfg.GetNonStashedEntries() {
		if keywordUsage[entry.Keyword] == nil {
			keywordUsage[entry.Keyword] = &KeywordInfo{
				Name:    entry.Keyword,
				Count:   0,
				Entries: []EntryInfo{},
			}
		}

		keywordUsage[entry.Keyword].Count++
		duration := entry.GetCurrentDuration()
		keywordUsage[entry.Keyword].TotalDuration += duration
		keywordUsage[entry.Keyword].Entries = append(keywordUsage[entry.Keyword].Entries, EntryInfo{
			ShortID:  entry.ShortID,
			Keyword:  entry.Keyword,
			Active:   entry.Active,
			Duration: duration,
		})
	}

	// Check if no keywords found
	if len(keywordUsage) == 0 {
		if keywordsListJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No keywords found")
		}
		return nil
	}

	// Sort keywords alphabetically
	var keywordNames []string
	for keywordName := range keywordUsage {
		keywordNames = append(keywordNames, keywordName)
	}
	sort.Strings(keywordNames)

	// Output based on requested format
	if keywordsListJSON {
		return outputKeywordsJSON(keywordNames, keywordUsage)
	} else if keywordsListUsage {
		return outputDetailedKeywordUsage(keywordNames, keywordUsage)
	} else if keywordsListCount {
		return outputKeywordsWithCount(keywordNames, keywordUsage)
	} else {
		return outputSimpleKeywordList(keywordNames, keywordUsage)
	}
}

// KeywordInfo holds information about a keyword's usage
type KeywordInfo struct {
	Name          string
	Count         int
	TotalDuration int
	Entries       []EntryInfo
}

// KeywordJSON represents keyword data for JSON output
type KeywordJSON struct {
	Keyword       string `json:"keyword"`
	Entries       int    `json:"entries"`
	TotalDuration int    `json:"total_duration"`
}

func outputSimpleKeywordList(keywordNames []string, _ map[string]*KeywordInfo) error {
	fmt.Printf("Found %d unique keywords:\n\n", len(keywordNames))

	for _, keywordName := range keywordNames {
		fmt.Printf("  %s\n", keywordName)
	}

	return nil
}

func outputKeywordsWithCount(keywordNames []string, keywordUsage map[string]*KeywordInfo) error {
	fmt.Printf("Found %d unique keywords:\n\n", len(keywordNames))

	// Calculate the maximum keyword name length for alignment
	maxLen := 0
	for _, keywordName := range keywordNames {
		if len(keywordName) > maxLen {
			maxLen = len(keywordName)
		}
	}

	for _, keywordName := range keywordNames {
		info := keywordUsage[keywordName]
		padding := strings.Repeat(" ", maxLen-len(keywordName))

		pluralEntries := "entries"
		if info.Count == 1 {
			pluralEntries = "entry"
		}

		fmt.Printf("  %s%s  (%d %s, %s)\n", keywordName, padding, info.Count, pluralEntries, formatDuration(info.TotalDuration))
	}

	return nil
}

func outputDetailedKeywordUsage(keywordNames []string, keywordUsage map[string]*KeywordInfo) error {
	fmt.Printf("Found %d unique keywords:\n\n", len(keywordNames))

	for i, keywordName := range keywordNames {
		info := keywordUsage[keywordName]

		pluralEntries := "entries"
		if info.Count == 1 {
			pluralEntries = "entry"
		}

		fmt.Printf("📝 %s (%d %s, %s total):\n", keywordName, info.Count, pluralEntries, formatDuration(info.TotalDuration))

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

			fmt.Printf("  %sID:%d (%s)\n", status, entryInfo.ShortID, duration)
		}

		// Add spacing between keywords (except for the last one)
		if i < len(keywordNames)-1 {
			fmt.Println()
		}
	}

	return nil
}

func outputKeywordsJSON(keywordNames []string, keywordUsage map[string]*KeywordInfo) error {
	keywords := make([]KeywordJSON, 0, len(keywordNames))

	for _, keywordName := range keywordNames {
		info := keywordUsage[keywordName]
		keywords = append(keywords, KeywordJSON{
			Keyword:       keywordName,
			Entries:       info.Count,
			TotalDuration: info.TotalDuration,
		})
	}

	jsonData, err := json.MarshalIndent(keywords, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keywords to JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}
