package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestValidationErrorRetry(t *testing.T) {
	// This test reproduces and verifies the fix for the validation error retry bug
	// where errors persist even after user fixes the invalid input

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// First attempt: Submit with empty keyword (should fail)
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("") // Empty keyword - invalid
		case "tags":
			model.fields[i].input.SetValue("test_tag")
		case "duration":
			model.fields[i].input.SetValue("01:00:00")
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	// This should fail with "keyword cannot be empty"
	err := model.validateAndApplyChanges()
	if err == nil {
		t.Fatal("Expected validation error for empty keyword, but got none")
	}

	expectedError := "keyword cannot be empty"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	// Simulate how the Update method would handle this error
	model.err = err

	t.Log("First validation failed as expected:", err.Error())

	// Second attempt: Fix the keyword and submit again (should succeed)
	for i, field := range model.fields {
		if field.name == "keyword" {
			model.fields[i].input.SetValue("valid_keyword") // Now provide valid keyword
		}
	}

	// This should succeed and clear the previous error
	err = model.validateAndApplyChanges()
	if err != nil {
		t.Errorf("Expected validation to succeed after fixing keyword, but got error: %v", err)
	}

	// Verify error is cleared from the model
	if model.err != nil {
		t.Errorf("Expected model.err to be cleared after successful validation, but got: %v", model.err)
	}

	// Verify the entry was updated correctly
	if model.entry.Keyword != "valid_keyword" {
		t.Errorf("Expected keyword to be 'valid_keyword', got '%s'", model.entry.Keyword)
	}

	t.Log("Second validation succeeded after fixing the error")
}

func TestMultipleValidationErrors(t *testing.T) {
	// Test that errors are properly cleared even when there are multiple types of validation errors

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// First attempt: Multiple errors (empty keyword + invalid duration)
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			model.fields[i].input.SetValue("") // Invalid: empty
		case "duration":
			model.fields[i].input.SetValue("invalid_duration") // Invalid format
		case "start_time":
			model.fields[i].input.SetValue("2025-08-08 10:00:00")
		}
	}

	err := model.validateAndApplyChanges()
	if err == nil {
		t.Fatal("Expected validation error, but got none")
	}

	// Should fail on keyword first
	if !strings.Contains(err.Error(), "keyword cannot be empty") {
		t.Errorf("Expected keyword error, got: %s", err.Error())
	}

	t.Log("First validation failed:", err.Error())

	// Second attempt: Fix keyword but duration still invalid
	for i, field := range model.fields {
		if field.name == "keyword" {
			model.fields[i].input.SetValue("valid_keyword")
		}
	}

	err = model.validateAndApplyChanges()
	if err == nil {
		t.Fatal("Expected validation error for invalid duration, but got none")
	}

	// Should now fail on duration format
	if !strings.Contains(err.Error(), "invalid duration format") {
		t.Errorf("Expected duration format error, got: %s", err.Error())
	}

	t.Log("Second validation failed on duration:", err.Error())

	// Third attempt: Fix both errors
	for i, field := range model.fields {
		if field.name == "duration" {
			model.fields[i].input.SetValue("02:00:00") // Valid format
		}
	}

	err = model.validateAndApplyChanges()
	if err != nil {
		t.Errorf("Expected validation to succeed after fixing all errors, but got: %v", err)
	}

	// Verify error is cleared
	if model.err != nil {
		t.Errorf("Expected model.err to be cleared, but got: %v", model.err)
	}

	t.Log("Final validation succeeded after fixing all errors")
}

func TestErrorClearedOnValidInput(t *testing.T) {
	// Test that error state is properly cleared in various scenarios

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	testCases := []struct {
		name          string
		mode          InputMode
		invalidFields map[string]string
		validFields   map[string]string
		expectedError string
	}{
		{
			name: "Duration + Start Time mode",
			mode: ModeDurationStartTime,
			invalidFields: map[string]string{
				"keyword":    "",
				"duration":   "01:00:00",
				"start_time": "2025-08-08 10:00:00",
			},
			validFields: map[string]string{
				"keyword":    "valid_keyword",
				"duration":   "01:00:00",
				"start_time": "2025-08-08 10:00:00",
			},
			expectedError: "keyword cannot be empty",
		},
		{
			name: "Start + End Time mode",
			mode: ModeStartEndTime,
			invalidFields: map[string]string{
				"keyword":    "valid",
				"start_time": "2025-08-08 10:00:00",
				"end_time":   "2025-08-08 08:00:00", // Before start time
			},
			validFields: map[string]string{
				"keyword":    "valid",
				"start_time": "2025-08-08 10:00:00",
				"end_time":   "2025-08-08 12:00:00", // After start time
			},
			expectedError: "end time must be after start time",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := NewFieldEditorModelWithMode(entry, tc.mode)

			// Set invalid fields
			for i, field := range model.fields {
				if value, exists := tc.invalidFields[field.name]; exists {
					model.fields[i].input.SetValue(value)
				}
			}

			// First validation should fail
			err := model.validateAndApplyChanges()
			if err == nil {
				t.Fatal("Expected validation error, but got none")
			}
			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("Expected error containing '%s', got '%s'", tc.expectedError, err.Error())
			}

			// Set valid fields
			for i, field := range model.fields {
				if value, exists := tc.validFields[field.name]; exists {
					model.fields[i].input.SetValue(value)
				}
			}

			// Second validation should succeed
			err = model.validateAndApplyChanges()
			if err != nil {
				t.Errorf("Expected validation to succeed, but got error: %v", err)
			}

			// Error should be cleared
			if model.err != nil {
				t.Errorf("Expected model.err to be cleared, but got: %v", model.err)
			}
		})
	}
}
