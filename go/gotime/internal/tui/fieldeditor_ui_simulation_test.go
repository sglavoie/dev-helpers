package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestUISimulationErrorRetry(t *testing.T) {
	// This test simulates the actual UI interaction to reproduce the exact bug scenario
	// where the user gets an error, fixes it, and submits again

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Set some valid fields but leave keyword empty
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("") // Empty - will cause error
		case "duration":
			model.fields[i].input.SetValue("01:30:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	// Simulate pressing Enter to submit (first attempt - should fail)
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}

	updatedModel, cmd := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	// Should have an error and not be done
	if model.err == nil {
		t.Fatal("Expected error after first submission with empty keyword, but got none")
	}
	if model.done {
		t.Fatal("Expected model to not be done after error, but it was marked as done")
	}

	expectedError := "keyword cannot be empty"
	if model.err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, model.err.Error())
	}

	t.Log("First submission failed as expected:", model.err.Error())

	// User fixes the keyword
	for i, field := range model.fields {
		if field.name == "keyword" {
			model.fields[i].input.SetValue("fixed_keyword")
			break
		}
	}

	// Simulate pressing Enter again (second attempt - should succeed)
	updatedModel, cmd = model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	// Should have no error and be done
	if model.err != nil {
		t.Errorf("Expected no error after fixing keyword, but got: %v", model.err)
	}
	if !model.done {
		t.Error("Expected model to be done after successful submission, but it wasn't")
	}

	// Verify the entry was updated correctly
	if model.entry.Keyword != "fixed_keyword" {
		t.Errorf("Expected keyword to be 'fixed_keyword', got '%s'", model.entry.Keyword)
	}

	// Verify duration was applied
	expectedDuration := 5400 // 1.5 hours in seconds
	if model.entry.Duration != expectedDuration {
		t.Errorf("Expected duration %d, got %d", expectedDuration, model.entry.Duration)
	}

	// Check that cmd is tea.Quit (indicating completion)
	if cmd == nil {
		t.Error("Expected quit command after successful submission, but got nil")
	}

	t.Log("Second submission succeeded after fixing the error")
}

func TestMultipleErrorRetryCycles(t *testing.T) {
	// Test multiple error-fix cycles to ensure error state is properly managed

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeStartEndTime)

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}

	// Cycle 1: Empty keyword
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 12:00:00")
		}
	}

	updatedModel, _ := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err == nil {
		t.Fatal("Expected error for empty keyword")
	}
	if model.done {
		t.Error("Should not be done with validation error")
	}

	// Fix keyword, but introduce new error (end before start)
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("valid_keyword")
		case "end_time":
			model.fields[i].input.SetValue("2025-08-08 08:00:00") // Before start time
		}
	}

	updatedModel, _ = model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err == nil {
		t.Fatal("Expected error for end time before start time")
	}
	if model.done {
		t.Error("Should not be done with validation error")
	}

	// Fix the end time error
	for i, field := range model.fields {
		if field.name == "end_time" {
			model.fields[i].input.SetValue("2025-08-08 12:00:00") // After start time
		}
	}

	updatedModel, _ = model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	if model.err != nil {
		t.Errorf("Expected no error after fixing all issues, but got: %v", model.err)
	}
	if !model.done {
		t.Error("Should be done after successful validation")
	}

	t.Log("Successfully completed multiple error-fix cycles")
}

func TestErrorDisplayPersistence(t *testing.T) {
	// Test that errors are displayed correctly and cleared when fixed

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Set invalid input
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("")
		case "duration":
			model.fields[i].input.SetValue("01:00:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	// Submit and get error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	// Check that View() shows the error
	view := model.View()
	if model.err != nil && !contains(view, "Error:") {
		t.Error("Expected error to be displayed in view")
	}
	if model.err != nil && !contains(view, "keyword cannot be empty") {
		t.Error("Expected specific error message to be shown in view")
	}

	// Fix the error
	for i, field := range model.fields {
		if field.name == "keyword" {
			model.fields[i].input.SetValue("valid_keyword")
		}
	}

	// Submit again
	updatedModel, _ = model.Update(enterMsg)
	model = updatedModel.(FieldEditorModel)

	// Check that error is cleared from view
	view = model.View()
	if model.err == nil && contains(view, "Error:") {
		t.Error("Error should not be displayed in view after successful validation")
	}

	t.Log("Error display correctly managed during retry cycles")
}

// Helper function to check if a string contains a substring
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		len(needle) > 0 &&
		haystack[len(haystack)-len(needle):] == needle ||
		findSubstring(haystack, needle)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
