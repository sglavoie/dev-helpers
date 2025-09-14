// Package ui provides the StatusBarModel component for displaying status information.
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// StatusBarModel displays status information at the bottom of the screen
type StatusBarModel struct {
	currentMode      Mode
	focusedComponent FocusedComponent
	message          string
	messageType      StatusType
	messageExpiry    time.Time
	width            int
	showHelp         bool
}

// NewStatusBarModel creates a new StatusBarModel
func NewStatusBarModel(mode Mode, focused FocusedComponent) *StatusBarModel {
	return &StatusBarModel{
		currentMode:      mode,
		focusedComponent: focused,
		message:          "Ready",
		messageType:      StatusInfo,
		messageExpiry:    time.Time{},
		width:            80,
		showHelp:         false,
	}
}

// Init implements tea.Model interface
func (m *StatusBarModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model interface
func (m *StatusBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WindowResizeMsg:
		m.width = msg.Width
		return m, nil

	case ModeTransitionMsg:
		m.currentMode = msg.NewMode
		return m, nil

	case FocusChangeMsg:
		m.focusedComponent = ParseFocusedComponent(msg.NewFocus)
		return m, nil

	case StatusUpdateMsg:
		m.message = msg.Message
		m.messageType = msg.MessageType
		// StatusUpdateMsg doesn't have duration, use default timeout for different types
		switch msg.MessageType {
		case StatusError:
			m.messageExpiry = time.Now().Add(5 * time.Second)
		case StatusWarning:
			m.messageExpiry = time.Now().Add(3 * time.Second)
		case StatusSuccess:
			m.messageExpiry = time.Now().Add(2 * time.Second)
		default:
			m.messageExpiry = time.Time{} // No expiry for info messages
		}
		return m, nil

	case ErrorMsg:
		m.message = msg.Message
		m.messageType = StatusError
		m.messageExpiry = time.Now().Add(5 * time.Second)
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		if key == "?" {
			m.showHelp = !m.showHelp
			return m, nil
		}
	}

	return m, nil
}

// View implements tea.Model interface
func (m *StatusBarModel) View() string {
	// Check if message has expired
	if !m.messageExpiry.IsZero() && time.Now().After(m.messageExpiry) {
		m.message = "Ready"
		m.messageType = StatusInfo
		m.messageExpiry = time.Time{}
	}

	if m.showHelp {
		return m.renderHelp()
	}

	return m.renderStatusBar()
}

// renderStatusBar renders the main status bar
func (m *StatusBarModel) renderStatusBar() string {
	var parts []string

	// Mode indicator
	modeStr := fmt.Sprintf("[%s]", m.currentMode.String())
	parts = append(parts, modeStr)

	// Component focus indicator
	focusStr := fmt.Sprintf("Focus:%s", m.focusedComponent.String())
	parts = append(parts, focusStr)

	// Message with type indicator
	messageStr := m.message
	switch m.messageType {
	case StatusError:
		messageStr = fmt.Sprintf("ERROR: %s", messageStr)
	case StatusWarning:
		messageStr = fmt.Sprintf("WARN: %s", messageStr)
	case StatusSuccess:
		messageStr = fmt.Sprintf("OK: %s", messageStr)
	}
	parts = append(parts, messageStr)

	// Join parts with separators
	statusContent := strings.Join(parts, " | ")

	// Add help indicator
	helpIndicator := "Press ? for help"

	// Calculate available space
	totalContentWidth := len(statusContent) + 3 + len(helpIndicator) // 3 for separator
	if totalContentWidth > m.width {
		// Truncate message if too long
		excess := totalContentWidth - m.width
		if len(messageStr) > excess {
			messageStr = messageStr[:len(messageStr)-excess-3] + "..."
			parts[len(parts)-1] = messageStr
			statusContent = strings.Join(parts, " | ")
		}
	}

	// Build final status bar
	result := statusContent

	// Add help indicator on the right if space allows
	usedWidth := len(result)
	if usedWidth+3+len(helpIndicator) <= m.width {
		padding := m.width - usedWidth - len(helpIndicator)
		if padding > 0 {
			result += strings.Repeat(" ", padding) + helpIndicator
		}
	}

	// Ensure status bar fills full width
	if len(result) < m.width {
		result += strings.Repeat(" ", m.width-len(result))
	}

	return result
}

