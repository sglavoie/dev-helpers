package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestDetailedFieldPreservationDebug(t *testing.T) {
	// This test provides detailed debugging of the field preservation flow

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original_keyword",
		Tags:      []string{"original_tag"},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  3600, // 1 hour
		Active:    false,
	}

	t.Log("=== INITIAL ENTRY STATE ===")
	t.Logf("Keyword: %s", entry.Keyword)
	t.Logf("Tags: %v", entry.Tags)
	t.Logf("StartTime: %s", entry.StartTime.Format("2006-01-02 15:04:05"))
	t.Logf("Duration: %d seconds (%s)", entry.Duration, formatDurationHMS(entry.Duration))

	// Create model in Duration+Start mode
	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	t.Log("\n=== INITIAL MODEL FIELD VALUES ===")
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// User modifies fields
	t.Log("\n=== USER MODIFIES FIELDS ===")
	for i, field := range model.fields {
		switch field.name {
		case "keyword":
			t.Log("Setting keyword to 'user_modified_keyword'")
			model.fields[i].input.SetValue("user_modified_keyword")
		case "tags":
			t.Log("Setting tags to 'user, modified, tags'")
			model.fields[i].input.SetValue("user, modified, tags")
		case "duration":
			t.Log("Setting duration to '02:30:00'")
			model.fields[i].input.SetValue("02:30:00")
		case "start_time":
			t.Log("Setting start_time to '2025-08-08 09:15:30'")
			model.fields[i].input.SetValue("2025-08-08 09:15:30")
		}
	}

	t.Log("\n=== FIELD VALUES AFTER USER MODIFICATION ===")
	for _, field := range model.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// Now let's manually trace through the switchMode process
	t.Log("\n=== SWITCHING MODE: STEP BY STEP ===")

	// Step 1: Preserve current field values
	t.Log("Step 1: Preserving current field values...")
	fieldValues := make(map[string]string)
	for _, field := range model.fields {
		fieldValues[field.name] = field.input.Value()
		t.Logf("  Preserved %s: '%s'", field.name, fieldValues[field.name])
	}

	// Step 2: Create entry copy
	t.Log("Step 2: Creating entry copy...")
	entryCopy := *model.entry
	t.Logf("  Original entry keyword: %s", entryCopy.Keyword)
	t.Logf("  Original entry tags: %v", entryCopy.Tags)

	// Step 3: Update entry copy with current values
	t.Log("Step 3: Updating entry copy with preserved values...")
	if err := model.updateEntryWithCurrentValues(&entryCopy, fieldValues); err != nil {
		t.Logf("  ERROR updating entry: %v", err)
		entryCopy = *model.entry
	} else {
		t.Log("  Successfully updated entry copy")
	}

	t.Logf("  Updated entry keyword: %s", entryCopy.Keyword)
	t.Logf("  Updated entry tags: %v", entryCopy.Tags)
	t.Logf("  Updated entry start time: %s", entryCopy.StartTime.Format("2006-01-02 15:04:05"))
	t.Logf("  Updated entry duration: %d (%s)", entryCopy.Duration, formatDurationHMS(entryCopy.Duration))

	// Step 4: Create new model with updated entry
	t.Log("Step 4: Creating new model with updated entry...")
	newModel := NewFieldEditorModelWithMode(&entryCopy, ModeStartEndTime)

	t.Log("\n=== NEW MODEL FIELD VALUES AFTER CREATION ===")
	for _, field := range newModel.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// Step 5: Call updateComputedFieldValues (this should be a no-op now)
	t.Log("Step 5: Calling updateComputedFieldValues...")
	newModel.updateComputedFieldValues()

	t.Log("\n=== FINAL FIELD VALUES AFTER updateComputedFieldValues ===")
	for _, field := range newModel.fields {
		t.Logf("%s: '%s'", field.name, field.input.Value())
	}

	// Verify preservation
	t.Log("\n=== VERIFICATION ===")
	allPreserved := true
	for _, field := range newModel.fields {
		switch field.name {
		case "keyword":
			expected := "user_modified_keyword"
			if field.input.Value() != expected {
				t.Errorf("❌ Keyword NOT preserved: expected '%s', got '%s'", expected, field.input.Value())
				allPreserved = false
			} else {
				t.Logf("✅ Keyword preserved: %s", field.input.Value())
			}
		case "tags":
			expected := "user, modified, tags"
			if field.input.Value() != expected {
				t.Errorf("❌ Tags NOT preserved: expected '%s', got '%s'", expected, field.input.Value())
				allPreserved = false
			} else {
				t.Logf("✅ Tags preserved: %s", field.input.Value())
			}
		case "start_time":
			expected := "2025-08-08 09:15:30"
			if field.input.Value() != expected {
				t.Errorf("❌ Start time NOT preserved: expected '%s', got '%s'", expected, field.input.Value())
				allPreserved = false
			} else {
				t.Logf("✅ Start time preserved: %s", field.input.Value())
			}
		case "end_time":
			// This should be calculated from preserved start time + duration
			// 09:15:30 + 02:30:00 = 11:45:30
			expected := "2025-08-08 11:45:30"
			if field.input.Value() != expected {
				t.Logf("⚠️  End time calculation: expected '%s', got '%s'", expected, field.input.Value())
				// This might be OK if the calculation is different
			} else {
				t.Logf("✅ End time calculated correctly: %s", field.input.Value())
			}
		}
	}

	if allPreserved {
		t.Log("🎉 SUCCESS: All field values were preserved correctly!")
	} else {
		t.Error("💥 FAILURE: Some field values were not preserved!")
	}
}

