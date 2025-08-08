package cmd

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestRunInteractiveFieldEditorFlow(t *testing.T) {
	// This test reproduces the exact flow from runInteractiveFieldEditor
	// to identify where the field preservation issue occurs

	// Create a test entry (simulates what gt set would find)
	originalEntry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original_keyword",
		Tags:      []string{"original_tag"},
		StartTime: time.Date(2025, 8, 8, 14, 0, 0, 0, time.Local),
		Duration:  3600, // 1 hour
		Active:    false,
	}

	t.Log("=== ORIGINAL ENTRY STATE ===")
	t.Logf("Address: %p", originalEntry)
	t.Logf("Keyword: %s", originalEntry.Keyword)
	t.Logf("Tags: %v", originalEntry.Tags)

	// This mimics exactly what runInteractiveFieldEditor does:

	// Step 1: Make a copy of the entry to pass to the editor (line 110)
	entryCopy := *originalEntry

	t.Log("\n=== ENTRY COPY BEFORE TUI ===")
	t.Logf("Copy Address: %p", &entryCopy)
	t.Logf("Copy Keyword: %s", entryCopy.Keyword)
	t.Logf("Copy Tags: %v", entryCopy.Tags)

	// Step 2: Manually modify the copy to simulate what the TUI would do
	// after user switches mode and edits keyword field
	t.Log("\n=== SIMULATING TUI CHANGES ===")
	entryCopy.Keyword = "user_modified_keyword"
	t.Logf("Copy modified - Keyword: %s", entryCopy.Keyword)

	// Step 3: Update the original entry with changes (line 118)
	t.Log("\n=== UPDATING ORIGINAL ENTRY ===")
	t.Logf("Before assignment - Original Keyword: %s", originalEntry.Keyword)
	*originalEntry = entryCopy
	t.Logf("After assignment - Original Keyword: %s", originalEntry.Keyword)

	// Step 4: Verify the changes were preserved
	t.Log("\n=== VERIFICATION ===")
	if originalEntry.Keyword != "user_modified_keyword" {
		t.Errorf("❌ BUG: Assignment failed! Expected 'user_modified_keyword', got '%s'", originalEntry.Keyword)
	} else {
		t.Logf("✅ SUCCESS: Assignment worked correctly: %s", originalEntry.Keyword)
		t.Log("The issue is NOT in the assignment logic in set.go")
		t.Log("The issue must be in the TUI not updating the entry properly")
	}
}

func TestEntryPointerBehavior(t *testing.T) {
	// Test to understand pointer behavior in the field editor

	entry := &models.Entry{
		ID:      uuid.NewString(),
		ShortID: 1,
		Keyword: "test",
		Tags:    []string{"tag1"},
	}

	t.Log("=== TESTING ENTRY POINTER BEHAVIOR ===")
	t.Logf("Original entry address: %p", entry)
	t.Logf("Original entry keyword: %s", entry.Keyword)

	// Test 1: Direct modification
	entry.Keyword = "modified_directly"
	t.Logf("After direct modification: %s", entry.Keyword)

	// Test 2: Copy and assign back
	entryCopy := *entry
	entryCopy.Keyword = "modified_via_copy"
	*entry = entryCopy
	t.Logf("After copy/assign: %s", entry.Keyword)

	// Test 3: What happens when we pass to a function
	modifyEntryInFunction(entry)
	t.Logf("After function modification: %s", entry.Keyword)
}

func modifyEntryInFunction(entry *models.Entry) {
	entry.Keyword = "modified_in_function"
}
