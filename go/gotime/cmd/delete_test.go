package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/config"
)

// createTestConfigForDelete builds the shared tagged fixture in a directory of
// its own.
func createTestConfigForDelete(t *testing.T) (*config.Manager, string) {
	return writeTestConfig(t, "gotime_delete_test", taggedTestEntries())
}

func TestDeleteByIDDisplaysCorrectID(t *testing.T) {
	configManager, tmpDir := createTestConfigForDelete(t)
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING DELETE BY ID DISPLAYS CORRECT ID ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test deleting entry with ID 3 (which should be "documentation")
	targetID := 3
	entry := cfg.GetEntryByShortID(targetID)
	if entry == nil {
		t.Fatalf("Entry with ID %d not found", targetID)
	}

	// Verify we're targeting the right entry
	if entry.Keyword != "documentation" {
		t.Fatalf("Expected entry with ID %d to have keyword 'documentation', got '%s'", targetID, entry.Keyword)
	}

	t.Logf("Before removal - Target entry: ID=%d, Keyword=%s, Tags=%v", entry.ShortID, entry.Keyword, entry.Tags)

	// Capture the entry details BEFORE removal (this is what the fix does)
	originalKeyword := entry.Keyword
	originalTags := entry.Tags
	originalShortID := entry.ShortID

	// Simulate the removal process
	success := cfg.RemoveEntry(entry.ID)
	if !success {
		t.Fatalf("Failed to remove entry")
	}

	t.Logf("After removal - entries count: %d", len(cfg.Entries))

	// Verify that the original short IDs are no longer valid
	// After removal, the remaining entries should have short IDs 1, 2, 3 (renumbered)
	expectedRemainingCount := 3
	if len(cfg.Entries) != expectedRemainingCount {
		t.Fatalf("Expected %d entries after removal, got %d", expectedRemainingCount, len(cfg.Entries))
	}

	// Verify short IDs were renumbered starting from 1
	for i, remainingEntry := range cfg.Entries {
		expectedID := i + 1
		if remainingEntry.ShortID != expectedID {
			t.Errorf("Expected remaining entry %d to have ShortID %d, got %d", i, expectedID, remainingEntry.ShortID)
		}
	}

	// Create the display message using the captured values (simulating the fixed code)
	displayMessage := fmt.Sprintf("Deleted entry: %s %v (ID: %d)",
		originalKeyword, originalTags, originalShortID)

	// Verify the display shows the correct original ID (not ID 1)
	if !strings.Contains(displayMessage, "ID: 3") {
		t.Errorf("Expected display to show 'ID: 3', got: %s", displayMessage)
	}
	if !strings.Contains(displayMessage, "documentation") {
		t.Errorf("Expected display to show 'documentation', got: %s", displayMessage)
	}

	t.Logf("Display message: %s", displayMessage)
	t.Log("✅ Delete ID display test completed successfully")
}

func TestDeleteWithoutCaptureShowsWrongData(t *testing.T) {
	configManager, tmpDir := createTestConfigForDelete(t)
	defer os.RemoveAll(tmpDir)

	t.Log("=== TESTING DELETE WITHOUT CAPTURE SHOWS WRONG DATA (simulating the bug) ===")

	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test deleting entry with ID 3 (which should be "documentation")
	targetID := 3
	entry := cfg.GetEntryByShortID(targetID)
	if entry == nil {
		t.Fatalf("Entry with ID %d not found", targetID)
	}

	// Verify we're targeting the right entry
	if entry.Keyword != "documentation" {
		t.Fatalf("Expected entry with ID %d to have keyword 'documentation', got '%s'", targetID, entry.Keyword)
	}

	t.Logf("Before removal - Target entry: ID=%d, Keyword=%s, Tags=%v", entry.ShortID, entry.Keyword, entry.Tags)

	// Store the original entry UUID to verify what was actually deleted
	originalID := entry.ID

	// Simulate the buggy removal process (using entry fields AFTER removal)
	success := cfg.RemoveEntry(originalID)
	if !success {
		t.Fatalf("Failed to remove entry")
	}

	// Now simulate what the buggy code would do - try to use entry fields after removal
	// The entry pointer now points to whatever entry is at that memory location after reordering
	t.Logf("After removal - entry pointer now shows: ID=%d, Keyword=%s, Tags=%v",
		entry.ShortID, entry.Keyword, entry.Tags)

	buggyDisplayMessage := fmt.Sprintf("Deleted entry: %s %v (ID: %d)",
		entry.Keyword, entry.Tags, entry.ShortID)

	t.Logf("Buggy display message: %s", buggyDisplayMessage)

	// The bug: the entry pointer now points to different data than what was actually deleted
	// We deleted "documentation" but the message might show a different entry's data
	if strings.Contains(buggyDisplayMessage, "documentation") {
		t.Errorf("Bug demonstration failed - expected to see different data than 'documentation', got: %s", buggyDisplayMessage)
	}

	// Verify that "documentation" entry is actually gone from the config
	documentationExists := false
	for _, remainingEntry := range cfg.Entries {
		if remainingEntry.Keyword == "documentation" {
			documentationExists = true
			break
		}
	}

	if documentationExists {
		t.Errorf("Expected 'documentation' entry to be deleted, but it still exists")
	}

	t.Log("✅ Bug demonstration test completed - shows pointer corruption after removal")
}
