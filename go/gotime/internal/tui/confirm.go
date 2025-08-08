package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmModel represents the confirmation dialog TUI
type ConfirmModel struct {
	message   string
	choices   []string
	selected  int
	done      bool
	confirmed bool
	err       error
}

// NewConfirmModel creates a new confirmation model
func NewConfirmModel(message string) ConfirmModel {
	return ConfirmModel{
		message:  message,
		choices:  []string{"Yes", "No"},
		selected: 1, // Default to "No" for safety
	}
}

func (m ConfirmModel) Init() tea.Cmd {
	return nil
}

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			m.confirmed = false
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit

		case "enter":
			m.done = true
			m.confirmed = (m.selected == 0) // Yes is at index 0
			return m, tea.Quit

		case "left", "h":
			if m.selected > 0 {
				m.selected--
			}

		case "right", "l":
			if m.selected < len(m.choices)-1 {
				m.selected++
			}

		case "y", "Y":
			m.selected = 0 // Yes
			m.done = true
			m.confirmed = true
			return m, tea.Quit

		case "n", "N":
			m.selected = 1 // No
			m.done = true
			m.confirmed = false
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m ConfirmModel) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")). // Red color for warning
		MarginBottom(1)

	b.WriteString(titleStyle.Render("⚠️  Confirmation Required"))
	b.WriteString("\n\n")

	// Message
	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		MarginBottom(2)

	b.WriteString(messageStyle.Render(m.message))
	b.WriteString("\n\n")

	// Choices
	choicesStyle := lipgloss.NewStyle().
		MarginBottom(1)

	var choiceStrings []string
	for i, choice := range m.choices {
		style := lipgloss.NewStyle().
			Padding(0, 2).
			Border(lipgloss.RoundedBorder())

		if i == m.selected {
			if choice == "Yes" {
				style = style.
					BorderForeground(lipgloss.Color("46")). // Green
					Foreground(lipgloss.Color("46")).
					Bold(true)
			} else {
				style = style.
					BorderForeground(lipgloss.Color("196")). // Red
					Foreground(lipgloss.Color("196")).
					Bold(true)
			}
		} else {
			style = style.
				BorderForeground(lipgloss.Color("241")). // Gray
				Foreground(lipgloss.Color("241"))
		}

		choiceStrings = append(choiceStrings, style.Render(choice))
	}

	b.WriteString(choicesStyle.Render(lipgloss.JoinHorizontal(lipgloss.Center, choiceStrings...)))
	b.WriteString("\n\n")

	// Instructions
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	b.WriteString(helpStyle.Render("←/h → /l: Navigate • Enter: Confirm • Y/N: Quick choice • Esc: Cancel"))

	return b.String()
}

// IsDone returns whether the user has finished with the confirmation
func (m ConfirmModel) IsDone() bool {
	return m.done
}

// IsConfirmed returns whether the user confirmed the action
func (m ConfirmModel) IsConfirmed() bool {
	return m.confirmed
}

// GetError returns any error that occurred
func (m ConfirmModel) GetError() error {
	return m.err
}

// RunConfirm runs the confirmation TUI and returns whether the action was confirmed
func RunConfirm(message string) (bool, error) {
	model := NewConfirmModel(message)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("failed to run confirmation TUI: %w", err)
	}

	confirmModel := finalModel.(ConfirmModel)
	if confirmModel.GetError() != nil {
		return false, confirmModel.GetError()
	}

	return confirmModel.IsConfirmed(), nil
}
