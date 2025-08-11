package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectorItem represents an item that can be selected in the table
type SelectorItem struct {
	ID          string // Unique identifier
	DisplayText string // Text to show in the table (legacy single-column format)
	Data        any    // Additional data associated with the item

	// Multi-column fields (optional, when provided will use columnar display)
	Columns []string // Column values for multi-column display
}

// SelectorModel represents the interactive table selector TUI
type SelectorModel struct {
	title         string
	items         []SelectorItem
	filteredItems []SelectorItem
	table         table.Model
	done          bool
	cancelled     bool
	selectedItem  *SelectorItem
	err           error
	showHelp      bool
	searchMode    bool
	searchInput   textinput.Model
	searchFocused bool // true when search input has focus, false when table has focus

	// Multi-selection support
	multiSelect   bool
	selectedItems map[string]bool // Map of item IDs to selection state
}

// NewSelectorModel creates a new selector model
func NewSelectorModel(title string, items []SelectorItem) SelectorModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Type to search..."
	searchInput.CharLimit = 50
	searchInput.Width = 40

	// Determine if we should use multi-column layout
	useColumns := len(items) > 0 && len(items[0].Columns) > 0

	var columns []table.Column
	var rows []table.Row

	if useColumns {
		// Detect column headers and widths based on content
		columnCount := len(items[0].Columns)

		// Create columns with appropriate headers and widths
		if columnCount == 5 { // ID, Keyword, Tags, Status, Duration format
			columns = []table.Column{
				{Title: "ID", Width: 4},
				{Title: "Keyword", Width: 15},
				{Title: "Tags", Width: 20},
				{Title: "Status", Width: 10},
				{Title: "Duration", Width: 12},
			}
		} else if columnCount == 4 { // Keyword, Tags, StartTime, Duration format (continue)
			columns = []table.Column{
				{Title: "Keyword", Width: 15},
				{Title: "Tags", Width: 20},
				{Title: "Start Time", Width: 15},
				{Title: "Duration", Width: 12},
			}
		} else {
			// Generic column layout for other cases
			columns = make([]table.Column, columnCount)
			baseWidth := 80 / columnCount
			for i := 0; i < columnCount; i++ {
				columns[i] = table.Column{
					Title: fmt.Sprintf("Col %d", i+1),
					Width: baseWidth,
				}
			}
		}

		// Convert items to multi-column table rows
		rows = make([]table.Row, len(items))
		for i, item := range items {
			rows[i] = table.Row(item.Columns)
		}
	} else {
		// Legacy single-column layout
		columns = []table.Column{
			{Title: "Selection", Width: 80},
		}

		// Convert items to single-column table rows
		rows = make([]table.Row, len(items))
		for i, item := range items {
			rows[i] = table.Row{item.DisplayText}
		}
	}

	// Create table with styling
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10), // Show up to 10 items
	)

	// Set table styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return SelectorModel{
		title:         title,
		items:         items,
		filteredItems: items, // Initially show all items
		table:         t,
		showHelp:      true,
		searchMode:    false,
		searchInput:   searchInput,
		searchFocused: false,
		multiSelect:   false,
		selectedItems: make(map[string]bool),
	}
}

// NewMultiSelectorModel creates a new multi-selection selector model
func NewMultiSelectorModel(title string, items []SelectorItem) SelectorModel {
	model := NewSelectorModel(title, items)
	model.multiSelect = true

	// Update table columns to include selection indicator
	if len(items) > 0 && len(items[0].Columns) > 0 {
		// Multi-column mode - add selection column as first column
		currentCols := model.table.Columns()
		newCols := make([]table.Column, len(currentCols)+1)
		newCols[0] = table.Column{Title: "☐", Width: 3}
		copy(newCols[1:], currentCols)
		model.table.SetColumns(newCols)
	} else {
		// Single column mode - the selection indicator is part of the text
		currentCols := model.table.Columns()
		newCols := make([]table.Column, len(currentCols))
		copy(newCols, currentCols)
		// Expand the width to accommodate the selection indicator
		if len(newCols) > 0 {
			newCols[0].Width += 3
		}
		model.table.SetColumns(newCols)
	}

	// Rebuild the table to include the checkbox indicators in the initial display
	model.rebuildTable()

	return model
}

