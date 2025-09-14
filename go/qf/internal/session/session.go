// Package session provides session management functionality for the qf application.
//
// This package handles the creation, persistence, and management of user sessions,
// which include filter sets, open file tabs, UI state, and session-specific settings.
// Sessions enable users to save their work and restore complete application state
// across application restarts.
package session

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// generateID generates a random UUID-like identifier
func generateID() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}

	// Format as UUID (8-4-4-4-12)
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // Set version to 4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // Set variant to 10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// Mode represents the application's modal state (Vim-style)
type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
)

// String returns the string representation of the Mode
func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "Normal"
	case ModeInsert:
		return "Insert"
	default:
		return "Unknown"
	}
}

// FocusedPane represents which pane currently has focus
type FocusedPane int

const (
	FocusedPaneViewer FocusedPane = iota
	FocusedPaneIncludeFilter
	FocusedPaneExcludeFilter
	FocusedPaneStatusBar
	FocusedPaneTabs
)

// String returns the string representation of the FocusedPane
func (f FocusedPane) String() string {
	switch f {
	case FocusedPaneViewer:
		return "Viewer"
	case FocusedPaneIncludeFilter:
		return "IncludeFilter"
	case FocusedPaneExcludeFilter:
		return "ExcludeFilter"
	case FocusedPaneStatusBar:
		return "StatusBar"
	case FocusedPaneTabs:
		return "Tabs"
	default:
		return "Unknown"
	}
}

// WindowSize represents terminal dimensions
type WindowSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// PaneLayout represents pane sizes and positions
type PaneLayout struct {
	FilterPaneHeight int  `json:"filter_pane_height"`
	ViewerHeight     int  `json:"viewer_height"`
	StatusBarHeight  int  `json:"status_bar_height"`
	TabBarVisible    bool `json:"tab_bar_visible"`
}

// UIState represents the interface state persistence
type UIState struct {
	Mode          Mode        `json:"mode"`
	FocusedPane   FocusedPane `json:"focused_pane"`
	WindowSize    WindowSize  `json:"window_size"`
	PaneLayout    PaneLayout  `json:"pane_layout"`
	StatusMessage string      `json:"status_message"`
}

// DefaultUIState returns a UIState with sensible defaults
func DefaultUIState() UIState {
	return UIState{
		Mode:        ModeNormal,
		FocusedPane: FocusedPaneViewer,
		WindowSize: WindowSize{
			Width:  80,
			Height: 24,
		},
		PaneLayout: PaneLayout{
			FilterPaneHeight: 5,
			ViewerHeight:     15,
			StatusBarHeight:  1,
			TabBarVisible:    false,
		},
		StatusMessage: "Ready",
	}
}

// FilterPatternType defines whether a pattern includes or excludes content
type FilterPatternType int

const (
	FilterInclude FilterPatternType = iota
	FilterExclude
)

// FilterPattern represents a compiled filter pattern with metadata
type FilterPattern struct {
	ID         string            `json:"id"`
	Expression string            `json:"expression"`
	Type       FilterPatternType `json:"type"`
	MatchCount int               `json:"match_count"`
	Color      string            `json:"color"`
	Created    time.Time         `json:"created"`
	IsValid    bool              `json:"is_valid"`
}

// FilterSet represents the current filter configuration
type FilterSet struct {
	Include []FilterPattern `json:"include"`
	Exclude []FilterPattern `json:"exclude"`
	Name    string          `json:"name"`
}

// FileTab represents a file tab in the application
type FileTab struct {
	ID       string    `json:"id"`
	FilePath string    `json:"file_path"`
	Content  []string  `json:"content,omitempty"` // Omit content for lighter persistence
	Active   bool      `json:"active"`
	Created  time.Time `json:"created"`
}

// SessionSettings represents session-specific settings
type SessionSettings struct {
	AutoSave          bool          `json:"auto_save"`
	AutoSaveInterval  time.Duration `json:"auto_save_interval"`
	MaxHistoryEntries int           `json:"max_history_entries"`
	EnableLogging     bool          `json:"enable_logging"`
}

