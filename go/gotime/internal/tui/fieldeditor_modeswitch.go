package tui

import (
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// switchMode changes the input mode and rebuilds the field layout
func (m FieldEditorModel) switchMode(newMode InputMode) FieldEditorModel {
	if m.inputMode == newMode {
		// Even if mode doesn't change, we should clear any error state
		// This handles the case where user presses same function key after error
		newModel := m
		newModel.err = nil
		return newModel
	}

	// Preserve current field values before switching
	fieldValues := make(map[string]string)
	for _, field := range m.fields {
		fieldValues[field.name] = field.input.Value()
	}

	// Create a copy of the entry to work with
	entryCopy := *m.entry

	// Update the entry copy with current field values (but don't validate completely)
	if err := m.updateEntryWithCurrentValues(&entryCopy, fieldValues); err != nil {
		// If there's an error, just use the original entry
		entryCopy = *m.entry
	}

	// Create new model with the updated entry and new mode
	newModel := NewFieldEditorModelWithMode(&entryCopy, newMode)

	// Calculate and display computed values for the new mode
	newModel.updateComputedFieldValues()

	// Focus the first field that's not keyword or tags (the time/duration fields)
	focusIndex := 0
	for i, field := range newModel.fields {
		if field.name != "keyword" && field.name != "tags" {
			focusIndex = i
			break
		}
	}

	if focusIndex < len(newModel.fields) {
		newModel.fields[focusIndex].input.Focus()
		newModel.focused = focusIndex
	} else {
		newModel.fields[0].input.Focus()
		newModel.focused = 0
	}

	return newModel
}

// cycleMode cycles through input modes in sequence: Duration+Start → Start+End → Duration+End → repeat
func (m FieldEditorModel) cycleMode() FieldEditorModel {
	var nextMode InputMode
	switch m.inputMode {
	case ModeDurationStartTime:
		nextMode = ModeStartEndTime
	case ModeStartEndTime:
		nextMode = ModeDurationEndTime
	case ModeDurationEndTime:
		nextMode = ModeDurationStartTime
	default:
		nextMode = ModeDurationStartTime // Default fallback
	}

	return m.switchMode(nextMode)
}

// updateEntryWithCurrentValues updates the entry with the current field values
// This is used when switching modes to preserve user changes
func (m FieldEditorModel) updateEntryWithCurrentValues(entry *models.Entry, fieldValues map[string]string) error {
	// Update keyword and tags (always safe)
	if keyword := fieldValues["keyword"]; keyword != "" {
		entry.Keyword = keyword
	}

	entry.Tags = parseTagList(fieldValues["tags"])

	// Try to parse and update time/duration fields based on current mode
	switch m.inputMode {
	case ModeDurationStartTime:
		return m.updateFromDurationStartTime(entry, fieldValues)
	case ModeStartEndTime:
		return m.updateFromStartEndTime(entry, fieldValues)
	case ModeDurationEndTime:
		return m.updateFromDurationEndTime(entry, fieldValues)
	}

	return nil
}

// updateFromDurationStartTime updates entry from duration+start_time mode
func (m FieldEditorModel) updateFromDurationStartTime(entry *models.Entry, fieldValues map[string]string) error {
	durationStr := fieldValues["duration"]
	startTimeStr := fieldValues["start_time"]

	if durationStr != "" && startTimeStr != "" {
		duration, err := parseDurationHMS(durationStr)
		if err == nil && duration > 0 {
			startTime, err := parseEntryTime(startTimeStr)
			if err == nil {
				setCompletedInterval(entry, startTime, startTime.Add(time.Duration(duration)*time.Second), duration)
			}
		}
	}
	return nil
}

// updateFromStartEndTime updates entry from start_time+end_time mode
func (m FieldEditorModel) updateFromStartEndTime(entry *models.Entry, fieldValues map[string]string) error {
	startTimeStr := fieldValues["start_time"]
	endTimeStr := fieldValues["end_time"]

	if startTimeStr != "" {
		startTime, err := parseEntryTime(startTimeStr)
		if err == nil {
			entry.StartTime = startTime

			if endTimeStr != "" {
				endTime, err := parseEntryTime(endTimeStr)
				if err == nil && endTime.After(startTime) {
					duration := int(endTime.Sub(startTime).Seconds())
					setCompletedInterval(entry, startTime, endTime, duration)
				}
			} else {
				// Empty end time = active entry
				setActiveInterval(entry, startTime)
			}
		}
	}
	return nil
}

// updateFromDurationEndTime updates entry from duration+end_time mode
func (m FieldEditorModel) updateFromDurationEndTime(entry *models.Entry, fieldValues map[string]string) error {
	durationStr := fieldValues["duration"]
	endTimeStr := fieldValues["end_time"]

	if durationStr != "" && endTimeStr != "" {
		duration, err := parseDurationHMS(durationStr)
		if err == nil && duration > 0 {
			endTime, err := parseEntryTime(endTimeStr)
			if err == nil {
				setCompletedInterval(entry, endTime.Add(-time.Duration(duration)*time.Second), endTime, duration)
			}
		}
	}
	return nil
}

// updateComputedFieldValues calculates and displays ONLY the computed field values for the current mode
// This should NOT overwrite user input fields, only update fields that are calculated from other inputs
func (m *FieldEditorModel) updateComputedFieldValues() {
	// NOTE: The original implementation was wrong - it was overwriting ALL time/duration fields
	// with entry values, which destroyed user input during mode switching.
	//
	// In the current design:
	// - ModeDurationStartTime: end_time is computed but not shown as a field
	// - ModeStartEndTime: duration is computed but not shown as a field
	// - ModeDurationEndTime: start_time is computed but not shown as a field
	//
	// Since computed values are not displayed as editable fields in any mode currently,
	// this method should do nothing and preserve all user input.
	// The actual computation happens during validation when the user submits the form.
}
