package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
		keywordField(entry),
		tagsField(entry),
	}

	// Add mode-specific fields
	switch mode {
	case ModeDurationStartTime:
		fields = append(fields,
			durationField(entry, "Total time in HH:MM:SS format (calculates end time)"),
			startTimeField(entry),
		)
	case ModeStartEndTime:
		fields = append(fields,
			startTimeField(entry),
			endTimeField(entry, "When tracking ended (calculates duration)", ""),
		)
	case ModeDurationEndTime:
		fields = append(fields,
			durationField(entry, "Total time in HH:MM:SS format (calculates start time)"),
			// Default to now for new entries
			endTimeField(entry, "When tracking ended (format: "+entryTimeLayout+")", formatEntryTime(time.Now())),
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
