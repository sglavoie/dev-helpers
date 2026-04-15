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
	selectedStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	cursorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	noteStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	headerStyle    = lipgloss.NewStyle().Bold(true)
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

type selectorItem struct {
	isSeparator bool
	label       string
	exercise    config.Exercise
}

type selectorModel struct {
	cfg       config.Config
	cursor    int
	filter    string
	selected  config.Exercise
	done      bool
	cancelled bool
}

func buildItems(cfg config.Config, filter string) []selectorItem {
	f := strings.ToLower(filter)
	byCategory := cfg.ExercisesByCategory()
	var items []selectorItem
	for _, cat := range cfg.Categories() {
		exs := byCategory[cat]
		var matching []config.Exercise
		for _, ex := range exs {
			if filter == "" || strings.Contains(strings.ToLower(ex.Name), f) || strings.Contains(strings.ToLower(ex.Note), f) {
				matching = append(matching, ex)
			}
		}
		if len(matching) == 0 {
			continue
		}
		items = append(items, selectorItem{isSeparator: true, label: fmt.Sprintf("── %s ──", cat)})
		for _, ex := range matching {
			items = append(items, selectorItem{exercise: ex})
		}
	}
	return items
}

func (m selectorModel) items() []selectorItem {
	return buildItems(m.cfg, m.filter)
}

func nextSelectable(items []selectorItem, start int) int {
	for i := start; i < len(items); i++ {
		if !items[i].isSeparator {
			return i
		}
	}
	return -1
}

func prevSelectable(items []selectorItem, start int) int {
	for i := start; i >= 0; i-- {
		if !items[i].isSeparator {
			return i
		}
	}
	return -1
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		items := m.items()
		switch msg.String() {
		case "up", "k":
			if prev := prevSelectable(items, m.cursor-1); prev >= 0 {
				m.cursor = prev
			}
		case "down", "j":
			if next := nextSelectable(items, m.cursor+1); next >= 0 {
				m.cursor = next
			}
		case "enter":
			if m.cursor >= 0 && m.cursor < len(items) && !items[m.cursor].isSeparator {
				m.selected = items[m.cursor].exercise
				m.done = true
				return m, tea.Quit
			}
		case "q", "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				newItems := buildItems(m.cfg, m.filter)
				if first := nextSelectable(newItems, 0); first >= 0 {
					m.cursor = first
				}
			}
		default:
			if len(msg.Text) > 0 && msg.Text != " " {
				m.filter += msg.Text
				newItems := buildItems(m.cfg, m.filter)
				if first := nextSelectable(newItems, 0); first >= 0 {
					m.cursor = first
				}
			} else if msg.String() == "space" {
				m.filter += " "
				newItems := buildItems(m.cfg, m.filter)
				if first := nextSelectable(newItems, 0); first >= 0 {
					m.cursor = first
				}
			}
		}
	}
	return m, nil
}

func fieldDefaults(ex config.Exercise) string {
	var parts []string
	for _, f := range ex.Fields {
		parts = append(parts, fmt.Sprintf("%s: %v", f.Name, f.Default))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func (m selectorModel) View() tea.View {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Select exercise:"))
	sb.WriteString("\n\n")

	if m.filter != "" {
		sb.WriteString(fmt.Sprintf("Filter: %s\n\n", m.filter))
	}

	items := m.items()
	if len(items) == 0 {
		sb.WriteString(dimStyle.Render("No matches. Press backspace to clear filter."))
		sb.WriteString("\n")
	} else {
		for i, item := range items {
			if item.isSeparator {
				sb.WriteString(separatorStyle.Render(item.label))
				sb.WriteString("\n")
				continue
			}
			hint := dimStyle.Render(fieldDefaults(item.exercise))
			note := ""
			if item.exercise.Note != "" {
				note = " " + noteStyle.Render(item.exercise.Note)
			}
			if i == m.cursor {
				line := fmt.Sprintf("%s %s%s %s", cursorStyle.Render(">"), selectedStyle.Render(item.exercise.Name), note, hint)
				sb.WriteString(line)
			} else {
				sb.WriteString(fmt.Sprintf("  %s%s %s", item.exercise.Name, note, hint))
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
func RunSelector(cfg config.Config) (config.Exercise, error) {
	items := buildItems(cfg, "")
	initialCursor := 0
	if first := nextSelectable(items, 0); first >= 0 {
		initialCursor = first
	}
	m := selectorModel{cfg: cfg, cursor: initialCursor}
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
