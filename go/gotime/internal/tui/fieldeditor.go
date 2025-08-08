package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

// InputMode represents different ways to specify time tracking entries
type InputMode int

const (
	ModeDurationStartTime InputMode = iota // Duration + Start Time -> calculates End Time
	ModeStartEndTime                       // Start Time + End Time -> calculates Duration
	ModeDurationEndTime                    // Duration + End Time -> calculates Start Time
)

func (m InputMode) String() string {
	switch m {
	case ModeDurationStartTime:
		return "Duration + Start Time"
	case ModeStartEndTime:
		return "Start Time + End Time"
	case ModeDurationEndTime:
		return "Duration + End Time"
	default:
		return "Unknown"
	}
}

// FieldEditorModel represents the field editing TUI
type FieldEditorModel struct {
	entry     *models.Entry
	fields    []fieldModel
	focused   int
	done      bool
	cancelled bool
	viewport  int // For scrolling through fields
	err       error
	inputMode InputMode
}

type fieldModel struct {
	name        string
	displayName string
	input       textinput.Model
	value       string
	description string
	fieldType   fieldType
}

type fieldType int

const (
	fieldTypeString fieldType = iota
	fieldTypeDuration
	fieldTypeTime
	fieldTypeTags
)

// NewFieldEditorModel creates a new field editor model
func NewFieldEditorModel(entry *models.Entry) FieldEditorModel {
	// Determine the best input mode based on the entry's current state
	inputMode := determineInputMode(entry)
	return NewFieldEditorModelWithMode(entry, inputMode)
}

// NewFieldEditorModelWithMode creates a field editor with a specific input mode
func NewFieldEditorModelWithMode(entry *models.Entry, mode InputMode) FieldEditorModel {
	// Common fields that appear in all modes
	fields := []fieldModel{
		{
			name:        "keyword",
			displayName: "Keyword",
			description: "Primary categorization for this entry",
			fieldType:   fieldTypeString,
			value:       entry.Keyword,
		},
		{
			name:        "tags",
			displayName: "Tags",
			description: "Comma-separated tags for secondary categorization",
			fieldType:   fieldTypeTags,
			value:       strings.Join(entry.Tags, ", "),
		},
	}

	// Add mode-specific fields
	switch mode {
	case ModeDurationStartTime:
		fields = append(fields,
			fieldModel{
				name:        "duration",
				displayName: "Duration",
				description: "Total time in HH:MM:SS format (calculates end time)",
				fieldType:   fieldTypeDuration,
				value:       formatDurationHMS(entry.GetCurrentDuration()),
			},
			fieldModel{
				name:        "start_time",
				displayName: "Start Time",
				description: "When tracking started (format: 2006-01-02 15:04:05)",
				fieldType:   fieldTypeTime,
				value:       entry.StartTime.Format("2006-01-02 15:04:05"),
			},
		)
	case ModeStartEndTime:
		fields = append(fields,
			fieldModel{
				name:        "start_time",
				displayName: "Start Time",
				description: "When tracking started (format: 2006-01-02 15:04:05)",
				fieldType:   fieldTypeTime,
				value:       entry.StartTime.Format("2006-01-02 15:04:05"),
			},
			fieldModel{
				name:        "end_time",
				displayName: "End Time",
				description: "When tracking ended (calculates duration)",
				fieldType:   fieldTypeTime,
				value: func() string {
					if entry.EndTime != nil {
						return entry.EndTime.Format("2006-01-02 15:04:05")
					}
					return ""
				}(),
			},
		)
	case ModeDurationEndTime:
		fields = append(fields,
			fieldModel{
				name:        "duration",
				displayName: "Duration",
				description: "Total time in HH:MM:SS format (calculates start time)",
				fieldType:   fieldTypeDuration,
				value:       formatDurationHMS(entry.GetCurrentDuration()),
			},
			fieldModel{
				name:        "end_time",
				displayName: "End Time",
				description: "When tracking ended (format: 2006-01-02 15:04:05)",
				fieldType:   fieldTypeTime,
				value: func() string {
					if entry.EndTime != nil {
						return entry.EndTime.Format("2006-01-02 15:04:05")
					}
					// Default to now for new entries
					return time.Now().Format("2006-01-02 15:04:05")
				}(),
			},
		)
	}

	// Initialize text inputs for each field
	for i := range fields {
		input := textinput.New()
		input.SetValue(fields[i].value)
		input.Width = 40
		if i == 0 {
			input.Focus()
		}
		fields[i].input = input
	}

	return FieldEditorModel{
		entry:     entry,
		fields:    fields,
		focused:   0,
		inputMode: mode,
	}
}

// determineInputMode selects the best input mode based on the entry's current state
func determineInputMode(entry *models.Entry) InputMode {
	// If it's an active entry, use Duration + Start Time (most common for active tracking)
	if entry.Active {
		return ModeDurationStartTime
	}

	// If it's a completed entry with both start and end times, use Start + End Time
	if entry.EndTime != nil {
		return ModeStartEndTime
	}

	// Default to Duration + Start Time for new entries
	return ModeDurationStartTime
}

