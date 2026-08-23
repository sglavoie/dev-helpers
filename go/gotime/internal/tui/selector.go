package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TableConfig represents the configuration for table columns
type TableConfig struct {
	Columns []table.Column
}

// SelectorItem represents an item that can be selected in the table
type SelectorItem struct {
	ID      string   // Unique identifier
	Data    any      // Additional data associated with the item
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
	return newSelectorModel(title, items, detectColumns(items))
}

// NewMultiSelectorModel creates a new multi-selection selector model
func NewMultiSelectorModel(title string, items []SelectorItem) SelectorModel {
	return NewSelectorModel(title, items).asMultiSelect()
}

// NewSelectorModelWithConfig creates a new selector model with explicit column configuration
func NewSelectorModelWithConfig(title string, items []SelectorItem, config TableConfig) SelectorModel {
	return newSelectorModel(title, items, config.Columns)
}

// NewMultiSelectorModelWithConfig creates a new multi-selection selector model with explicit column configuration
func NewMultiSelectorModelWithConfig(title string, items []SelectorItem, config TableConfig) SelectorModel {
	return NewSelectorModelWithConfig(title, items, config).asMultiSelect()
}

// newSelectorModel builds the state every selector shares: a search input, one
// table row per item and the common table styling.
func newSelectorModel(title string, items []SelectorItem, columns []table.Column) SelectorModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Type to search..."
	searchInput.CharLimit = 50
	searchInput.Width = 40

	// Convert items to multi-column table rows
	rows := make([]table.Row, len(items))
	for i, item := range items {
		rows[i] = table.Row(item.Columns)
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
		searchInput:   searchInput,
		selectedItems: make(map[string]bool),
	}
}

// detectColumns infers column headers and widths from the shape of the items,
// which is how callers that do not supply a TableConfig get a usable table.
func detectColumns(items []SelectorItem) []table.Column {
	columnCount := len(items[0].Columns)

	switch {
	case columnCount == 5 && isUndoOperation(items[0].Columns[1]):
		// Undo operations format: Index, Operation, Description, Timestamp, Relative Time
		return []table.Column{
			{Title: "Index", Width: 6},
			{Title: "Operation", Width: 12},
			{Title: "Description", Width: 30},
			{Title: "Timestamp", Width: 15},
			{Title: "Relative", Width: 16},
		}
	case columnCount == 5: // ID, Keyword, Tags, Status, Duration format
		return []table.Column{
			{Title: "ID", Width: 4},
			{Title: "Keyword", Width: 15},
			{Title: "Tags", Width: 20},
			{Title: "Status", Width: 10},
			{Title: "Duration", Width: 12},
		}
	case columnCount == 4: // Keyword, Tags, StartTime, Duration format (continue)
		return []table.Column{
			{Title: "Keyword", Width: 15},
			{Title: "Tags", Width: 20},
			{Title: "End Time", Width: 15},
			{Title: "Duration", Width: 12},
		}
	}

	// Generic column layout for other cases
	columns := make([]table.Column, columnCount)
	baseWidth := 80 / columnCount
	for i := range columnCount {
		columns[i] = table.Column{
			Title: fmt.Sprintf("Col %d", i+1),
			Width: baseWidth,
		}
	}
	return columns
}

// isUndoOperation reports whether a second column holds one of the operation
// names the undo history uses, which is what distinguishes an undo listing from
// an entry listing of the same width.
func isUndoOperation(column string) bool {
	return strings.Contains(column, "delete") ||
		strings.Contains(column, "bulk_edit") ||
		strings.Contains(column, "clear")
}

// asMultiSelect turns a freshly built selector into a multi-selection one by
// prepending the checkbox column and redrawing the rows with their indicators.
func (m SelectorModel) asMultiSelect() SelectorModel {
	m.multiSelect = true

	// Update table columns to include selection indicator
	if currentCols := m.table.Columns(); len(currentCols) > 0 {
		newCols := make([]table.Column, len(currentCols)+1)
		newCols[0] = table.Column{Title: "☐", Width: 3}
		copy(newCols[1:], currentCols)
		m.table.SetColumns(newCols)
	}

	// Rebuild the table to include the checkbox indicators in the initial display
	m.rebuildTable()

	return m
}

func (m SelectorModel) Init() tea.Cmd {
	return textinput.Blink
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