func (m SelectorModel) Init() tea.Cmd {
	return textinput.Blink
}

// rebuildTable recreates the table with the current filtered items
func (m *SelectorModel) rebuildTable() {
	// Determine if we should use multi-column layout
	useColumns := len(m.filteredItems) > 0 && len(m.filteredItems[0].Columns) > 0

	// Convert filtered items to table rows
	rows := make([]table.Row, len(m.filteredItems))
	for i, item := range m.filteredItems {
		if useColumns {
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
		} else {
			if m.multiSelect {
				// Add selection indicator to display text
				indicator := "☐ "
				if m.selectedItems[item.ID] {
					indicator = "☑ "
				}
				rows[i] = table.Row{indicator + item.DisplayText}
			} else {
				rows[i] = table.Row{item.DisplayText}
			}
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
			// Create searchable text from columns or fallback to DisplayText
			var searchableText string
			if len(item.Columns) > 0 {
				// Combine all columns into a single searchable string
				var textBuilder strings.Builder
				for i, col := range item.Columns {
					if i > 0 {
						textBuilder.WriteString(" ")
					}
					textBuilder.WriteString(strings.ToLower(col))
				}
				searchableText = textBuilder.String()
			} else {
				// Fall back to DisplayText search for legacy items
				searchableText = strings.ToLower(item.DisplayText)
			}

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
}

func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.searchMode {
			// Handle search mode keys
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
				} else {
					// Handle selection from table
					if len(m.filteredItems) > 0 {
						selectedRow := m.table.Cursor()
						if selectedRow < len(m.filteredItems) {
							if m.multiSelect {
								// In multi-select mode, enter finalizes selection
								m.done = true
								return m, tea.Quit
							} else {
								// Single select mode
								m.done = true
								m.selectedItem = &m.filteredItems[selectedRow]
								return m, tea.Quit
							}
						}
					}
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
			case "up", "ctrl+p":
				if m.searchFocused {
					// If search input is focused, switch to table and navigate
					m.searchFocused = false
					m.searchInput.Blur()
				}
				// Let table handle navigation
				m.table, cmd = m.table.Update(msg)
				return m, cmd
			case "down", "ctrl+n":
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
					oldValue := m.searchInput.Value()
					m.searchInput, cmd = m.searchInput.Update(msg)
					newValue := m.searchInput.Value()
					if oldValue != newValue {
						m.filterItems(newValue)
					}
					return m, cmd
				} else {
					// Space toggles selection in table
					if m.multiSelect && len(m.filteredItems) > 0 {
						selectedRow := m.table.Cursor()
						if selectedRow < len(m.filteredItems) {
							item := &m.filteredItems[selectedRow]
							// Toggle selection
							m.selectedItems[item.ID] = !m.selectedItems[item.ID]
							// Rebuild table to show selection changes
							m.rebuildTable()
							return m, nil
						}
					}
				}
			default:
				if m.searchFocused {
					// Update search input only when it has focus
					oldValue := m.searchInput.Value()
					m.searchInput, cmd = m.searchInput.Update(msg)
					newValue := m.searchInput.Value()

					// If search query changed, filter items
					if oldValue != newValue {
						m.filterItems(newValue)
					}
					return m, cmd
				} else {
					// When table has focus, let it handle other navigation keys
					m.table, cmd = m.table.Update(msg)
					return m, cmd
				}
			}
		} else {
			// Handle normal navigation mode keys
			switch msg.String() {
			case "ctrl+c", "esc", "q":
				m.done = true
				m.cancelled = true
				m.err = fmt.Errorf("cancelled")
				return m, tea.Quit

			case "enter":
				if len(m.filteredItems) > 0 {
					if m.multiSelect {
						// In multi-select mode, enter finalizes selection
						m.done = true
						return m, tea.Quit
					} else {
						// Single select mode
						selectedRow := m.table.Cursor()
						if selectedRow < len(m.filteredItems) {
							m.done = true
							m.selectedItem = &m.filteredItems[selectedRow]
							return m, tea.Quit
						}
					}
				}

			case " ", "space":
				if m.multiSelect && len(m.filteredItems) > 0 {
					selectedRow := m.table.Cursor()
					if selectedRow < len(m.filteredItems) {
						item := &m.filteredItems[selectedRow]
						// Toggle selection
						m.selectedItems[item.ID] = !m.selectedItems[item.ID]
						// Rebuild table to show selection changes
						m.rebuildTable()
						return m, nil
					}
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
		}
	}

	// Update table for other message types
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

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

		var help string
		if m.searchMode {
			if m.searchFocused {
				// Search input has focus
				if m.multiSelect {
					help = "Type to search • Tab/↑/↓: Focus table • Enter: Focus table • Esc: Exit search"
				} else {
					help = "Type to search • Tab/↑/↓: Focus table • Enter: Focus table • Esc: Exit search • ?/h: Help"
				}
			} else {
				// Table has focus in search mode
				if m.multiSelect {
					help = "Tab: Focus search • ↑/↓: Navigate • Space: Toggle • Enter: Confirm • Esc: Exit search"
				} else {
					help = "Tab: Focus search • ↑/↓: Navigate • Enter: Select • Esc: Exit search • ?/h: Help"
				}
			}
		} else {
			if m.multiSelect {
				help = "↑/j ↓/k: Navigate • f: Search • Space: Toggle • Enter: Confirm • ?/h: Help • Esc/q: Cancel"
			} else {
				help = "↑/j ↓/k: Navigate • f: Search • Enter: Select • ?/h: Help • Esc/q: Cancel"
			}
		}
		b.WriteString(helpStyle.Render(help))
	}

	return b.String()
}

