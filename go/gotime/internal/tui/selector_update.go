package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// rebuildTable recreates the table with the current filtered items
func (m *SelectorModel) rebuildTable() {
	// Convert filtered items to table rows
	rows := make([]table.Row, len(m.filteredItems))
	for i, item := range m.filteredItems {
		if m.multiSelect {
			// Add selection indicator as first column
			indicator := "☐"
			if m.selectedItems[item.ID] {
				indicator = "☑"
			}
			row := make(table.Row, len(item.Columns)+1)
			row[0] = indicator
			copy(row[1:], item.Columns)
			rows[i] = row
		} else {
			rows[i] = table.Row(item.Columns)
		}
	}

	m.table.SetRows(rows)
}

// filterItems filters the items based on the search query using space-delimited fuzzy search
func (m *SelectorModel) filterItems(query string) {
	if query == "" {
		m.filteredItems = m.items
	} else {
		var filtered []SelectorItem

		// Split query into individual terms for fuzzy search
		queryTerms := strings.Fields(strings.ToLower(query))

		for _, item := range m.items {
			// Combine all columns into a single searchable string
			var textBuilder strings.Builder
			for i, col := range item.Columns {
				if i > 0 {
					textBuilder.WriteString(" ")
				}
				textBuilder.WriteString(strings.ToLower(col))
			}
			searchableText := textBuilder.String()

			// Check if all query terms match somewhere in the searchable text
			allTermsMatch := true
			for _, term := range queryTerms {
				if !strings.Contains(searchableText, term) {
					allTermsMatch = false
					break
				}
			}

			// Only include items where all terms were found
			if allTermsMatch {
				filtered = append(filtered, item)
			}
		}
		m.filteredItems = filtered
	}

	// Rebuild the table with filtered items
	m.rebuildTable()

	// Reset cursor position if it's beyond the filtered results
	currentCursor := m.table.Cursor()
	if len(m.filteredItems) == 0 {
		// No items to select, cursor will be handled by table component
		// (table typically sets cursor to -1 when no rows available)
	} else if currentCursor >= len(m.filteredItems) {
		// Cursor is beyond available filtered items, reset to first item
		m.table.SetCursor(0)
	}
}

// toggleSelectionUnderCursor flips the multi-selection state of the row the
// cursor sits on, reporting whether anything changed.
func (m *SelectorModel) toggleSelectionUnderCursor() bool {
	if !m.multiSelect || len(m.filteredItems) == 0 {
		return false
	}

	selectedRow := m.table.Cursor()
	if selectedRow >= len(m.filteredItems) {
		return false
	}

	item := &m.filteredItems[selectedRow]
	m.selectedItems[item.ID] = !m.selectedItems[item.ID]
	// Rebuild table to show selection changes
	m.rebuildTable()
	return true
}

// confirmSelection finishes the selection: in multi-select mode the accumulated
// selection is kept, otherwise the row under the cursor becomes the result. It
// reports whether the selector is done and should quit.
func (m *SelectorModel) confirmSelection() bool {
	if len(m.filteredItems) == 0 {
		return false
	}

	if m.multiSelect {
		// In multi-select mode, enter finalizes selection
		m.done = true
		return true
	}

	// Single select mode
	selectedRow := m.table.Cursor()
	if selectedRow >= len(m.filteredItems) {
		return false
	}

	m.done = true
	m.selectedItem = &m.filteredItems[selectedRow]
	return true
}

func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if key, ok := msg.(tea.KeyMsg); ok {
		if m.searchMode {
			return m.updateSearchMode(key)
		}
		return m.updateNavigationMode(key)
	}

	// Update table for other message types
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// updateSearchMode handles keys while the search field is on screen, where
// focus alternates between the query input and the table.
func (m SelectorModel) updateSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		// Exit search mode
		m.searchMode = false
		m.searchFocused = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m.filterItems("")
		return m, nil

	case "enter":
		if m.searchFocused {
			// Switch focus from search input to table
			m.searchFocused = false
			m.searchInput.Blur()
			return m, nil
		}
		if m.confirmSelection() {
			return m, tea.Quit
		}

	case "tab":
		// Toggle focus between search input and table
		m.searchFocused = !m.searchFocused
		if m.searchFocused {
			m.searchInput.Focus()
		} else {
			m.searchInput.Blur()
		}
		return m, textinput.Blink

	case "up", "ctrl+p", "down", "ctrl+n":
		if m.searchFocused {
			// If search input is focused, switch to table and navigate
			m.searchFocused = false
			m.searchInput.Blur()
		}
		// Let table handle navigation
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case " ", "space":
		if m.searchFocused {
			// Space goes to search input
			return m.updateSearchQuery(msg)
		}
		// Space toggles selection in table
		if m.toggleSelectionUnderCursor() {
			return m, nil
		}

	default:
		if m.searchFocused {
			// Update search input only when it has focus
			return m.updateSearchQuery(msg)
		}
		// When table has focus, let it handle other navigation keys
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	// Nothing consumed the key, so let the table have it.
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// updateSearchQuery forwards a key to the search input and refilters the items
// when the query actually changed.
func (m SelectorModel) updateSearchQuery(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	oldValue := m.searchInput.Value()
	m.searchInput, cmd = m.searchInput.Update(msg)
	newValue := m.searchInput.Value()

	// If search query changed, filter items
	if oldValue != newValue {
		m.filterItems(newValue)
	}
	return m, cmd
}

// updateNavigationMode handles keys while the selector is browsing its items
// without a search field.
func (m SelectorModel) updateNavigationMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c", "esc", "q":
		m.done = true
		m.cancelled = true
		m.err = fmt.Errorf("cancelled")
		return m, tea.Quit

	case "enter":
		if m.confirmSelection() {
			return m, tea.Quit
		}

	case " ", "space":
		if m.toggleSelectionUnderCursor() {
			return m, nil
		}

	case "f":
		// Enter search mode
		m.searchMode = true
		m.searchFocused = true
		m.searchInput.Focus()
		return m, textinput.Blink

	case "?", "h":
		m.showHelp = !m.showHelp
		return m, nil

	default:
		// Let table handle all other navigation (up/down/home/end/etc)
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	// Nothing consumed the key, so let the table have it.
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}
