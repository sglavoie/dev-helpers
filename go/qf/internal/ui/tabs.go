// Package ui provides the TabsModel component for managing file tabs.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/dev-helpers/go/qf/internal/session"
)

// TabsModel manages the file tabs display and interaction
type TabsModel struct {
	tabs        []session.FileTab
	activeIndex int
	width       int
	focused     bool
}

// NewTabsModel creates a new TabsModel
func NewTabsModel(tabs []session.FileTab) *TabsModel {
	activeIndex := -1
	for i, tab := range tabs {
		if tab.Active {
			activeIndex = i
			break
		}
	}

	return &TabsModel{
		tabs:        tabs,
		activeIndex: activeIndex,
		width:       80,
		focused:     false,
	}
}

// Init implements tea.Model interface
func (m *TabsModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model interface
func (m *TabsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WindowResizeMsg:
		m.width = msg.Width
		return m, nil

	case FocusChangeMsg:
		m.focused = (ParseFocusedComponent(msg.NewFocus) == FocusTabs)
		return m, nil

	case TabSwitchMsg:
		// Update active tab based on message
		for i, tab := range m.tabs {
			if tab.ID == msg.NewTabID {
				m.activeIndex = i
				// Update the active flag in the tab
				for j := range m.tabs {
					m.tabs[j].Active = (j == i)
				}
				break
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.focused {
			return m.handleKeyPress(msg)
		}
	}

	return m, nil
}

// handleKeyPress processes key presses when tabs are focused
func (m *TabsModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "h", "left":
		return m.switchToPrevTab()
	case "l", "right":
		return m.switchToNextTab()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return m.switchToTabNumber(key)
	case "x", "ctrl+w":
		return m.closeCurrentTab()
	}

	return m, nil
}

// switchToPrevTab switches to the previous tab
func (m *TabsModel) switchToPrevTab() (tea.Model, tea.Cmd) {
	if len(m.tabs) <= 1 {
		return m, nil
	}

	prevIndex := (m.activeIndex - 1 + len(m.tabs)) % len(m.tabs)
	currentTabID := ""
	if m.activeIndex >= 0 && m.activeIndex < len(m.tabs) {
		currentTabID = m.tabs[m.activeIndex].ID
	}

	return m, func() tea.Msg {
		return NewTabSwitchMsg(
			m.tabs[prevIndex].ID,
			currentTabID,
			prevIndex,
			"tab_left",
		)
	}
}

// switchToNextTab switches to the next tab
func (m *TabsModel) switchToNextTab() (tea.Model, tea.Cmd) {
	if len(m.tabs) <= 1 {
		return m, nil
	}

	nextIndex := (m.activeIndex + 1) % len(m.tabs)
	currentTabID := ""
	if m.activeIndex >= 0 && m.activeIndex < len(m.tabs) {
		currentTabID = m.tabs[m.activeIndex].ID
	}

	return m, func() tea.Msg {
		return NewTabSwitchMsg(
			m.tabs[nextIndex].ID,
			currentTabID,
			nextIndex,
			"tab_right",
		)
	}
}

// switchToTabNumber switches to a specific tab by number
func (m *TabsModel) switchToTabNumber(key string) (tea.Model, tea.Cmd) {
	tabNum := int(key[0] - '0') // Convert character to number
	tabIndex := tabNum - 1      // Convert to 0-based index

	if tabIndex < 0 || tabIndex >= len(m.tabs) {
		return m, func() tea.Msg {
			return NewErrorMsg(
				fmt.Sprintf("Tab %d does not exist", tabNum),
				"tab_switching",
				"tabs",
				true,
			)
		}
	}

	currentTabID := ""
	if m.activeIndex >= 0 && m.activeIndex < len(m.tabs) {
		currentTabID = m.tabs[m.activeIndex].ID
	}

	return m, func() tea.Msg {
		return NewTabSwitchMsg(
			m.tabs[tabIndex].ID,
			currentTabID,
			tabIndex,
			"number_key",
		)
	}
}

// closeCurrentTab closes the currently active tab
func (m *TabsModel) closeCurrentTab() (tea.Model, tea.Cmd) {
	if m.activeIndex < 0 || m.activeIndex >= len(m.tabs) {
		return m, func() tea.Msg {
			return NewErrorMsg("No active tab to close", "tab_management", "tabs", true)
		}
	}

	// Remove tab from local list
	m.tabs = append(m.tabs[:m.activeIndex], m.tabs[m.activeIndex+1:]...)

	// Adjust active index
	if len(m.tabs) == 0 {
		m.activeIndex = -1
	} else if m.activeIndex >= len(m.tabs) {
		m.activeIndex = len(m.tabs) - 1
	}

	// Update active flag
	if m.activeIndex >= 0 {
		for i := range m.tabs {
			m.tabs[i].Active = (i == m.activeIndex)
		}
	}

	return m, func() tea.Msg {
		return NewStatusUpdateMsg("Tab closed", StatusInfo, "tab_closed")
	}
}

// View implements tea.Model interface
func (m *TabsModel) View() string {
	if len(m.tabs) <= 1 {
		// Don't show tab bar for single tab
		return ""
	}

	var parts []string
	remainingWidth := m.width
	maxTabWidth := 20 // Maximum width for each tab

	// Calculate tab width
	tabWidth := maxTabWidth
	if len(m.tabs) > 0 {
		availableWidth := m.width - 4 // Reserve space for separators
		calculatedWidth := availableWidth / len(m.tabs)
		if calculatedWidth < tabWidth {
			tabWidth = calculatedWidth
		}
		if tabWidth < 8 {
			tabWidth = 8 // Minimum tab width
		}
	}

	for i, tab := range m.tabs {
		if remainingWidth < 8 {
			// Not enough space, add "..." indicator
			parts = append(parts, "...")
			break
		}

		// Generate tab display name
		displayName := tab.FilePath
		if len(displayName) > 30 {
			// Use just filename if path is too long
			pathParts := strings.Split(displayName, "/")
			if len(pathParts) > 0 {
				displayName = pathParts[len(pathParts)-1]
			}
		}

		// Truncate if still too long
		if len(displayName) > tabWidth-4 {
			displayName = displayName[:tabWidth-7] + "..."
		}

		// Tab formatting
		tabContent := fmt.Sprintf(" %s ", displayName)

		// Pad to tab width
		for len(tabContent) < tabWidth {
			tabContent += " "
		}

		// Add tab number indicator
		tabNumbered := fmt.Sprintf("[%d]%s", i+1, tabContent)

		// Highlight active tab
		if i == m.activeIndex {
			if m.focused {
				tabNumbered = fmt.Sprintf("*%s*", tabContent)
			} else {
				tabNumbered = fmt.Sprintf(">%s<", tabContent)
			}
		}

		parts = append(parts, tabNumbered)
		remainingWidth -= len(tabNumbered)

		// Add separator if not last tab
		if i < len(m.tabs)-1 && remainingWidth > 1 {
			parts = append(parts, "|")
			remainingWidth -= 1
		}
	}

	result := strings.Join(parts, "")

	// Pad to full width
	if len(result) < m.width {
		result += strings.Repeat(" ", m.width-len(result))
	}

	return result
}

// SetTabs updates the tabs list
func (m *TabsModel) SetTabs(tabs []session.FileTab) {
	m.tabs = tabs

	// Update active index
	m.activeIndex = -1
	for i, tab := range tabs {
		if tab.Active {
			m.activeIndex = i
			break
		}
	}
}

// GetActiveTab returns the currently active tab
func (m *TabsModel) GetActiveTab() *session.FileTab {
	if m.activeIndex < 0 || m.activeIndex >= len(m.tabs) {
		return nil
	}
	return &m.tabs[m.activeIndex]
}

// GetTabCount returns the number of tabs
func (m *TabsModel) GetTabCount() int {
	return len(m.tabs)
}

// MessageHandler interface implementation

// HandleMessage implements MessageHandler interface
func (m *TabsModel) HandleMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.Update(msg)
}

// GetComponentType implements MessageHandler interface
func (m *TabsModel) GetComponentType() string {
	return "tabs"
}

// IsMessageSupported implements MessageHandler interface
func (m *TabsModel) IsMessageSupported(msg tea.Msg) bool {
	switch msg.(type) {
	case WindowResizeMsg, FocusChangeMsg, TabSwitchMsg:
		return true
	case tea.KeyMsg:
		return m.focused
	default:
		return false
	}
}

// Interface compliance
var _ MessageHandler = (*TabsModel)(nil)
var _ tea.Model = (*TabsModel)(nil)