// DefaultSessionSettings returns SessionSettings with sensible defaults
func DefaultSessionSettings() SessionSettings {
	return SessionSettings{
		AutoSave:          false,
		AutoSaveInterval:  5 * time.Minute,
		MaxHistoryEntries: 100,
		EnableLogging:     false,
	}
}

// Session represents a complete user session with all state
type Session struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	FilterSet      FilterSet       `json:"filter_set"`
	OpenFiles      []FileTab       `json:"open_files"`
	ActiveTabIndex int             `json:"active_tab_index"`
	UIState        UIState         `json:"ui_state"`
	Created        time.Time       `json:"created"`
	LastModified   time.Time       `json:"last_modified"`
	Settings       SessionSettings `json:"settings"`
	mutex          sync.RWMutex    `json:"-"` // Exclude from JSON serialization
}

// SessionInfo represents session metadata
type SessionInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"last_modified"`
	FilePath     string    `json:"file_path"`
	TabCount     int       `json:"tab_count"`
	FilterCount  int       `json:"filter_count"`
}

// NewSession creates a new session with the given name and default values
func NewSession(name string) *Session {
	now := time.Now()
	sessionID := generateID()

	return &Session{
		ID:             sessionID,
		Name:           name,
		FilterSet:      FilterSet{Name: name},
		OpenFiles:      make([]FileTab, 0),
		ActiveTabIndex: -1, // No active tab initially
		UIState:        DefaultUIState(),
		Created:        now,
		LastModified:   now,
		Settings:       DefaultSessionSettings(),
	}
}

// AddFileTab adds a new file tab to the session
func (s *Session) AddFileTab(filePath string, content []string) (*FileTab, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if file is already open
	for _, tab := range s.OpenFiles {
		if tab.FilePath == filePath {
			return nil, fmt.Errorf("file already open: %s", filePath)
		}
	}

	// Create new tab
	tab := FileTab{
		ID:       generateID(),
		FilePath: filePath,
		Content:  content,
		Active:   false, // Will be set active by SetActiveTab if needed
		Created:  time.Now(),
	}

	// If this is the first tab, make it active
	if len(s.OpenFiles) == 0 {
		tab.Active = true
		s.ActiveTabIndex = 0
		s.UIState.PaneLayout.TabBarVisible = false // Single tab doesn't need tab bar
	} else {
		s.UIState.PaneLayout.TabBarVisible = true // Multiple tabs need tab bar
	}

	// Add tab to session
	s.OpenFiles = append(s.OpenFiles, tab)

	s.LastModified = time.Now()
	return &tab, nil
}

// RemoveFileTab removes a file tab by ID
func (s *Session) RemoveFileTab(tabID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Find the tab index
	tabIndex := -1
	for i, tab := range s.OpenFiles {
		if tab.ID == tabID {
			tabIndex = i
			break
		}
	}

	if tabIndex == -1 {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Handle active tab adjustment
	wasActive := s.OpenFiles[tabIndex].Active

	// Remove the tab
	s.OpenFiles = append(s.OpenFiles[:tabIndex], s.OpenFiles[tabIndex+1:]...)

	// Adjust active tab index
	if wasActive {
		if len(s.OpenFiles) == 0 {
			s.ActiveTabIndex = -1
		} else if tabIndex < len(s.OpenFiles) {
			// Tab to the right becomes active
			s.ActiveTabIndex = tabIndex
			s.OpenFiles[tabIndex].Active = true
		} else {
			// Last tab was removed, make previous tab active
			s.ActiveTabIndex = len(s.OpenFiles) - 1
			s.OpenFiles[s.ActiveTabIndex].Active = true
		}
	} else if s.ActiveTabIndex > tabIndex {
		// Adjust index if removed tab was before active tab
		s.ActiveTabIndex--
	}

	// Update tab bar visibility
	s.UIState.PaneLayout.TabBarVisible = len(s.OpenFiles) > 1

	s.LastModified = time.Now()
	return nil
}

// SetActiveTab sets the active tab by index
func (s *Session) SetActiveTab(index int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if index < 0 || index >= len(s.OpenFiles) {
		return fmt.Errorf("invalid tab index: %d (valid range: 0-%d)", index, len(s.OpenFiles)-1)
	}

	// Deactivate current active tab
	if s.ActiveTabIndex >= 0 && s.ActiveTabIndex < len(s.OpenFiles) {
		s.OpenFiles[s.ActiveTabIndex].Active = false
	}

	// Activate new tab
	s.ActiveTabIndex = index
	s.OpenFiles[index].Active = true
	s.LastModified = time.Now()

	return nil
}

// GetActiveTab returns the currently active tab, or nil if none
func (s *Session) GetActiveTab() *FileTab {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.ActiveTabIndex < 0 || s.ActiveTabIndex >= len(s.OpenFiles) {
		return nil
	}

	return &s.OpenFiles[s.ActiveTabIndex]
}

// UpdateFilterSet updates the session's filter set
func (s *Session) UpdateFilterSet(filterSet FilterSet) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.FilterSet = filterSet
	s.LastModified = time.Now()
}

// UpdateUIState updates the session's UI state
func (s *Session) UpdateUIState(uiState UIState) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.UIState = uiState
	s.LastModified = time.Now()
}

// GetSessionInfo returns metadata about the session
func (s *Session) GetSessionInfo() SessionInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	filterCount := len(s.FilterSet.Include) + len(s.FilterSet.Exclude)

	return SessionInfo{
		ID:           s.ID,
		Name:         s.Name,
		Created:      s.Created,
		LastModified: s.LastModified,
		FilePath:     GetSessionPath(s.Name),
		TabCount:     len(s.OpenFiles),
		FilterCount:  filterCount,
	}
}

// Clone creates a deep copy of the session
func (s *Session) Clone() *Session {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Create new session with same data
	clone := &Session{
		ID:             generateID(),     // New ID for the clone
		Name:           s.Name + "-copy", // Modify name to avoid conflicts
		FilterSet:      s.FilterSet,
		OpenFiles:      make([]FileTab, len(s.OpenFiles)),
		ActiveTabIndex: s.ActiveTabIndex,
		UIState:        s.UIState,
		Created:        time.Now(),
		LastModified:   time.Now(),
		Settings:       s.Settings,
	}

	// Deep copy tabs
	copy(clone.OpenFiles, s.OpenFiles)
	for i := range clone.OpenFiles {
		clone.OpenFiles[i].ID = generateID() // New IDs for cloned tabs
		if clone.OpenFiles[i].Content != nil {
			clone.OpenFiles[i].Content = make([]string, len(s.OpenFiles[i].Content))
			copy(clone.OpenFiles[i].Content, s.OpenFiles[i].Content)
		}
	}

	// Deep copy filter patterns
	clone.FilterSet.Include = make([]FilterPattern, len(s.FilterSet.Include))
	copy(clone.FilterSet.Include, s.FilterSet.Include)
	for i := range clone.FilterSet.Include {
		clone.FilterSet.Include[i].ID = generateID() // New IDs for cloned patterns
	}

	clone.FilterSet.Exclude = make([]FilterPattern, len(s.FilterSet.Exclude))
	copy(clone.FilterSet.Exclude, s.FilterSet.Exclude)
	for i := range clone.FilterSet.Exclude {
		clone.FilterSet.Exclude[i].ID = generateID() // New IDs for cloned patterns
	}

	return clone
}

// GetSessionsDir returns the directory where sessions are stored
func GetSessionsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".config", "qf", "sessions")
	err = os.MkdirAll(sessionsDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return sessionsDir, nil
}

// GetSessionPath returns the file path for a session by name
func GetSessionPath(sessionName string) string {
	sessionsDir, err := GetSessionsDir()
	if err != nil {
		// Fallback to current directory if config dir fails
		return sessionName + ".qf-session"
	}

	return filepath.Join(sessionsDir, sessionName+".qf-session")
}

// SaveSession saves the session to disk with JSON persistence
func (s *Session) SaveSession() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sessionPath := GetSessionPath(s.Name)

	// Create backup if file exists
	if _, err := os.Stat(sessionPath); err == nil {
		backupPath := sessionPath + ".backup"
		if err := os.Rename(sessionPath, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Marshal session to JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Write to temporary file first (atomic write)
	tempPath := sessionPath + ".tmp"
	err = os.WriteFile(tempPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	// Atomic rename
	err = os.Rename(tempPath, sessionPath)
	if err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk by name
func LoadSession(sessionName string) (*Session, error) {
	sessionPath := GetSessionPath(sessionName)

	// Check if file exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("session not found: %s", sessionName)
	}

	// Read session file
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Unmarshal JSON
	var session Session
	err = json.Unmarshal(data, &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Validate session
	err = ValidateSession(&session)
	if err != nil {
		return nil, fmt.Errorf("invalid session data: %w", err)
	}

	// Migrate session if needed
	err = MigrateSession(&session)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate session: %w", err)
	}

	return &session, nil
}

// ValidateSession validates session integrity
func ValidateSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	if session.ID == "" {
		return fmt.Errorf("session ID is empty")
	}

	if session.Name == "" {
		return fmt.Errorf("session name is empty")
	}

	if session.Created.IsZero() {
		return fmt.Errorf("session creation time is zero")
	}

	if session.LastModified.IsZero() {
		return fmt.Errorf("session last modified time is zero")
	}

	// Validate active tab index
	if session.ActiveTabIndex < -1 || session.ActiveTabIndex >= len(session.OpenFiles) {
		return fmt.Errorf("invalid active tab index: %d", session.ActiveTabIndex)
	}

	// Validate tab consistency
	activeTabCount := 0
	for i, tab := range session.OpenFiles {
		if tab.ID == "" {
			return fmt.Errorf("tab %d has empty ID", i)
		}
		if tab.FilePath == "" {
			return fmt.Errorf("tab %d has empty file path", i)
		}
		if tab.Active {
			activeTabCount++
			if i != session.ActiveTabIndex {
				return fmt.Errorf("tab %d is marked active but ActiveTabIndex is %d", i, session.ActiveTabIndex)
			}
		}
	}

	if len(session.OpenFiles) > 0 && activeTabCount != 1 {
		return fmt.Errorf("expected exactly 1 active tab, found %d", activeTabCount)
	}

	// Validate filter patterns
	for i, pattern := range session.FilterSet.Include {
		if pattern.ID == "" {
			return fmt.Errorf("include pattern %d has empty ID", i)
		}
		if pattern.Expression == "" {
			return fmt.Errorf("include pattern %d has empty expression", i)
		}
	}

	for i, pattern := range session.FilterSet.Exclude {
		if pattern.ID == "" {
			return fmt.Errorf("exclude pattern %d has empty ID", i)
		}
		if pattern.Expression == "" {
			return fmt.Errorf("exclude pattern %d has empty expression", i)
		}
	}

	return nil
}

// MigrateSession migrates session data for version compatibility
func MigrateSession(session *Session) error {
	// For now, no migrations are needed since this is the initial version
	// Future versions can add migration logic here based on version fields

	// Ensure all required fields have sensible defaults if missing
	if session.Settings.AutoSaveInterval == 0 {
		session.Settings.AutoSaveInterval = 5 * time.Minute
	}

	if session.Settings.MaxHistoryEntries == 0 {
		session.Settings.MaxHistoryEntries = 100
	}

	// Ensure UI state has valid values
	if session.UIState.WindowSize.Width <= 0 {
		session.UIState.WindowSize.Width = 80
	}
	if session.UIState.WindowSize.Height <= 0 {
		session.UIState.WindowSize.Height = 24
	}

	if session.UIState.PaneLayout.FilterPaneHeight <= 0 {
		session.UIState.PaneLayout.FilterPaneHeight = 5
	}
	if session.UIState.PaneLayout.ViewerHeight <= 0 {
		session.UIState.PaneLayout.ViewerHeight = 15
	}
	if session.UIState.PaneLayout.StatusBarHeight <= 0 {
		session.UIState.PaneLayout.StatusBarHeight = 1
	}

	// Update tab bar visibility based on tab count
	session.UIState.PaneLayout.TabBarVisible = len(session.OpenFiles) > 1

	return nil
}
