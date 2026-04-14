package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/dev-helpers/go/hr/internal/config"
)

var ErrCancelled = errors.New("selection cancelled")

var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	headerStyle   = lipgloss.NewStyle().Bold(true)
)

type selectorModel struct {
	exercises []config.Exercise
	cursor    int
	filter    string
	selected  config.Exercise
	done      bool
	cancelled bool
}

func (m selectorModel) filtered() []config.Exercise {
	if m.filter == "" {
		return m.exercises
	}
	f := strings.ToLower(m.filter)
	var result []config.Exercise
	for _, ex := range m.exercises {
		if strings.Contains(strings.ToLower(ex.Name), f) {
			result = append(result, ex)
		}
	}
	return result
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		filtered := m.filtered()
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case "enter":
			if len(filtered) > 0 {
				m.selected = filtered[m.cursor]
				m.done = true
				return m, tea.Quit
			}
		case "q", "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.cursor = 0
			}
		default:
			if len(msg.Text) > 0 && msg.Text != " " {
				m.filter += msg.Text
				m.cursor = 0
			} else if msg.String() == "space" {
				m.filter += " "
				m.cursor = 0
			}
		}
	}
	return m, nil
}

func (m selectorModel) View() tea.View {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Select exercise:"))
	sb.WriteString("\n\n")

	if m.filter != "" {
		sb.WriteString(fmt.Sprintf("Filter: %s\n\n", m.filter))
	}

	filtered := m.filtered()
	if len(filtered) == 0 {
		sb.WriteString(dimStyle.Render("No matches. Press backspace to clear filter."))
		sb.WriteString("\n")
	} else {
		for i, ex := range filtered {
			repsHint := dimStyle.Render(fmt.Sprintf("(default %d reps)", ex.DefaultReps))
			if i == m.cursor {
				line := fmt.Sprintf("%s %s %s", cursorStyle.Render(">"), selectedStyle.Render(ex.Name), repsHint)
				sb.WriteString(line)
			} else {
				sb.WriteString(fmt.Sprintf("  %s %s", ex.Name, repsHint))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("↑/↓ or j/k: navigate  enter: select  type: filter  q/esc: cancel"))
	sb.WriteString("\n")

	return tea.NewView(sb.String())
}

// RunSelector runs the interactive exercise selector and returns the chosen exercise.
func RunSelector(exercises []config.Exercise) (config.Exercise, error) {
	m := selectorModel{exercises: exercises}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return config.Exercise{}, fmt.Errorf("running selector: %w", err)
	}
	result := final.(selectorModel)
	if result.cancelled {
		return config.Exercise{}, ErrCancelled
	}
	return result.selected, nil
}
