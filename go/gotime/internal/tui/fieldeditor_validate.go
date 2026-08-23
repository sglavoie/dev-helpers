package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (m *FieldEditorModel) validateAndApplyChanges() error {
	// Clear any previous error before validating again
	m.err = nil

	// Parse field values into temporary variables
	fieldValues := make(map[string]string)
	for _, field := range m.fields {
		fieldValues[field.name] = strings.TrimSpace(field.input.Value())
	}

	// Validate and apply common fields
	if keyword := fieldValues["keyword"]; keyword != "" {
		// Check if it's a number (reserved for IDs)
		if num, err := strconv.Atoi(keyword); err == nil && num >= 1 && num <= 1_000 {
			return fmt.Errorf("keyword cannot be a number")
		}
		m.entry.Keyword = keyword
	} else {
		return fmt.Errorf("keyword cannot be empty")
	}

	// Handle tags
	m.entry.Tags = parseTagList(fieldValues["tags"])

	// Handle mode-specific validation and calculations
	return m.validateAndCalculateByMode(fieldValues)
}

// validateAndCalculateByMode handles mode-specific validation and calculations
func (m *FieldEditorModel) validateAndCalculateByMode(fieldValues map[string]string) error {
	switch m.inputMode {
	case ModeDurationStartTime:
		return m.validateDurationStartTime(fieldValues)
	case ModeStartEndTime:
		return m.validateStartEndTime(fieldValues)
	case ModeDurationEndTime:
		return m.validateDurationEndTime(fieldValues)
	default:
		return fmt.Errorf("unknown input mode")
	}
}

// validateDurationStartTime handles Duration + Start Time mode
func (m *FieldEditorModel) validateDurationStartTime(fieldValues map[string]string) error {
	duration, err := requirePositiveDuration(fieldValues["duration"])
	if err != nil {
		return err
	}

	startTime, err := requireTime("start", fieldValues["start_time"])
	if err != nil {
		return err
	}

	// Calculate end time and apply to entry as a completed entry
	setCompletedInterval(m.entry, startTime, startTime.Add(time.Duration(duration)*time.Second), duration)

	return nil
}

// validateStartEndTime handles Start Time + End Time mode
func (m *FieldEditorModel) validateStartEndTime(fieldValues map[string]string) error {
	startTime, err := requireTime("start", fieldValues["start_time"])
	if err != nil {
		return err
	}

	// Parse end time (or handle empty for active entry)
	endTimeStr := fieldValues["end_time"]
	if endTimeStr == "" {
		setActiveInterval(m.entry, startTime)
		return nil
	}

	endTime, err := requireTime("end", endTimeStr)
	if err != nil {
		return err
	}

	// Validate that end time is after start time
	if endTime.Before(startTime) || endTime.Equal(startTime) {
		return fmt.Errorf("end time must be after start time")
	}

	// Calculate duration and apply to entry
	duration := int(endTime.Sub(startTime).Seconds())
	setCompletedInterval(m.entry, startTime, endTime, duration)

	return nil
}

// validateDurationEndTime handles Duration + End Time mode
func (m *FieldEditorModel) validateDurationEndTime(fieldValues map[string]string) error {
	duration, err := requirePositiveDuration(fieldValues["duration"])
	if err != nil {
		return err
	}

	endTime, err := requireTime("end", fieldValues["end_time"])
	if err != nil {
		return err
	}

	// Calculate start time (end time minus duration) and apply to entry
	setCompletedInterval(m.entry, endTime.Add(-time.Duration(duration)*time.Second), endTime, duration)

	return nil
}

// requirePositiveDuration reads a duration field, rejecting anything that is
// not a positive HH:MM:SS value.
func requirePositiveDuration(value string) (int, error) {
	duration, err := parseDurationHMS(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format (use HH:MM:SS): %v", err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return duration, nil
}

// requireTime reads a time field, naming it in the error so the user knows
// which of the two bounds was rejected.
func requireTime(name, value string) (time.Time, error) {
	parsed, err := parseEntryTime(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s time format (use YYYY-MM-DD HH:MM:SS): %v", name, err)
	}
	return parsed, nil
}
