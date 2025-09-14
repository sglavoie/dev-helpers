// Package integration provides common types used across integration tests for the qf application.
//
// This file contains shared data structures and interfaces that are used by multiple
// integration tests to avoid code duplication and ensure consistency.
//
// These types serve as test contracts and will be replaced or unified with actual
// implementation types from internal/ packages once they are created.
package integration

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// FilterPatternType defines whether a pattern includes or excludes content
type FilterPatternType int

const (
	FilterInclude FilterPatternType = iota
	FilterExclude
)

// FilterPattern represents a compiled filter pattern with metadata
type FilterPattern struct {
	ID         string            // UUID for identification
	Expression string            // Raw regex pattern
	Type       FilterPatternType // Include or Exclude
	MatchCount int               // Usage statistics
	Color      string            // Highlighting color
	Created    time.Time         // Metadata
	IsValid    bool              // Compilation status
}

// FilterSet represents the current filter configuration
type FilterSet struct {
	Include []FilterPattern
	Exclude []FilterPattern
	Name    string
}

// Mode represents the application's modal state (Vim-style)
type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
)

// FileTab represents a file tab in the application
type FileTab struct {
	ID       string    // Unique tab identifier
	FilePath string    // Path to the file
	Content  []string  // File content lines
	Active   bool      // Whether this tab is currently active
	Created  time.Time // When the tab was created
}

// AppState represents the complete application state
type AppState struct {
	Tabs      []FileTab   // All open file tabs
	ActiveTab int         // Index of currently active tab
	FilterSet FilterSet   // Current filter configuration
	Mode      Mode        // Normal or Insert mode
	Session   SessionInfo // Current session information
}

// SessionInfo represents session metadata
type SessionInfo struct {
	Name     string    // Session name
	Created  time.Time // When session was created
	Modified time.Time // Last modification time
	FilePath string    // Path to session file
}

// Application defines the main application interface for integration testing
type Application interface {
	// File operations
	OpenFiles(filePaths []string) error
	GetTabs() []FileTab
	GetActiveTab() *FileTab
	CloseTab(tabID string) error

	// Tab navigation
	SwitchToTab(index int) error
	GetTabByIndex(index int) *FileTab

	// Filter operations
	AddFilter(pattern FilterPattern) error
	GetFilterSet() FilterSet
	ApplyFiltersToCurrentTab() error
	ApplyFiltersToAllTabs() error

	// Session management
	SaveSession(name string) error
	LoadSession(name string) error
	GetSessionInfo() SessionInfo

	// UI state
	GetAppState() AppState
	ProcessKeyPress(key tea.KeyMsg) (tea.Model, tea.Cmd)
}

// BasicFilteringApplication extends Application with basic filtering workflow methods
type BasicFilteringApplication interface {
	Application

	// Basic filtering workflow specific methods
	OpenFile(filePath string) error
	EnterInsertMode() error
	EnterNormalMode() error
	GetCurrentMode() Mode
	AddIncludePattern(pattern string) error
	AddExcludePattern(pattern string) error
	GetVisibleLines() []string
	GetCurrentState() AppState
	SendKeyPress(key tea.KeyMsg) error
	Shutdown() error
}
