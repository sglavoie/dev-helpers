package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StartInputModel represents the start input TUI
type StartInputModel struct {
	keywordInput textinput.Model
	tagsInput    textinput.Model
	focused      int
	done         bool
	cancelled    bool
	keyword      string
	tags         []string
	err          error
}

// NewStartInputModel creates a new start input model
func NewStartInputModel() StartInputModel {
	keywordInput := textinput.New()
	keywordInput.Placeholder = "Enter keyword (e.g., coding, meeting, docs)"
	keywordInput.Focus()
	keywordInput.CharLimit = 50
	keywordInput.Width = 50

	tagsInput := textinput.New()
	tagsInput.Placeholder = "Enter tags separated by spaces (optional)"
	tagsInput.CharLimit = 200
	tagsInput.Width = 50

	return StartInputModel{
		keywordInput: keywordInput,
		tagsInput:    tagsInput,
		focused:      0,
	}
}

func (m StartInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m StartInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			m.cancelled = true
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit

		case "enter":
			if m.focused == 0 {
				// Move to tags input
				m.keywordInput.Blur()
				m.tagsInput.Focus()
				m.focused = 1
				return m, textinput.Blink
			} else {
				// Submit form
				keyword := strings.TrimSpace(m.keywordInput.Value())
				if keyword == "" {
					return m, nil
				}

				m.keyword = keyword

				// Parse tags
				tagsStr := strings.TrimSpace(m.tagsInput.Value())
				if tagsStr != "" {
					m.tags = strings.Fields(tagsStr)
				}

				m.done = true
				return m, tea.Quit
			}

		case "tab", "shift+tab", "up", "down":
			// Toggle between inputs
			if m.focused == 0 {
				m.keywordInput.Blur()
				m.tagsInput.Focus()
				m.focused = 1
			} else {
				m.tagsInput.Blur()
				m.keywordInput.Focus()
				m.focused = 0
			}
			return m, textinput.Blink
		}
	}

	// Update the focused input
	if m.focused == 0 {
		m.keywordInput, cmd = m.keywordInput.Update(msg)
	} else {
		m.tagsInput, cmd = m.tagsInput.Update(msg)
	}

	return m, cmd
}

func (m StartInputModel) View() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Start New Time Tracking"))
	b.WriteString("\n\n")

	// Keyword field
	keywordStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	if m.focused == 0 {
		keywordStyle = keywordStyle.Bold(true).Foreground(lipgloss.Color("39"))
	}

	b.WriteString(keywordStyle.Render("Keyword:"))
	b.WriteString("\n")
	b.WriteString(m.keywordInput.View())
	b.WriteString("\n\n")

	// Tags field
	tagsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	if m.focused == 1 {
		tagsStyle = tagsStyle.Bold(true).Foreground(lipgloss.Color("39"))
	}

	b.WriteString(tagsStyle.Render("Tags:"))
	b.WriteString("\n")
	b.WriteString(m.tagsInput.View())
	b.WriteString("\n\n")

	// Help
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	b.WriteString(helpStyle.Render("Tab/↑↓: Navigate • Enter: Next/Submit • Esc: Cancel"))

	return b.String()
}

// IsDone returns whether the user has finished with the input
func (m StartInputModel) IsDone() bool {
	return m.done
}

// IsCancelled returns whether the user cancelled the input
func (m StartInputModel) IsCancelled() bool {
	return m.cancelled
}

// GetKeyword returns the entered keyword
func (m StartInputModel) GetKeyword() string {
	return m.keyword
}

// GetTags returns the entered tags
func (m StartInputModel) GetTags() []string {
	return m.tags
}

// GetError returns any error that occurred
func (m StartInputModel) GetError() error {
	return m.err
}

// RunStartInput runs the start input TUI and returns the keyword and tags
func RunStartInput() (string, []string, error) {
	model := NewStartInputModel()

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", nil, fmt.Errorf("failed to run start input TUI: %w", err)
	}

	startModel := finalModel.(StartInputModel)
	if startModel.GetError() != nil {
		return "", nil, startModel.GetError()
	}

	if startModel.IsCancelled() {
		return "", nil, fmt.Errorf("cancelled")
	}

	return startModel.GetKeyword(), startModel.GetTags(), nil
}
