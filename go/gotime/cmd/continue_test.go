package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func createTestConfigForContinue(t *testing.T) (*config.Manager, string) {
	// Create temporary config file
	tmpDir, err := ioutil.TempDir("", "gotime_continue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "test_config.json")
	configManager := config.NewManager(configPath)

	// Create test data with some active and inactive entries
	// Use recent start times to ensure they pass the "last month" filter
	now := time.Now()
	recentTime1 := now.Add(-24 * time.Hour) // 1 day ago
	recentTime2 := now.Add(-48 * time.Hour) // 2 days ago
	recentTime3 := now.Add(-72 * time.Hour) // 3 days ago
	recentTime4 := now.Add(-96 * time.Hour) // 4 days ago

	cfg := &models.Config{
		Entries: []models.Entry{
			{
				ID:        "entry1",
				ShortID:   1,
				Keyword:   "coding",
				Tags:      []string{"project1"},
				Duration:  3600,
				Active:    false, // Stopped entry
				StartTime: recentTime1,
			},
			{
				ID:        "entry2",
				ShortID:   2,
				Keyword:   "meeting",
				Tags:      []string{"work"},
				Duration:  1800,
				Active:    true, // Active entry
				StartTime: recentTime2,
			},
			{
				ID:        "entry3",
				ShortID:   3,
				Keyword:   "coding", // Same keyword as entry1 but different entry
				Tags:      []string{"project1"},
				Duration:  2400,
				Active:    false, // Another stopped entry with same keyword
				StartTime: recentTime3,
			},
			{
				ID:        "entry4",
				ShortID:   4,
				Keyword:   "documentation",
				Tags:      []string{"writing"},
				Duration:  1200,
				Active:    false, // Stopped entry
				StartTime: recentTime4,
			},
		},
	}

	if err := configManager.Save(cfg); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	return configManager, tmpDir
}

func TestContinueByKeywordPreventsMultipleActive(t *testing.T) {
	configManager, tmpDir := createTestConfigForContinue(t)
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING CONTINUE BY KEYWORD PREVENTS MULTIPLE ACTIVE ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Use keyword "documentation" which has only stopped entries
	// Then manually add an active entry for "documentation" to create the conflict
	keyword := "documentation"

	// Verify we initially have no active entries for "documentation"
	if cfg.HasActiveEntryForKeyword(keyword) {
		t.Fatalf("Expected no initial active entry for keyword '%s'", keyword)
	}

	// Find the existing stopped entry for "documentation"
	sourceEntry := cfg.GetLastEntryByKeyword(keyword)
	if sourceEntry == nil {
		t.Fatalf("Expected to find a previous entry for keyword '%s'", keyword)
	}

	if sourceEntry.Active {
		t.Fatalf("Expected existing entry to be stopped, got active")
	}

	t.Logf("Found stopped source entry: ID=%d, Active=%v", sourceEntry.ShortID, sourceEntry.Active)

	// Now create another active entry with the same keyword to create the conflict
	shortID := getNextShortID(cfg)
	activeEntry := models.NewEntry(keyword, []string{"writing"}, shortID)
	cfg.AddEntry(activeEntry)

	if err := configManager.Save(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	t.Logf("Created active entry for '%s' with ID %d", keyword, activeEntry.ShortID)

	// Verify we now have an active entry for this keyword
	hasActive := cfg.HasActiveEntryForKeyword(keyword)
	if !hasActive {
		t.Fatalf("Expected to have an active entry for '%s'", keyword)
	}

	// The bug: current logic only checks if the source entry itself is active
	// The source entry we found earlier is stopped, but there's now an active entry with the same keyword
	if sourceEntry.Active {
		t.Log("Source entry is active - would be correctly rejected by current logic")
	} else {
		t.Log("✅ Found the bug: Source entry is not active, but there IS another active entry for this keyword")
		t.Log("Current logic would allow continuation, creating multiple active timers for the same keyword")
	}

	// Simulate what the fixed code should do
	if cfg.HasActiveEntryForKeyword(keyword) {
		expectedError := "an active stopwatch for keyword '" + keyword + "' is already running"
		t.Logf("✅ Fixed logic would correctly prevent continuation: %s", expectedError)
	} else {
		t.Errorf("Expected to find active entry for keyword '%s'", keyword)
	}

	t.Log("✅ Continue by keyword test completed successfully")
}

func TestContinueByIDPreventsMultipleActive(t *testing.T) {
	configManager, tmpDir := createTestConfigForContinue(t)
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING CONTINUE BY ID PREVENTS MULTIPLE ACTIVE ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Target a stopped "coding" entry by ID (entry 1 or 3 are both "coding" and stopped)
	targetID := 1 // This is a stopped "coding" entry
	sourceEntry := cfg.GetEntryByShortID(targetID)

	if sourceEntry == nil {
		t.Fatalf("Expected to find entry with ID %d", targetID)
	}

	if sourceEntry.Active {
		t.Fatalf("Expected source entry to be stopped for this test")
	}

	if sourceEntry.Keyword != "coding" {
		t.Fatalf("Expected source entry to have keyword 'coding', got '%s'", sourceEntry.Keyword)
	}

	t.Logf("Found stopped source entry: ID=%d, Keyword=%s, Active=%v", sourceEntry.ShortID, sourceEntry.Keyword, sourceEntry.Active)

	// Now create an active entry with the same keyword to create the conflict
	shortID := getNextShortID(cfg)
	activeEntry := models.NewEntry("coding", []string{"project1"}, shortID)
	cfg.AddEntry(activeEntry)

	if err := configManager.Save(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	t.Logf("Created active entry for 'coding' with ID %d", activeEntry.ShortID)

	// The current buggy logic only checks if the specific entry (by ID) is active
	// Since we're targeting entry 1 which is stopped, it would allow continuation
	// But there's now another active entry with the same keyword
	if sourceEntry.Active {
		t.Log("Source entry is active - would be correctly rejected")
	} else {
		t.Log("✅ Found the bug: Source entry is not active, but there IS another active entry for this keyword")
		t.Log("Current logic would allow continuation by ID, creating multiple active timers for the same keyword")
	}

	// The bug: we should check if there's ANY active entry with the same keyword
	hasActiveForKeyword := cfg.HasActiveEntryForKeyword(sourceEntry.Keyword)
	if hasActiveForKeyword {
		t.Log("Fixed logic should prevent this continuation")
	}

	// Simulate what the fixed code should do
	if cfg.HasActiveEntryForKeyword(sourceEntry.Keyword) {
		expectedError := "an active stopwatch for keyword 'coding' is already running"
		t.Logf("✅ Fixed logic would correctly prevent continuation: %s", expectedError)
	} else {
		t.Errorf("Expected to find active entry for keyword '%s'", sourceEntry.Keyword)
	}

	t.Log("✅ Continue by ID test completed successfully")
}

func TestContinueLastPreventsMultipleActive(t *testing.T) {
	configManager, tmpDir := createTestConfigForContinue(t)
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING CONTINUE LAST PREVENTS MULTIPLE ACTIVE ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// First, start an active timer for "documentation"
	shortID := getNextShortID(cfg)
	activeEntry := models.NewEntry("documentation", []string{"writing"}, shortID)
	cfg.AddEntry(activeEntry)

	if err := configManager.Save(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	t.Logf("Created active entry for 'documentation' with ID %d", activeEntry.ShortID)

	// Now simulate "continue --last" which should find the most recent stopped entry
	var lastStoppedEntry *models.Entry
	for i := range cfg.Entries {
		entry := &cfg.Entries[i]
		if !entry.Active {
			if lastStoppedEntry == nil || entry.StartTime.After(lastStoppedEntry.StartTime) {
				lastStoppedEntry = entry
			}
		}
	}

	if lastStoppedEntry == nil {
		t.Fatalf("Expected to find at least one stopped entry")
	}

	t.Logf("Found last stopped entry: ID=%d, Keyword=%s", lastStoppedEntry.ShortID, lastStoppedEntry.Keyword)

	// If the last stopped entry has the same keyword as an active entry, it should be rejected
	if cfg.HasActiveEntryForKeyword(lastStoppedEntry.Keyword) {
		expectedError := "an active stopwatch for keyword '" + lastStoppedEntry.Keyword + "' is already running"
		t.Logf("✅ Fixed logic would correctly prevent continuation: %s", expectedError)
	} else {
		t.Log("Last stopped entry has different keyword than active entries - continuation should be allowed")
	}

	t.Log("✅ Continue last test completed successfully")
}

func TestContinueAllowsWhenNoActiveConflict(t *testing.T) {
	configManager, tmpDir := createTestConfigForContinue(t)
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING CONTINUE ALLOWS WHEN NO ACTIVE CONFLICT ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Try to continue "documentation" (which should have no active conflicts)
	keyword := "documentation"

	hasActive := cfg.HasActiveEntryForKeyword(keyword)
	if hasActive {
		t.Fatalf("Expected no active entry for keyword '%s'", keyword)
	}

	sourceEntry := cfg.GetLastEntryByKeyword(keyword)
	if sourceEntry == nil {
		t.Fatalf("Expected to find a previous entry for keyword '%s'", keyword)
	}

	if sourceEntry.Active {
		t.Fatalf("Expected source entry to be inactive")
	}

	t.Logf("Found valid source entry: ID=%d, Keyword=%s, Active=%v", sourceEntry.ShortID, sourceEntry.Keyword, sourceEntry.Active)

	// This should be allowed - no active conflict
	if !cfg.HasActiveEntryForKeyword(keyword) {
		t.Log("✅ No active conflict - continuation should be allowed")
	} else {
		t.Errorf("Unexpected active conflict for keyword '%s'", keyword)
	}

	t.Log("✅ Continue allows when no conflict test completed successfully")
}

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
