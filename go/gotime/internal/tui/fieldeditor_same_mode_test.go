package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestSameModeErrorClearing(t *testing.T) {
	// Test that pressing the same function key clears errors
	// e.g., already in Duration+Start mode, user presses F1 again

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "test",
		Tags:      []string{},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Active:    true,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Simulate an error state
	model.err = fmt.Errorf("test validation error")

	if model.err == nil {
		t.Fatal("Error should be present for test setup")
	}

	// Press F1 again (same mode)
	newModel := model.switchMode(ModeDurationStartTime)

	// Error should be cleared even though mode didn't change
	if newModel.err != nil {
		t.Errorf("Error should be cleared when pressing same function key, but got: %v", newModel.err)
	}

	// Mode should remain the same
	if newModel.inputMode != ModeDurationStartTime {
		t.Errorf("Mode should remain ModeDurationStartTime, got %v", newModel.inputMode)
	}

	t.Log("Same-mode key press properly cleared error")
}

func TestAllSameModeKeysCleanError(t *testing.T) {
	// Test that all F1/F2/F3 keys clear errors even when pressed for same mode

	testCases := []struct {
		startMode InputMode
		keyMode   InputMode
		name      string
	}{
		{ModeDurationStartTime, ModeDurationStartTime, "F1 in Duration+Start mode"},
		{ModeStartEndTime, ModeStartEndTime, "F2 in Start+End mode"},
		{ModeDurationEndTime, ModeDurationEndTime, "F3 in Duration+End mode"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := &models.Entry{
				ID:        uuid.NewString(),
				ShortID:   1,
				Keyword:   "test",
				Tags:      []string{},
				StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
				Active:    true,
			}

			model := NewFieldEditorModelWithMode(entry, tc.startMode)
			model.err = fmt.Errorf("test error")

			newModel := model.switchMode(tc.keyMode)

			if newModel.err != nil {
				t.Errorf("Error should be cleared for %s, but got: %v", tc.name, newModel.err)
			}

			if newModel.inputMode != tc.keyMode {
				t.Errorf("Mode should be %v, got %v", tc.keyMode, newModel.inputMode)
			}
		})
	}
}