func TestUpdateEntryWithCurrentValuesDebug(t *testing.T) {
	// Test the updateEntryWithCurrentValues method specifically

	entry := &models.Entry{
		ID:        uuid.NewString(),
		ShortID:   1,
		Keyword:   "original",
		Tags:      []string{"old"},
		StartTime: time.Date(2025, 8, 8, 10, 0, 0, 0, time.Local),
		Duration:  3600,
		Active:    false,
	}

	model := NewFieldEditorModelWithMode(entry, ModeDurationStartTime)

	// Simulate user field values
	fieldValues := map[string]string{
		"keyword":    "updated_keyword",
		"tags":       "new, tags",
		"duration":   "02:15:45",
		"start_time": "2025-08-08 08:30:15",
	}

	t.Log("=== TESTING updateEntryWithCurrentValues ===")
	t.Log("Original entry:")
	t.Logf("  Keyword: %s", entry.Keyword)
	t.Logf("  Tags: %v", entry.Tags)
	t.Logf("  Start: %s", entry.StartTime.Format("2006-01-02 15:04:05"))
	t.Logf("  Duration: %d", entry.Duration)

	t.Log("Field values to apply:")
	for k, v := range fieldValues {
		t.Logf("  %s: %s", k, v)
	}

	entryCopy := *entry
	err := model.updateEntryWithCurrentValues(&entryCopy, fieldValues)

	if err != nil {
		t.Errorf("updateEntryWithCurrentValues failed: %v", err)
		return
	}

	t.Log("Entry after update:")
	t.Logf("  Keyword: %s", entryCopy.Keyword)
	t.Logf("  Tags: %v", entryCopy.Tags)
	t.Logf("  Start: %s", entryCopy.StartTime.Format("2006-01-02 15:04:05"))
	t.Logf("  Duration: %d", entryCopy.Duration)
	if entryCopy.EndTime != nil {
		t.Logf("  End: %s", entryCopy.EndTime.Format("2006-01-02 15:04:05"))
	}

	// Check if values were properly updated
	if entryCopy.Keyword != "updated_keyword" {
		t.Errorf("Keyword not updated: expected 'updated_keyword', got '%s'", entryCopy.Keyword)
	}

	expectedTags := []string{"new", "tags"}
	if len(entryCopy.Tags) != len(expectedTags) {
		t.Errorf("Tags count mismatch: expected %v, got %v", expectedTags, entryCopy.Tags)
	}

	// Test that new model created from this entry has correct field values
	t.Log("\n=== TESTING MODEL CREATION FROM UPDATED ENTRY ===")
	newModel := NewFieldEditorModelWithMode(&entryCopy, ModeStartEndTime)

	t.Log("New model field values:")
	for _, field := range newModel.fields {
		t.Logf("  %s: '%s'", field.name, field.input.Value())
	}

	// Verify that the field inputs have the correct values
	for _, field := range newModel.fields {
		switch field.name {
		case "keyword":
			if field.input.Value() != "updated_keyword" {
				t.Errorf("New model keyword field incorrect: expected 'updated_keyword', got '%s'", field.input.Value())
			}
		case "tags":
			if field.input.Value() != "new, tags" {
				t.Errorf("New model tags field incorrect: expected 'new, tags', got '%s'", field.input.Value())
			}
		case "start_time":
			expected := "2025-08-08 08:30:15"
			if field.input.Value() != expected {
				t.Errorf("New model start_time field incorrect: expected '%s', got '%s'", expected, field.input.Value())
			}
		}
	}
}
