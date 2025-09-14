// Package ui provides shared message types for the qf application's UI components.
//
// This package defines the message types used for communication between Bubble Tea
// components in the interactive log filter composer. All message types implement
// the tea.Msg interface and follow the established contracts from the UI tests.
package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/dev-helpers/go/qf/internal/config"
	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
)

// FilterUpdateMsg represents filter changes that need to propagate across components.
// This message triggers real-time filter application throughout the UI.
type FilterUpdateMsg struct {
	FilterSet FilterSet
	Source    string // Component that originated the update
	Timestamp time.Time
}

// FilterSet represents the current filter configuration
type FilterSet struct {
	Include []core.FilterPattern // Include patterns using OR logic
	Exclude []core.FilterPattern // Exclude patterns using veto logic
	Name    string               // Session identifier
}

// ModeTransitionMsg handles transitions between Normal and Insert modes (Vim-style)
type ModeTransitionMsg struct {
	NewMode   Mode
	PrevMode  Mode
	Context   string // Which component triggered the transition
	Timestamp time.Time
}

// Mode represents the current UI mode
type Mode int

const (
	// ModeNormal represents the normal mode for navigation and commands
	ModeNormal Mode = iota
	// ModeInsert represents the insert mode for text editing
	ModeInsert
	// ModeCommand represents command mode (file operations, configuration)
	ModeCommand
)

// String returns a string representation of the Mode
func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "NORMAL"
	case ModeInsert:
		return "INSERT"
	case ModeCommand:
		return "COMMAND"
	default:
		return "UNKNOWN"
	}
}

// ErrorMsg represents user-facing errors with context and recovery information
type ErrorMsg struct {
	Message     string // User-friendly error message
	Context     string // Context where error occurred
	Recoverable bool   // Whether the user can recover from this error
	Timestamp   time.Time
	Source      string // Component that generated the error
}

// FileOpenMsg handles file loading operations
type FileOpenMsg struct {
	FilePath string   // Path to the file being opened
	Content  []string // File content lines (nil if failed)
	TabID    string   // Unique tab identifier
	Success  bool     // Whether the operation succeeded
	Error    error    // Error details if failed
}

// ContentUpdateMsg notifies components that displayed content has changed
type ContentUpdateMsg struct {
	TabID         string                   // Which file tab was updated
	FilteredLines []string                 // Lines that passed filters
	LineNumbers   []int                    // Original line numbers of filtered lines
	Highlights    map[int][]core.Highlight // Highlighting information per line
	Stats         core.FilterStats         // Filter performance statistics
}

// ViewportUpdateMsg handles viewport and scroll changes
type ViewportUpdateMsg struct {
	TabID          string // Which file tab
	ScrollPosition int    // Current scroll position
	CursorLine     int    // Current cursor line (1-based)
	ViewportHeight int    // Visible lines count
	Source         string // Component that triggered the update
}

// SearchMsg handles search operations within the viewer
type SearchMsg struct {
	Pattern       string // Search pattern
	CaseSensitive bool   // Case sensitive search
	Direction     SearchDirection
	TabID         string // Which file tab to search
}

// SearchDirection indicates search direction
type SearchDirection int

const (
	SearchForward SearchDirection = iota
	SearchBackward
)

// SearchResultMsg returns search results
type SearchResultMsg struct {
	Pattern      string        // Original search pattern
	Matches      []SearchMatch // Found matches
	CurrentMatch int           // Index of current match
	TabID        string        // Which file tab
	Total        int           // Total matches found
}

// SearchMatch represents a single search match
type SearchMatch struct {
	LineNumber int    // Line number (1-based)
	Start      int    // Start position in line
	End        int    // End position in line
	Context    string // Surrounding context
}

// StatusUpdateMsg updates the status bar information
type StatusUpdateMsg struct {
	Message     string // Status message to display
	MessageType StatusType
	Timestamp   time.Time
	Context     string // Additional context information
}

// StatusType indicates the type of status message
type StatusType int

const (
	StatusInfo StatusType = iota
	StatusWarning
	StatusError
	StatusSuccess
)

// FocusMsg handles focus changes between components (legacy)
type FocusMsg struct {
	Component string // Component to focus
	PrevFocus string // Previously focused component
	Reason    string // Reason for focus change
}

// FocusChangeMsg represents a focus transition between components
type FocusChangeMsg struct {
	NewFocus  string    // Component receiving focus
	PrevFocus string    // Component losing focus
	Reason    string    // Reason for focus change (tab_key, escape_key, etc.)
	Timestamp time.Time // When the focus change occurred
}

// ResizeMsg handles terminal resize events
type ResizeMsg struct {
	Width  int
	Height int
}

// WindowResizeMsg handles terminal resize events (alias for ResizeMsg)
type WindowResizeMsg struct {
	Width  int
	Height int
}

// ConfigReloadMsg handles configuration reload events
type ConfigReloadMsg struct {
	ConfigPath string
	Success    bool
	Error      error
	Timestamp  time.Time
}

// ConfigUpdateMsg handles configuration updates for components
type ConfigUpdateMsg struct {
	Config          *config.Config // New configuration
	OldConfig       *config.Config // Previous configuration for comparison
	UpdatedSections []string       // Which configuration sections changed
	Source          string         // Source of the update (file_watch, manual_reload, etc.)
	Timestamp       time.Time      // When the update occurred
	Partial         bool           // Whether this is a partial configuration update
}