func (m FieldEditorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m FieldEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			if err := m.validateAndApplyChanges(); err != nil {
				m.err = err
				return m, nil
			}
			m.done = true
			return m, tea.Quit

		case "tab", "down":
			m.fields[m.focused].input.Blur()
			m.focused = (m.focused + 1) % len(m.fields)
			m.fields[m.focused].input.Focus()
			return m, nil

		case "up":
			m.fields[m.focused].input.Blur()
			m.focused = (m.focused - 1 + len(m.fields)) % len(m.fields)
			m.fields[m.focused].input.Focus()
			return m, nil

		case "shift+tab":
			return m.cycleMode(), nil
		}
	}

	// Handle input updates
	var cmd tea.Cmd
	m.fields[m.focused].input, cmd = m.fields[m.focused].input.Update(msg)

	return m, cmd
}

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

	if tagsStr := fieldValues["tags"]; tagsStr != "" {
		tags := strings.Split(tagsStr, ",")
		cleanTags := []string{}
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				cleanTags = append(cleanTags, tag)
			}
		}
		entry.Tags = cleanTags
	} else {
		entry.Tags = []string{}
	}

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
			startTime, err := time.ParseInLocation("2006-01-02 15:04:05", startTimeStr, time.Local)
			if err == nil {
				endTime := startTime.Add(time.Duration(duration) * time.Second)
				entry.StartTime = startTime
				entry.EndTime = &endTime
				entry.Duration = duration
				entry.Active = false
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
		startTime, err := time.ParseInLocation("2006-01-02 15:04:05", startTimeStr, time.Local)
		if err == nil {
			entry.StartTime = startTime

			if endTimeStr != "" {
				endTime, err := time.ParseInLocation("2006-01-02 15:04:05", endTimeStr, time.Local)
				if err == nil && endTime.After(startTime) {
					duration := int(endTime.Sub(startTime).Seconds())
					entry.EndTime = &endTime
					entry.Duration = duration
					entry.Active = false
				}
			} else {
				// Empty end time = active entry
				entry.EndTime = nil
				entry.Duration = 0
				entry.Active = true
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
			endTime, err := time.ParseInLocation("2006-01-02 15:04:05", endTimeStr, time.Local)
			if err == nil {
				startTime := endTime.Add(-time.Duration(duration) * time.Second)
				entry.StartTime = startTime
				entry.EndTime = &endTime
				entry.Duration = duration
				entry.Active = false
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

func (m FieldEditorModel) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("📝 Edit Entry Fields"))
	b.WriteString("\n")

	// Mode selector - show all modes with current one highlighted
	b.WriteString(m.renderModeSelector())
	b.WriteString("\n")

	// Entry info
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginBottom(1)

	status := "Stopped"
	if m.entry.Active {
		status = "Running"
	}

	b.WriteString(infoStyle.Render(fmt.Sprintf("Entry ID: %d | Status: %s", m.entry.ShortID, status)))
	b.WriteString("\n\n")

	// Fields in a 2-column layout
	leftColumn := []string{}
	rightColumn := []string{}

	for i, field := range m.fields {
		fieldView := m.renderField(field, i == m.focused)
		if i%2 == 0 {
			leftColumn = append(leftColumn, fieldView)
		} else {
			rightColumn = append(rightColumn, fieldView)
		}
	}

	// Balance columns
	for len(leftColumn) < len(rightColumn) {
		leftColumn = append(leftColumn, "")
	}
	for len(rightColumn) < len(leftColumn) {
		rightColumn = append(rightColumn, "")
	}

	// Join columns
	leftContent := strings.Join(leftColumn, "\n\n")
	rightContent := strings.Join(rightColumn, "\n\n")

	columns := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(50).Render(leftContent),
		lipgloss.NewStyle().Width(50).MarginLeft(4).Render(rightContent),
	)

	b.WriteString(columns)
	b.WriteString("\n\n")

	// Instructions
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	b.WriteString(helpStyle.Render("Tab/↑↓: Navigate • Enter: Save Changes • Esc: Cancel"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Shift+Tab: Switch Mode"))

	// Show error if any
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			MarginTop(1)
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.err.Error())))
	}

	return b.String()
}

func (m FieldEditorModel) renderField(field fieldModel, focused bool) string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		MarginTop(1)

	if focused {
		inputStyle = inputStyle.BorderForeground(lipgloss.Color("62"))
	} else {
		inputStyle = inputStyle.BorderForeground(lipgloss.Color("241"))
	}

	b.WriteString(labelStyle.Render(field.displayName))
	b.WriteString("\n")
	b.WriteString(descStyle.Render(field.description))
	b.WriteString("\n")
	b.WriteString(inputStyle.Render(field.input.View()))

	return b.String()
}

