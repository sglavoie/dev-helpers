package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestInteractiveContinueFiltersActiveKeywords(t *testing.T) {
	configManager, tmpDir := createTestConfigForContinue(t)
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING INTERACTIVE CONTINUE FILTERS ACTIVE KEYWORDS ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Start an active timer for "coding"
	shortID := getNextShortID(cfg)
	activeEntry := models.NewEntry("coding", []string{"project1"}, shortID)
	cfg.AddEntry(activeEntry)

	if err := configManager.Save(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	t.Logf("Created active entry for 'coding' with ID %d", activeEntry.ShortID)

	// Now test the interactive continue filtering logic
	// Simulate the runInteractiveContinue logic
	oneMonthAgo := activeEntry.StartTime.AddDate(0, -1, 0) // Use recent time for test
	keywordEntries := make(map[string]*models.Entry)

	// Find the most recent entry for each keyword within the last month
	// but exclude keywords that already have active timers
	for i := range cfg.Entries {
		entry := &cfg.Entries[i]
		if entry.StartTime.After(oneMonthAgo) && !entry.Active {
			// Skip this keyword if there's already an active timer for it
			if cfg.HasActiveEntryForKeyword(entry.Keyword) {
				t.Logf("Filtering out keyword '%s' - has active timer", entry.Keyword)
				continue
			}

			if existing, exists := keywordEntries[entry.Keyword]; !exists || entry.StartTime.After(existing.StartTime) {
				keywordEntries[entry.Keyword] = entry
			}
		}
	}

	t.Logf("Keywords available for continuation: %d", len(keywordEntries))

	// Verify that "coding" is not in the available keywords (it has an active timer we just created)
	if _, exists := keywordEntries["coding"]; exists {
		t.Errorf("Expected 'coding' to be filtered out, but it's still available")
	}

	// Verify that "meeting" is not in the available keywords (it was already active in test data)
	if _, exists := keywordEntries["meeting"]; exists {
		t.Errorf("Expected 'meeting' to be filtered out, but it's still available")
	}

	// Verify that other keywords are still available
	expectedAvailableKeywords := []string{"documentation"} // Only this should be available

	if len(keywordEntries) != len(expectedAvailableKeywords) {
		t.Errorf("Expected %d available keywords, got %d", len(expectedAvailableKeywords), len(keywordEntries))
	}

	for _, expectedKeyword := range expectedAvailableKeywords {
		if _, exists := keywordEntries[expectedKeyword]; !exists {
			t.Errorf("Expected keyword '%s' to be available, but it's not", expectedKeyword)
		}
	}

	// Verify that if we try to continue "documentation" it should work
	if len(keywordEntries) > 0 {
		for keyword := range keywordEntries {
			if cfg.HasActiveEntryForKeyword(keyword) {
				t.Errorf("Keyword '%s' is available but has active timer - should have been filtered", keyword)
			}
		}
	}

	t.Log("✅ Interactive continue filtering test completed successfully")
}

func TestInteractiveContinueAllKeywordsActive(t *testing.T) {
	configManager, tmpDir := createTestConfigForContinue(t)
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING INTERACTIVE CONTINUE WHEN ALL KEYWORDS HAVE ACTIVE TIMERS ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Start active timers for all unique keywords
	uniqueKeywords := make(map[string]bool)
	for _, entry := range cfg.Entries {
		uniqueKeywords[entry.Keyword] = true
	}

	for keyword := range uniqueKeywords {
		shortID := getNextShortID(cfg)
		activeEntry := models.NewEntry(keyword, []string{"test"}, shortID)
		cfg.AddEntry(activeEntry)
		t.Logf("Created active entry for '%s' with ID %d", keyword, activeEntry.ShortID)
	}

	if err := configManager.Save(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Now test the interactive continue filtering logic
	// Simulate the runInteractiveContinue logic
	oneMonthAgo := cfg.Entries[0].StartTime.AddDate(0, -1, 0) // Use recent time for test
	keywordEntries := make(map[string]*models.Entry)

	// Find the most recent entry for each keyword within the last month
	// but exclude keywords that already have active timers
	for i := range cfg.Entries {
		entry := &cfg.Entries[i]
		if entry.StartTime.After(oneMonthAgo) && !entry.Active {
			// Skip this keyword if there's already an active timer for it
			if cfg.HasActiveEntryForKeyword(entry.Keyword) {
				t.Logf("Filtering out keyword '%s' - has active timer", entry.Keyword)
				continue
			}

			if existing, exists := keywordEntries[entry.Keyword]; !exists || entry.StartTime.After(existing.StartTime) {
				keywordEntries[entry.Keyword] = entry
			}
		}
	}

	t.Logf("Keywords available for continuation: %d", len(keywordEntries))

	// Since all keywords have active timers, no keywords should be available
	if len(keywordEntries) != 0 {
		t.Errorf("Expected 0 available keywords when all have active timers, got %d", len(keywordEntries))
		for keyword := range keywordEntries {
			t.Errorf("Unexpected available keyword: %s", keyword)
		}
	}

	t.Log("✅ Interactive continue all active test completed successfully")
}

func TestMakeUniqueKey(t *testing.T) {
	tests := []struct {
		name     string
		keyword  string
		tags     []string
		expected string
	}{
		{
			name:     "keyword only",
			keyword:  "coding",
			tags:     []string{},
			expected: "coding",
		},
		{
			name:     "keyword with one tag",
			keyword:  "coding",
			tags:     []string{"frontend"},
			expected: "coding|frontend",
		},
		{
			name:     "keyword with multiple tags sorted",
			keyword:  "coding",
			tags:     []string{"backend", "api"},
			expected: "coding|api,backend",
		},
		{
			name:     "keyword with tags in different order should be same",
			keyword:  "coding",
			tags:     []string{"frontend", "backend"},
			expected: "coding|backend,frontend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeUniqueKey(tt.keyword, tt.tags)
			if result != tt.expected {
				t.Errorf("makeUniqueKey(%s, %v) = %s, want %s", tt.keyword, tt.tags, result, tt.expected)
			}
		})
	}

	// Test that same tags in different order produce same key
	t.Run("tags order independence", func(t *testing.T) {
		key1 := makeUniqueKey("coding", []string{"frontend", "backend", "api"})
		key2 := makeUniqueKey("coding", []string{"api", "frontend", "backend"})
		key3 := makeUniqueKey("coding", []string{"backend", "api", "frontend"})

		if key1 != key2 || key2 != key3 {
			t.Errorf("Keys should be identical regardless of tag order: %s, %s, %s", key1, key2, key3)
		}
	})
}

func TestContinueUniqueKeywordTags(t *testing.T) {
	// Create test data with same keywords but different tags
	now := time.Now()
	configManager, tmpDir := writeTestConfig(t, "gotime_unique_test", []models.Entry{
		{
			ID:        "entry1",
			ShortID:   1,
			Keyword:   "coding",
			Tags:      []string{"frontend"},
			Duration:  3600,
			Active:    false,
			StartTime: now.Add(-24 * time.Hour),
		},
		{
			ID:        "entry2",
			ShortID:   2,
			Keyword:   "coding",            // Same keyword
			Tags:      []string{"backend"}, // Different tags
			Duration:  1800,
			Active:    false,
			StartTime: now.Add(-48 * time.Hour),
		},
		{
			ID:        "entry3",
			ShortID:   3,
			Keyword:   "coding",                        // Same keyword again
			Tags:      []string{"frontend", "testing"}, // Different tag combination
			Duration:  2400,
			Active:    false,
			StartTime: now.Add(-72 * time.Hour),
		},
		{
			ID:        "entry4",
			ShortID:   4,
			Keyword:   "documentation",      // Different keyword
			Tags:      []string{"frontend"}, // Same tag as entry1 but different keyword
			Duration:  1200,
			Active:    false,
			StartTime: now.Add(-96 * time.Hour),
		},
	})
	defer os.RemoveAll(tmpDir)

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test the unique entry logic from runInteractiveContinue
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	uniqueEntries := make(map[string]*models.Entry)

	for i := range cfg.Entries {
		entry := &cfg.Entries[i]
		if entry.StartTime.After(oneMonthAgo) && !entry.Active && !entry.Stashed {
			// Skip if there's already an active timer for this keyword
			if cfg.HasActiveEntryForKeyword(entry.Keyword) {
				continue
			}

			uniqueKey := makeUniqueKey(entry.Keyword, entry.Tags)
			if existing, exists := uniqueEntries[uniqueKey]; !exists || entry.StartTime.After(existing.StartTime) {
				uniqueEntries[uniqueKey] = entry
			}
		}
	}

	// Verify we have all unique keyword+tags combinations
	expectedKeys := map[string]bool{
		"coding|frontend":         true, // entry1
		"coding|backend":          true, // entry2
		"coding|frontend,testing": true, // entry3
		"documentation|frontend":  true, // entry4
	}

	if len(uniqueEntries) != len(expectedKeys) {
		t.Errorf("Expected %d unique entries, got %d", len(expectedKeys), len(uniqueEntries))
	}

	for key := range uniqueEntries {
		if !expectedKeys[key] {
			t.Errorf("Unexpected key found: %s", key)
		}
	}

	for expectedKey := range expectedKeys {
		if _, exists := uniqueEntries[expectedKey]; !exists {
			t.Errorf("Expected key not found: %s", expectedKey)
		}
	}

	t.Log("✅ Unique keyword+tags test completed successfully")
}
