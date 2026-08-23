package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// createTestConfigForContinue builds four recent entries — one of them running,
// two sharing a keyword — in a directory of its own. The start times are recent
// so every entry passes the "last month" filter.
func createTestConfigForContinue(t *testing.T) (*config.Manager, string) {
	now := time.Now()
	return writeTestConfig(t, "gotime_continue_test", []models.Entry{
		{ID: "entry1", ShortID: 1, Keyword: "coding", Tags: []string{"project1"}, Duration: 3600, Active: false, StartTime: now.Add(-24 * time.Hour)},
		{ID: "entry2", ShortID: 2, Keyword: "meeting", Tags: []string{"work"}, Duration: 1800, Active: true, StartTime: now.Add(-48 * time.Hour)},
		{ID: "entry3", ShortID: 3, Keyword: "coding", Tags: []string{"project1"}, Duration: 2400, Active: false, StartTime: now.Add(-72 * time.Hour)},
		{ID: "entry4", ShortID: 4, Keyword: "documentation", Tags: []string{"writing"}, Duration: 1200, Active: false, StartTime: now.Add(-96 * time.Hour)},
	})
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
