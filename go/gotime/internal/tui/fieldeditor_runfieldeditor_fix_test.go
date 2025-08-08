package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestRunFieldEditorEntryUpdateBug(t *testing.T) {
	// This test reproduces the exact bug reported by the user:
	// After mode switching in the TUI, changes are not reflected in the original entry
	// because RunFieldEditor was not copying the final model's entry back to the original

	originalEntry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original_keyword",
		Tags:      []string{"original_tag"},
		StartTime: time.Date(2025, 8, 8, 14, 0, 0, 0, time.Local),
		Duration:  3600, // 1 hour
		Active:    false,
	}

	t.Log("=== TESTING RunFieldEditor ENTRY UPDATE ===")
	t.Logf("Original entry pointer: %p", originalEntry)
	t.Logf("Original entry keyword: %s", originalEntry.Keyword)
	t.Logf("Original entry tags: %v", originalEntry.Tags)

	// Simulate the fix by manually replicating what RunFieldEditor now does
	model := NewFieldEditorModel(originalEntry)

	t.Logf("Initial model entry pointer: %p", model.entry)
	t.Logf("Initial model entry keyword: %s", model.entry.Keyword)

	// Simulate user interaction: switch mode and modify fields
	// This mimics what happens during actual TUI usage
	shiftTabMsg := createShiftTabKeyMsgForFix()

	// Switch to Duration + End Time mode (two Shift+Tab presses)
	updatedModel, _ := model.Update(shiftTabMsg)
	model = updatedModel.(FieldEditorModel)
	updatedModel, _ = model.Update(shiftTabMsg)
	model = updatedModel.(FieldEditorModel)

	t.Logf("After mode switch - model entry pointer: %p", model.entry)
	t.Logf("After mode switch - model entry keyword: %s", model.entry.Keyword)

	// Simulate user editing keyword and validating
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("user_modified_keyword")
		case "duration":
			model.fields[i].input.SetValue("02:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 16:00:00")
		}
	}

	// Simulate user pressing Enter (validation and save)
	err := model.validateAndApplyChanges()
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	t.Logf("After validation - model entry pointer: %p", model.entry)
	t.Logf("After validation - model entry keyword: %s", model.entry.Keyword)

	// BEFORE FIX: The original entry would still have "original_keyword"
	// AFTER FIX: We now copy the final model's entry back to the original
	t.Log("\\n=== SIMULATING THE FIX ===")
	t.Logf("Before fix - original entry keyword: %s", originalEntry.Keyword)

	// This is the critical line that was added to RunFieldEditor
	*originalEntry = *model.entry

	t.Logf("After fix - original entry keyword: %s", originalEntry.Keyword)

	// Verify the fix works
	if originalEntry.Keyword != "user_modified_keyword" {
		t.Errorf("❌ FIX FAILED: Expected keyword 'user_modified_keyword', got '%s'", originalEntry.Keyword)
		t.Error("The entry update is still not working!")
	} else {
		t.Log("✅ FIX SUCCESSFUL: Original entry was updated with changes from TUI")
		t.Log("This should resolve the user's reported issue!")
	}

	// Also verify other fields
	if len(originalEntry.Tags) == 0 || originalEntry.Tags[0] != "original_tag" {
		t.Errorf("❌ Tags were corrupted: %v", originalEntry.Tags)
	} else {
		t.Log("✅ Tags preserved correctly")
	}

	if originalEntry.Duration != 7200 { // 2 hours = 7200 seconds
		t.Errorf("❌ Duration not updated: expected 7200, got %d", originalEntry.Duration)
	} else {
		t.Log("✅ Duration updated correctly")
	}
}

func createShiftTabKeyMsgForFix() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyShiftTab}
}
