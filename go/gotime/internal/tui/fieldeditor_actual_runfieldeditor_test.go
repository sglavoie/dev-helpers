package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// TestRunFieldEditorEntryPointerStability tests whether the entry pointer is stable
func TestRunFieldEditorEntryPointerStability(t *testing.T) {
	// Test to understand if the entry pointer changes during RunFieldEditor

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test_keyword",
		Tags:      []string{"tag1"},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  3600,
		Active:    false,
	}

	t.Log("=== TESTING ENTRY POINTER STABILITY ===")
	t.Logf("Original entry pointer: %p", entry)
	t.Logf("Original entry address: %p", &entry)
	t.Logf("Original keyword: %s", entry.Keyword)

	// Test what happens if we pass the entry to a function that modifies it
	modifyEntryDirectly(entry)
	t.Logf("After direct modification: %s", entry.Keyword)

	// Test what happens with copy and reassign
	entryCopy := *entry
	entryCopy.Keyword = "modified_copy"
	*entry = entryCopy
	t.Logf("After copy/reassign: %s", entry.Keyword)
	t.Logf("Entry pointer after reassign: %p", entry)

	// Test with pointer to copy
	testWithPointerCopy(&entry)
	t.Logf("After pointer copy test: %s", entry.Keyword)
}

func modifyEntryDirectly(entry *models.Entry) {
	entry.Keyword = "directly_modified"
}

func testWithPointerCopy(entryPtr **models.Entry) {
	// Create a copy of the entry
	entryCopy := **entryPtr
	entryCopy.Keyword = "pointer_copy_modified"

	// Assign back
	**entryPtr = entryCopy
}