// IsDone returns whether the user has finished with the selection
func (m SelectorModel) IsDone() bool {
	return m.done
}

// IsCancelled returns whether the user cancelled the selection
func (m SelectorModel) IsCancelled() bool {
	return m.cancelled
}

// GetSelectedItem returns the selected item
func (m SelectorModel) GetSelectedItem() *SelectorItem {
	return m.selectedItem
}

// GetError returns any error that occurred
func (m SelectorModel) GetError() error {
	return m.err
}

// GetSelectedItems returns all selected items in multi-select mode
func (m SelectorModel) GetSelectedItems() []*SelectorItem {
	var selected []*SelectorItem
	for _, item := range m.items {
		if m.selectedItems[item.ID] {
			// Create a copy to avoid reference issues
			itemCopy := item
			selected = append(selected, &itemCopy)
		}
	}
	return selected
}

// IsMultiSelect returns whether this selector is in multi-select mode
func (m SelectorModel) IsMultiSelect() bool {
	return m.multiSelect
}

// RunSelector runs the selector TUI and returns the selected item
func RunSelector(title string, items []SelectorItem) (*SelectorItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}

	model := NewSelectorModel(title, items)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run selector TUI: %w", err)
	}

	selectorModel := finalModel.(SelectorModel)
	if selectorModel.GetError() != nil {
		return nil, selectorModel.GetError()
	}

	if selectorModel.IsCancelled() {
		return nil, fmt.Errorf("cancelled")
	}

	return selectorModel.GetSelectedItem(), nil
}

// RunMultiSelector runs the multi-selector TUI and returns the selected items
func RunMultiSelector(title string, items []SelectorItem) ([]*SelectorItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}

	model := NewMultiSelectorModel(title, items)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run multi-selector TUI: %w", err)
	}

	selectorModel := finalModel.(SelectorModel)
	if selectorModel.GetError() != nil {
		return nil, selectorModel.GetError()
	}

	if selectorModel.IsCancelled() {
		return nil, fmt.Errorf("cancelled")
	}

	return selectorModel.GetSelectedItems(), nil
}
