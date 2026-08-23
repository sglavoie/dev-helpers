package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m SelectorModel) View() string {
	if len(m.items) == 0 {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")).
			Render("No items to display")
	}

	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n")

	// Search field (if in search mode)
	if m.searchMode {
		searchStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("33")).
			MarginBottom(1)

		b.WriteString(searchStyle.Render("🔍 Search: "))
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
	}

	// Show filtered count if searching
	if m.searchMode && len(m.filteredItems) < len(m.items) {
		countStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			MarginBottom(1)
		count := fmt.Sprintf("Showing %d of %d items", len(m.filteredItems), len(m.items))
		b.WriteString(countStyle.Render(count))
		b.WriteString("\n")
	}

	// Table view
	if len(m.filteredItems) == 0 {
		noResultsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			MarginTop(1).
			MarginBottom(1)
		b.WriteString(noResultsStyle.Render("No matching items found"))
		b.WriteString("\n")
	} else {
		// Create a bordered style for the table
		baseStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

		b.WriteString(baseStyle.Render(m.table.View()))
		b.WriteString("\n")
	}

	// Help
	if m.showHelp {
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

		b.WriteString(helpStyle.Render(m.helpText()))
	}

	return b.String()
}

// helpText returns the key hints for the selector's current mode and focus.
func (m SelectorModel) helpText() string {
	if !m.searchMode {
		if m.multiSelect {
			return "↑/j ↓/k: Navigate • f: Search • Space: Toggle • Enter: Confirm • ?/h: Help • Esc/q: Cancel"
		}
		return "↑/j ↓/k: Navigate • f: Search • Enter: Select • ?/h: Help • Esc/q: Cancel"
	}

	if m.searchFocused {
		// Search input has focus
		if m.multiSelect {
			return "Type to search • Tab/↑/↓: Focus table • Enter: Focus table • Esc: Exit search"
		}
		return "Type to search • Tab/↑/↓: Focus table • Enter: Focus table • Esc: Exit search • ?/h: Help"
	}

	// Table has focus in search mode
	if m.multiSelect {
		return "Tab: Focus search • ↑/↓: Navigate • Space: Toggle • Enter: Confirm • Esc: Exit search"
	}
	return "Tab: Focus search • ↑/↓: Navigate • Enter: Select • Esc: Exit search • ?/h: Help"
}