func (m FieldEditorModel) renderModeSelector() string {
	// Define all available modes
	allModes := []InputMode{
		ModeDurationStartTime,
		ModeStartEndTime,
		ModeDurationEndTime,
	}

	// Styles for mode display
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Bold(true).
		Padding(0, 1)

	unselectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("33")).
		Bold(true)

	var modeItems []string
	for _, mode := range allModes {
		if mode == m.inputMode {
			modeItems = append(modeItems, selectedStyle.Render(mode.String()))
		} else {
			modeItems = append(modeItems, unselectedStyle.Render(mode.String()))
		}
	}

	// Join modes with separators
	modeDisplay := strings.Join(modeItems, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(" | "))

	return headerStyle.Render("Modes: ") + modeDisplay
}

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
	if tagsStr := fieldValues["tags"]; tagsStr != "" {
		tags := strings.Split(tagsStr, ",")
		cleanTags := []string{}
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				cleanTags = append(cleanTags, tag)
			}
		}
		m.entry.Tags = cleanTags
	} else {
		m.entry.Tags = []string{}
	}

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
	// Parse duration
	duration, err := parseDurationHMS(fieldValues["duration"])
	if err != nil {
		return fmt.Errorf("invalid duration format (use HH:MM:SS): %v", err)
	}
	if duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	// Parse start time
	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", fieldValues["start_time"], time.Local)
	if err != nil {
		return fmt.Errorf("invalid start time format (use YYYY-MM-DD HH:MM:SS): %v", err)
	}

	// Calculate end time
	endTime := startTime.Add(time.Duration(duration) * time.Second)

	// Apply to entry
	m.entry.StartTime = startTime
	m.entry.EndTime = &endTime
	m.entry.Duration = duration
	m.entry.Active = false // Completed entry

	return nil
}

// validateStartEndTime handles Start Time + End Time mode
func (m *FieldEditorModel) validateStartEndTime(fieldValues map[string]string) error {
	// Parse start time
	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", fieldValues["start_time"], time.Local)
	if err != nil {
		return fmt.Errorf("invalid start time format (use YYYY-MM-DD HH:MM:SS): %v", err)
	}

	// Parse end time (or handle empty for active entry)
	endTimeStr := fieldValues["end_time"]
	if endTimeStr == "" {
		// Active entry
		m.entry.StartTime = startTime
		m.entry.EndTime = nil
		m.entry.Duration = 0
		m.entry.Active = true
		return nil
	}

	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", endTimeStr, time.Local)
	if err != nil {
		return fmt.Errorf("invalid end time format (use YYYY-MM-DD HH:MM:SS): %v", err)
	}

	// Validate that end time is after start time
	if endTime.Before(startTime) || endTime.Equal(startTime) {
		return fmt.Errorf("end time must be after start time")
	}

	// Calculate duration
	duration := int(endTime.Sub(startTime).Seconds())

	// Apply to entry
	m.entry.StartTime = startTime
	m.entry.EndTime = &endTime
	m.entry.Duration = duration
	m.entry.Active = false

	return nil
}

// validateDurationEndTime handles Duration + End Time mode
func (m *FieldEditorModel) validateDurationEndTime(fieldValues map[string]string) error {
	// Parse duration
	duration, err := parseDurationHMS(fieldValues["duration"])
	if err != nil {
		return fmt.Errorf("invalid duration format (use HH:MM:SS): %v", err)
	}
	if duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	// Parse end time
	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", fieldValues["end_time"], time.Local)
	if err != nil {
		return fmt.Errorf("invalid end time format (use YYYY-MM-DD HH:MM:SS): %v", err)
	}

	// Calculate start time (end time minus duration)
	startTime := endTime.Add(-time.Duration(duration) * time.Second)

	// Apply to entry
	m.entry.StartTime = startTime
	m.entry.EndTime = &endTime
	m.entry.Duration = duration
	m.entry.Active = false

	return nil
}

// Helper functions
func formatDurationHMS(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}

func parseDurationHMS(duration string) (int, error) {
	parts := strings.Split(duration, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("duration must be in HH:MM:SS format")
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 {
		return 0, fmt.Errorf("invalid hours")
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes >= 60 {
		return 0, fmt.Errorf("invalid minutes (0-59)")
	}

	seconds, err := strconv.Atoi(parts[2])
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, fmt.Errorf("invalid seconds (0-59)")
	}

	return hours*3600 + minutes*60 + seconds, nil
}

// IsDone returns whether the user has finished editing
func (m FieldEditorModel) IsDone() bool {
	return m.done
}

// IsCancelled returns whether the user cancelled the editing
func (m FieldEditorModel) IsCancelled() bool {
	return m.cancelled
}

// GetError returns any error that occurred
func (m FieldEditorModel) GetError() error {
	return m.err
}

// RunFieldEditor runs the field editor TUI
func RunFieldEditor(entry *models.Entry) error {
	model := NewFieldEditorModel(entry)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run field editor TUI: %w", err)
	}

	fieldEditorModel := finalModel.(FieldEditorModel)
	if fieldEditorModel.GetError() != nil {
		return fieldEditorModel.GetError()
	}

	if fieldEditorModel.IsCancelled() {
		return fmt.Errorf("editing cancelled")
	}

	// CRITICAL FIX: Apply changes from the final model back to the original entry
	// This is necessary because mode switching creates new models with entry copies,
	// so the finalModel.entry may point to a copy, not the original entry
	*entry = *fieldEditorModel.entry

	return nil
}