// QuitMsg handles application quit events
type QuitMsg struct {
	Reason    string // Reason for quitting (user_quit, error, etc.)
	Graceful  bool   // Whether to quit gracefully
	SaveFirst bool   // Whether to save session before quitting
}

// TabSwitchMsg handles tab switching events
type TabSwitchMsg struct {
	NewTabID      string // Tab ID to switch to
	PreviousTabID string // Previous tab ID
	NewTabIndex   int    // Tab index to switch to
	Context       string // Context information (e.g., "tab_left", "number_key")
}

// SessionSaveMsg handles session save events
type SessionSaveMsg struct {
	SessionName string // Name of the session
	Success     bool   // Whether save was successful
	Error       error  // Error if save failed
}

// Note: TabSwitchMsg is defined in keyboard.go

// MessageHandler defines the contract for components that handle UI messages
type MessageHandler interface {
	tea.Model
	HandleMessage(msg tea.Msg) (tea.Model, tea.Cmd)
	GetComponentType() string
	IsMessageSupported(msg tea.Msg) bool
}

// MessagePropagator defines the contract for message propagation between components
type MessagePropagator interface {
	PropagateMessage(msg tea.Msg, targetComponents []string) tea.Cmd
	RegisterComponent(name string, handler MessageHandler)
	UnregisterComponent(name string)
}

// Helper functions for message creation

// NewFilterUpdateMsg creates a new filter update message
func NewFilterUpdateMsg(filterSet FilterSet, source string) FilterUpdateMsg {
	return FilterUpdateMsg{
		FilterSet: filterSet,
		Source:    source,
		Timestamp: time.Now(),
	}
}

// NewModeTransitionMsg creates a new mode transition message
func NewModeTransitionMsg(newMode, prevMode Mode, context string) ModeTransitionMsg {
	return ModeTransitionMsg{
		NewMode:   newMode,
		PrevMode:  prevMode,
		Context:   context,
		Timestamp: time.Now(),
	}
}

// NewErrorMsg creates a new error message
func NewErrorMsg(message, context, source string, recoverable bool) ErrorMsg {
	return ErrorMsg{
		Message:     message,
		Context:     context,
		Recoverable: recoverable,
		Timestamp:   time.Now(),
		Source:      source,
	}
}

// NewFileOpenMsg creates a new file open message
func NewFileOpenMsg(filePath, tabID string, content []string, success bool, err error) FileOpenMsg {
	return FileOpenMsg{
		FilePath: filePath,
		Content:  content,
		TabID:    tabID,
		Success:  success,
		Error:    err,
	}
}

// NewContentUpdateMsg creates a new content update message
func NewContentUpdateMsg(tabID string, result core.FilterResult) ContentUpdateMsg {
	return ContentUpdateMsg{
		TabID:         tabID,
		FilteredLines: result.MatchedLines,
		LineNumbers:   result.LineNumbers,
		Highlights:    result.MatchHighlights,
		Stats:         result.Stats,
	}
}

// NewStatusUpdateMsg creates a new status update message
func NewStatusUpdateMsg(message string, msgType StatusType, context string) StatusUpdateMsg {
	return StatusUpdateMsg{
		Message:     message,
		MessageType: msgType,
		Timestamp:   time.Now(),
		Context:     context,
	}
}

// NewWindowResizeMsg creates a new window resize message
func NewWindowResizeMsg(width, height int) WindowResizeMsg {
	return WindowResizeMsg{
		Width:  width,
		Height: height,
	}
}

// NewConfigReloadMsg creates a new config reload message
func NewConfigReloadMsg(configPath string, success bool, err error) ConfigReloadMsg {
	return ConfigReloadMsg{
		ConfigPath: configPath,
		Success:    success,
		Error:      err,
		Timestamp:  time.Now(),
	}
}

// NewConfigUpdateMsg creates a new config update message
func NewConfigUpdateMsg(newConfig, oldConfig *config.Config, updatedSections []string, source string, partial bool) ConfigUpdateMsg {
	return ConfigUpdateMsg{
		Config:          newConfig,
		OldConfig:       oldConfig,
		UpdatedSections: updatedSections,
		Source:          source,
		Timestamp:       time.Now(),
		Partial:         partial,
	}
}

// NewQuitMsg creates a new quit message
func NewQuitMsg(reason string, graceful bool) QuitMsg {
	return QuitMsg{
		Reason:    reason,
		Graceful:  graceful,
		SaveFirst: false,
	}
}

// NewTabSwitchMsg creates a new tab switch message
func NewTabSwitchMsg(newTabID, previousTabID string, newTabIndex int, context string) TabSwitchMsg {
	return TabSwitchMsg{
		NewTabID:      newTabID,
		PreviousTabID: previousTabID,
		NewTabIndex:   newTabIndex,
		Context:       context,
	}
}

// NewSessionSaveMsg creates a new session save message
func NewSessionSaveMsg(sessionName string, success bool, err error) SessionSaveMsg {
	return SessionSaveMsg{
		SessionName: sessionName,
		Success:     success,
		Error:       err,
	}
}

// NewFocusChangeMsg creates a new focus change message
func NewFocusChangeMsg(newFocus, prevFocus string, reason string) FocusChangeMsg {
	return FocusChangeMsg{
		NewFocus:  newFocus,
		PrevFocus: prevFocus,
		Reason:    reason,
		Timestamp: time.Now(),
	}
}