// renderHelp renders the help overlay
func (m *StatusBarModel) renderHelp() string {
	var lines []string

	lines = append(lines, "=== QF HELP ===")
	lines = append(lines, "")

	// Mode-specific help
	switch m.currentMode {
	case ModeNormal:
		lines = append(lines, "NORMAL MODE:")
		lines = append(lines, "  i        - Enter insert mode")
		lines = append(lines, "  :        - Enter command mode")
		lines = append(lines, "  Tab      - Cycle focus between panes")
		lines = append(lines, "  j/k      - Move up/down in viewer")
		lines = append(lines, "  h/l      - Move left/right")
		lines = append(lines, "  g/G      - Go to top/bottom")
		lines = append(lines, "  /        - Search")
		lines = append(lines, "  n/N      - Next/previous search result")
		lines = append(lines, "  1-9      - Switch to tab 1-9")
		lines = append(lines, "  Ctrl+O   - Open file")
		lines = append(lines, "  Ctrl+W   - Close tab")
		lines = append(lines, "  Ctrl+S   - Save session")
		lines = append(lines, "  Ctrl+Q   - Quit")

	case ModeInsert:
		lines = append(lines, "INSERT MODE:")
		lines = append(lines, "  Esc      - Return to normal mode")
		lines = append(lines, "  Enter    - Confirm input")
		lines = append(lines, "  Backspace- Delete character")
		lines = append(lines, "  Tab      - Next field")

	case ModeCommand:
		lines = append(lines, "COMMAND MODE:")
		lines = append(lines, "  Esc      - Return to normal mode")
		lines = append(lines, "  Enter    - Execute command")
		lines = append(lines, "Commands:")
		lines = append(lines, "  :q       - Quit")
		lines = append(lines, "  :w       - Save session")
		lines = append(lines, "  :o <file>- Open file")
	}

	lines = append(lines, "")
	lines = append(lines, "Press ? again to close help")

	// Format help text to fit width
	var formattedLines []string
	for _, line := range lines {
		if len(line) > m.width-2 {
			// Wrap long lines
			for len(line) > 0 {
				if len(line) <= m.width-2 {
					formattedLines = append(formattedLines, line)
					break
				}
				formattedLines = append(formattedLines, line[:m.width-2])
				line = line[m.width-2:]
			}
		} else {
			// Pad short lines
			padded := line + strings.Repeat(" ", m.width-2-len(line))
			formattedLines = append(formattedLines, padded)
		}
	}

	// Join all lines
	return strings.Join(formattedLines, "\n")
}

// SetMessage sets a status message
func (m *StatusBarModel) SetMessage(message string, msgType StatusType, duration time.Duration) {
	m.message = message
	m.messageType = msgType
	if duration > 0 {
		m.messageExpiry = time.Now().Add(duration)
	} else {
		m.messageExpiry = time.Time{}
	}
}

// ClearMessage clears the current status message
func (m *StatusBarModel) ClearMessage() {
	m.message = "Ready"
	m.messageType = StatusInfo
	m.messageExpiry = time.Time{}
}

// GetCurrentMessage returns the current message and type
func (m *StatusBarModel) GetCurrentMessage() (string, StatusType) {
	return m.message, m.messageType
}

// MessageHandler interface implementation

// HandleMessage implements MessageHandler interface
func (m *StatusBarModel) HandleMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.Update(msg)
}

// GetComponentType implements MessageHandler interface
func (m *StatusBarModel) GetComponentType() string {
	return "status_bar"
}

// IsMessageSupported implements MessageHandler interface
func (m *StatusBarModel) IsMessageSupported(msg tea.Msg) bool {
	switch msg.(type) {
	case WindowResizeMsg, ModeTransitionMsg, FocusChangeMsg, StatusUpdateMsg, ErrorMsg:
		return true
	case tea.KeyMsg:
		// Status bar handles help toggle
		return true
	default:
		return false
	}
}

// Interface compliance
var _ MessageHandler = (*StatusBarModel)(nil)
var _ tea.Model = (*StatusBarModel)(nil)
