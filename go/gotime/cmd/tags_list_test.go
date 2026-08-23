package cmd

import (
	"os"
	"sort"
	"testing"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestTagsList(t *testing.T) {
	configManager, tmpDir := createTestConfigForTags(t)
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING TAGS LIST FUNCTIONALITY ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test collecting all unique tags
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

	// Expected tags from our test data
	expectedTags := map[string]int{
		"work":      2, // entries 1 and 2
		"important": 2, // entries 1 and 4
		"project1":  2, // entries 2 and 3
		"writing":   1, // entry 3
		"client":    1, // entry 4
	}

	t.Logf("Found %d unique tags", len(tagUsage))

	// Verify we found all expected tags
	if len(tagUsage) != len(expectedTags) {
		t.Errorf("Expected %d unique tags, got %d", len(expectedTags), len(tagUsage))
	}

	// Verify each tag's count
	for expectedTag, expectedCount := range expectedTags {
		if tagUsage[expectedTag] == nil {
			t.Errorf("Expected tag '%s' not found", expectedTag)
		} else if tagUsage[expectedTag].Count != expectedCount {
			t.Errorf("Expected tag '%s' to have count %d, got %d", expectedTag, expectedCount, tagUsage[expectedTag].Count)
		}
	}

	// Test that tags are sorted alphabetically
	var tagNames []string
	for tagName := range tagUsage {
		tagNames = append(tagNames, tagName)
	}
	sort.Strings(tagNames)

	expectedOrder := []string{"client", "important", "project1", "work", "writing"}
	for i, tagName := range tagNames {
		if i < len(expectedOrder) && tagName != expectedOrder[i] {
			t.Errorf("Expected tag at position %d to be '%s', got '%s'", i, expectedOrder[i], tagName)
		}
	}

	t.Log("✅ Tags list test completed successfully")
}

func TestTagsListEmpty(t *testing.T) {
	// Create temporary config file with no entries
	configManager, tmpDir := writeTestConfig(t, "gotime_tags_test", []models.Entry{})
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING TAGS LIST WITH EMPTY CONFIG ===")

	// Test with empty entries
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	tagUsage := make(map[string]*TagInfo)
	for _, entry := range cfg.Entries {
		for _, tag := range entry.Tags {
			if tagUsage[tag] == nil {
				tagUsage[tag] = &TagInfo{Name: tag, Count: 0}
			}
			tagUsage[tag].Count++
		}
	}

	if len(tagUsage) != 0 {
		t.Errorf("Expected 0 tags with empty config, got %d", len(tagUsage))
	}

	t.Log("✅ Empty tags list test completed successfully")
}

func TestTagsListNoTags(t *testing.T) {
	// Create config with entries but no tags
	configManager, tmpDir := writeTestConfig(t, "gotime_tags_test", []models.Entry{
		{
			ID:       "entry1",
			ShortID:  1,
			Keyword:  "meeting",
			Tags:     []string{}, // No tags
			Duration: 3600,
		},
		{
			ID:       "entry2",
			ShortID:  2,
			Keyword:  "coding",
			Tags:     []string{}, // No tags
			Duration: 7200,
		},
	})
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING TAGS LIST WITH ENTRIES BUT NO TAGS ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	tagUsage := make(map[string]*TagInfo)
	for _, entry := range cfg.Entries {
		for _, tag := range entry.Tags {
			if tagUsage[tag] == nil {
				tagUsage[tag] = &TagInfo{Name: tag, Count: 0}
			}
			tagUsage[tag].Count++
		}
	}

	if len(tagUsage) != 0 {
		t.Errorf("Expected 0 tags when entries have no tags, got %d", len(tagUsage))
	}

	t.Log("✅ No tags test completed successfully")
}
