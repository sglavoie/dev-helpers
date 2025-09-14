// Package ui implements the main application Bubble Tea model for qf Interactive Log Filter Composer.
//
// This package provides the AppModel that serves as the central orchestrator for all UI components,
// handling modal interface state, message routing, layout management, and integration with core services.
package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/dev-helpers/go/qf/internal/config"
	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
	"github.com/sglavoie/dev-helpers/go/qf/internal/file"
	"github.com/sglavoie/dev-helpers/go/qf/internal/session"
)

// AppModel is the main application model that orchestrates all UI components
// and manages global application state following Bubble Tea architecture patterns.
type AppModel struct {
	// Core application state
	width  int
	height int

	// Modal interface state
	currentMode Mode
	prevMode    Mode
	focused     FocusedComponent

	// Component models
	filterPane *FilterPaneModel
	viewer     *ViewerModel
	tabs       *TabsModel
	statusBar  *StatusBarModel
	overlay    *OverlayModel

	// Services integration
	filterEngine core.FilterEngine
	config       *config.Config
	session      *session.Session

	// Component registry for message propagation
	components map[string]MessageHandler
	mutex      sync.RWMutex

	// State management
	quitting          bool
	lastError         error
	keyBindings       map[string]KeyBinding
	configWatcher     *config.ConfigWatcher
	autoSaveTimer     *time.Timer
	backgroundContext context.Context
	backgroundCancel  context.CancelFunc

	// Layout state
	layoutInvalid bool
	styles        AppStyles
}

// AppStyles contains all styling for the application using Lipgloss
type AppStyles struct {
	Base           lipgloss.Style
	FocusedBorder  lipgloss.Style
	InactiveBorder lipgloss.Style
	StatusBar      lipgloss.Style
	ErrorStyle     lipgloss.Style
	HelpStyle      lipgloss.Style
	TitleStyle     lipgloss.Style
}

// NewAppModel creates a new AppModel with default configuration
func NewAppModel(initialFiles []string) *AppModel {
	// Load configuration
	cfg, err := config.LoadFromFile()
	if err != nil {
		// Use default configuration if loading fails
		cfg = config.NewDefaultConfig()
	}

	// Create background context for long-running operations
	bgCtx, bgCancel := context.WithCancel(context.Background())

	// Create new session
	sessionName := fmt.Sprintf("session-%d", time.Now().Unix())
	sess := session.NewSession(sessionName)

	// Create filter engine with configuration
	filterEngine := core.NewFilterEngine(
		core.WithDebounceDelay(time.Duration(cfg.Performance.DebounceDelayMs)*time.Millisecond),
		core.WithCacheSize(cfg.Performance.CacheSizeMb*1024*1024), // Convert MB to bytes
		core.WithMaxWorkers(cfg.Performance.MaxWorkers),
	)

	app := &AppModel{
		// Initialize with reasonable defaults
		width:  80,
		height: 24,

		// Start in Normal mode with viewer focused
		currentMode: ModeNormal,
		prevMode:    ModeNormal,
		focused:     FocusViewer,

		// Services
		filterEngine: filterEngine,
		config:       cfg,
		session:      sess,

		// State
		components:        make(map[string]MessageHandler),
		quitting:          false,
		keyBindings:       GetDefaultKeyBindings(),
		layoutInvalid:     true,
		backgroundContext: bgCtx,
		backgroundCancel:  bgCancel,
	}

	// Initialize styles based on configuration
	app.initStyles()

	// Initialize UI components
	app.initComponents()

	// Register components for message propagation
	app.registerComponents()

	// Load initial files if provided
	if len(initialFiles) > 0 {
		app.loadInitialFiles(initialFiles)
	}

	// Setup configuration watcher for hot-reload
	app.setupConfigWatcher()

	return app
}

// initStyles initializes application styles based on configuration
func (m *AppModel) initStyles() {
	theme := m.config.UI.Theme

	var (
		primaryColor   = lipgloss.Color("39")  // Blue
		secondaryColor = lipgloss.Color("36")  // Cyan
		errorColor     = lipgloss.Color("196") // Red
		borderColor    = lipgloss.Color("240") // Gray
		focusColor     = lipgloss.Color("39")  // Blue
	)

	// Adjust colors based on theme
	switch theme {
	case "dark":
		primaryColor = lipgloss.Color("75")   // Light blue
		secondaryColor = lipgloss.Color("80") // Light cyan
		borderColor = lipgloss.Color("243")   // Light gray
	case "light":
		primaryColor = lipgloss.Color("21")   // Dark blue
		secondaryColor = lipgloss.Color("24") // Dark cyan
		borderColor = lipgloss.Color("235")   // Dark gray
	}

	m.styles = AppStyles{
		Base: lipgloss.NewStyle().
			Padding(0).
			Margin(0),

		FocusedBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(focusColor).
			Padding(0, 1),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(primaryColor).
			Background(borderColor).
			Padding(0, 1).
			Width(80),

		ErrorStyle: lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true),

		HelpStyle: lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true),

		TitleStyle: lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Background(borderColor).
			Padding(0, 1),
	}
}

// initComponents initializes all UI component models
func (m *AppModel) initComponents() {
	// Create filter pane with current session's filter set
	m.filterPane = NewFilterPaneModel()

	// Create viewer with empty content initially
	m.viewer = NewViewerModel()

	// Create tabs model with session's open files
	m.tabs = NewTabsModel(m.session.OpenFiles)

	// Create status bar
	m.statusBar = NewStatusBarModel(m.currentMode, m.focused)

	// Create overlay (initially hidden)
	m.overlay = NewOverlayModel(m.filterEngine)
}

// registerComponents registers all components with the message propagation system
func (m *AppModel) registerComponents() {
	m.components["filter_pane"] = m.filterPane
	m.components["viewer"] = m.viewer
	m.components["tabs"] = m.tabs
	m.components["status_bar"] = m.statusBar
	m.components["overlay"] = m.overlay
}

// loadInitialFiles loads the provided files into tabs
func (m *AppModel) loadInitialFiles(filePaths []string) {
	for _, filePath := range filePaths {
		// Expand and validate file path using centralized utilities
		expandedPath := file.ExpandPath(filePath)

		if err := file.IsFileAccessible(expandedPath); err != nil {
			// Create error message for inaccessible file using centralized error handling
			m.lastError = fmt.Errorf("cannot access file %s: %w", filePath, err)
			continue
		}

		// Create new file tab
		fileTab := file.NewFileTab(expandedPath)

		// Add to session
		_, err := m.session.AddFileTab(expandedPath, []string{})
		if err != nil {
			m.lastError = fmt.Errorf("failed to add file tab: %w", err)
			continue
		}

		// Load file content in background
		go m.loadFileContent(fileTab)
	}
}

// loadFileContent loads file content asynchronously
func (m *AppModel) loadFileContent(fileTab *file.FileTab) {
	err := fileTab.LoadFromFile(m.backgroundContext)
	if err != nil {
		// This would normally send the error message through a channel or callback
		m.lastError = err
	} else {
		// Convert file.Line to []string for FileOpenMsg
		var lines []string
		for _, line := range fileTab.Content {
			lines = append(lines, line.Content)
		}

		// Send successful file load message
		fileMsg := NewFileOpenMsg(fileTab.Path, fileTab.ID, lines, true, nil)
		// This would normally send the message through a channel or callback
		_ = fileMsg // Placeholder for now
	}
}

// setupConfigWatcher sets up configuration hot-reload
func (m *AppModel) setupConfigWatcher() {
	configPath := config.GetConfigPath()
	m.configWatcher = config.NewConfigWatcher(configPath, func(newConfig *config.Config) {
		m.mutex.Lock()
		defer m.mutex.Unlock()

		oldConfig := m.config
		m.config = newConfig

		// Reinitialize styles if UI config changed
		if !m.uiConfigEqual(oldConfig.UI, newConfig.UI) {
			m.initStyles()
			m.layoutInvalid = true
		}

		// Update filter engine configuration if performance settings changed
		if !m.performanceConfigEqual(oldConfig.Performance, newConfig.Performance) {
			// Recreate filter engine with new settings
			m.filterEngine = core.NewFilterEngine(
				core.WithDebounceDelay(time.Duration(newConfig.Performance.DebounceDelayMs)*time.Millisecond),
				core.WithCacheSize(newConfig.Performance.CacheSizeMb*1024*1024),
				core.WithMaxWorkers(newConfig.Performance.MaxWorkers),
			)
		}
	})

	// Start watching in background
	go m.configWatcher.Watch()
}

// uiConfigEqual compares UI configurations
func (m *AppModel) uiConfigEqual(a, b config.UIConfig) bool {
	return a.Theme == b.Theme &&
		a.ShowLineNumbers == b.ShowLineNumbers &&
		len(a.HighlightColors) == len(b.HighlightColors)
}

// performanceConfigEqual compares performance configurations
func (m *AppModel) performanceConfigEqual(a, b config.PerformanceConfig) bool {
	return a.DebounceDelayMs == b.DebounceDelayMs &&
		a.CacheSizeMb == b.CacheSizeMb &&
		a.MaxWorkers == b.MaxWorkers
}

// Init implements tea.Model interface
func (m *AppModel) Init() tea.Cmd {
	// Initialize all child components
	var cmds []tea.Cmd

	// Initialize components
	if cmd := m.filterPane.Init(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.viewer.Init(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.tabs.Init(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.statusBar.Init(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.overlay.Init(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Setup auto-save timer if enabled
	if m.session.Settings.AutoSave {
		cmds = append(cmds, m.setupAutoSave())
	}

	// Initial status message
	cmds = append(cmds, func() tea.Msg {
		return NewStatusUpdateMsg("qf ready - Press 'i' for insert mode, ':' for command mode", StatusInfo, "startup_message")
	})

	return tea.Batch(cmds...)
}

// Update implements tea.Model interface with comprehensive message routing
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle built-in Bubble Tea messages
	if cmd := m.handleBuiltinMessages(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Handle key messages - early return to prevent double processing
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if cmd := m.handleKeyMessage(keyMsg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Handle custom UI messages
	if cmd := m.handleCustomMessages(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Update all child components
	childCmds := m.updateChildComponents(msg)
	cmds = append(cmds, childCmds...)

	return m, tea.Batch(cmds...)
}

// handleBuiltinMessages handles built-in Bubble Tea messages like window resize
func (m *AppModel) handleBuiltinMessages(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layoutInvalid = true
		// Propagate resize to all components
		return m.propagateToComponents(NewWindowResizeMsg(m.width, m.height))
	}
	return nil
}

// handleCustomMessages handles application-specific messages
func (m *AppModel) handleCustomMessages(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case ModeTransitionMsg:
		return m.handleModeTransition(msg)
	case FocusChangeMsg:
		return m.handleFocusChange(msg)
	case FilterUpdateMsg:
		return m.handleFilterUpdate(msg)
	case FileOpenMsg:
		return m.handleFileOpen(msg)
	case TabSwitchMsg:
		return m.handleTabSwitch(msg)
	case ErrorMsg:
		return m.handleError(msg)
	case StatusUpdateMsg:
		return m.handleStatusUpdate(msg)
	case ConfigReloadMsg:
		return m.handleConfigReload(msg)
	case QuitMsg:
		return m.handleQuit(msg)
	}
	return nil
}

// updateChildComponents updates all child components and collects their commands
func (m *AppModel) updateChildComponents(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd

	// Update filter pane
	if updatedFilterPane, cmd := m.filterPane.Update(msg); cmd != nil {
		m.filterPane = updatedFilterPane.(*FilterPaneModel)
		cmds = append(cmds, cmd)
	}

	// Update viewer
	if updatedViewer, cmd := m.viewer.Update(msg); cmd != nil {
		m.viewer = updatedViewer.(*ViewerModel)
		cmds = append(cmds, cmd)
	}

	// Update tabs
	if updatedTabs, cmd := m.tabs.Update(msg); cmd != nil {
		m.tabs = updatedTabs.(*TabsModel)
		cmds = append(cmds, cmd)
	}

	// Update status bar
	if updatedStatusBar, cmd := m.statusBar.Update(msg); cmd != nil {
		m.statusBar = updatedStatusBar.(*StatusBarModel)
		cmds = append(cmds, cmd)
	}

	// Update overlay
	if updatedOverlay, cmd := m.overlay.Update(msg); cmd != nil {
		m.overlay = updatedOverlay.(*OverlayModel)
		cmds = append(cmds, cmd)
	}

	return cmds
}

// handleKeyMessage processes key messages based on mode and focus
func (m *AppModel) handleKeyMessage(keyMsg tea.KeyMsg) tea.Cmd {
	key := keyMsg.String()

	// Global shortcuts that work in any mode
	switch key {
	case "ctrl+c", "ctrl+q":
		return func() tea.Msg {
			return NewQuitMsg("user_quit", true)
		}
	case "ctrl+s":
		return m.saveSession()
	}

	// Mode-specific key handling
	switch m.currentMode {
	case ModeNormal:
		return m.handleNormalModeKeys(key)
	case ModeInsert:
		return m.handleInsertModeKeys(key)
	case ModeCommand:
		return m.handleCommandModeKeys(key)
	}

	return nil
}

// handleNormalModeKeys handles keys in Normal mode
func (m *AppModel) handleNormalModeKeys(key string) tea.Cmd {
	switch key {
	case "esc":
		// Already in Normal mode, just ensure focus is on viewer
		if m.focused != FocusViewer {
			return func() tea.Msg {
				return NewFocusChangeMsg(FocusViewer.String(), m.focused.String(), "escape_key")
			}
		}
	case "i":
		// Enter Insert mode on focused component
		return func() tea.Msg {
			return NewModeTransitionMsg(ModeInsert, ModeNormal, "insert_key")
		}
	case ":":
		// Enter Command mode
		return func() tea.Msg {
			return NewModeTransitionMsg(ModeCommand, ModeNormal, "command_key")
		}
	case "tab":
		// Cycle focus
		return m.cycleFocus()
	case "h", "left":
		// Focus previous component or send to focused component
		return m.handleNavigationKey("left")
	case "l", "right":
		// Focus next component or send to focused component
		return m.handleNavigationKey("right")
	case "j", "down":
		return m.handleNavigationKey("down")
	case "k", "up":
		return m.handleNavigationKey("up")
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Switch to tab number
		return m.switchToTabNumber(key)
	case "ctrl+o":
		// Open file
		return m.openFileDialog()
	case "ctrl+w":
		// Close current tab
		return m.closeCurrentTab()
	}

	return nil
}

// handleInsertModeKeys handles keys in Insert mode
func (m *AppModel) handleInsertModeKeys(key string) tea.Cmd {
	switch key {
	case "esc":
		// Return to Normal mode
		return func() tea.Msg {
			return NewModeTransitionMsg(ModeNormal, ModeInsert, "escape_key")
		}
	case "tab":
		// In insert mode, tab moves to next field within the same component
		return nil // Let focused component handle
	default:
		// All other keys are handled by the focused component
		return nil
	}
}

// handleCommandModeKeys handles keys in Command mode
func (m *AppModel) handleCommandModeKeys(key string) tea.Cmd {
	switch key {
	case "esc":
		// Return to Normal mode
		return func() tea.Msg {
			return NewModeTransitionMsg(ModeNormal, ModeCommand, "escape_key")
		}
	case "enter":
		// Execute command and return to Normal mode
		return func() tea.Msg {
			return NewModeTransitionMsg(ModeNormal, ModeCommand, "command_executed")
		}
	default:
		// Let command input component handle other keys
		return nil
	}
}

// cycleFocus cycles focus between components
func (m *AppModel) cycleFocus() tea.Cmd {
	focusOrder := []FocusedComponent{
		FocusViewer,
		FocusIncludeFilter,
		FocusExcludeFilter,
		FocusTabs,
		FocusStatusBar,
	}

	currentIndex := 0
	for i, component := range focusOrder {
		if component == m.focused {
			currentIndex = i
			break
		}
	}

	nextIndex := (currentIndex + 1) % len(focusOrder)
	nextFocus := focusOrder[nextIndex]

	return func() tea.Msg {
		return NewFocusChangeMsg(nextFocus.String(), m.focused.String(), "tab_key")
	}
}

// handleNavigationKey handles navigation keys
func (m *AppModel) handleNavigationKey(direction string) tea.Cmd {
	// In Normal mode, navigation keys can change focus or be sent to components
	switch m.focused {
	case FocusViewer:
		// Viewer handles its own navigation
		return nil
	case FocusIncludeFilter, FocusExcludeFilter:
		if direction == "left" || direction == "right" {
			// Switch between filter panes
			newFocus := FocusIncludeFilter
			if m.focused == FocusIncludeFilter {
				newFocus = FocusExcludeFilter
			}
			return func() tea.Msg {
				return NewFocusChangeMsg(newFocus.String(), m.focused.String(), "navigation_key")
			}
		}
		return nil
	case FocusTabs:
		if direction == "left" || direction == "right" {
			// Let tabs component handle tab switching
			return nil
		}
		return nil
	default:
		return nil
	}
}

// switchToTabNumber switches to a specific tab by number
func (m *AppModel) switchToTabNumber(key string) tea.Cmd {
	tabNum := int(key[0] - '0') // Convert character to number
	if tabNum < 1 || tabNum > len(m.session.OpenFiles) {
		return func() tea.Msg {
			return NewErrorMsg(
				fmt.Sprintf("Tab %d does not exist", tabNum),
				"tab_switching",
				"app",
				true,
			)
		}
	}

	tabIndex := tabNum - 1
	currentTab := m.session.GetActiveTab()
	newTab := &m.session.OpenFiles[tabIndex]

	currentTabID := ""
	if currentTab != nil {
		currentTabID = currentTab.ID
	}

	return func() tea.Msg {
		return NewTabSwitchMsg(newTab.ID, currentTabID, tabIndex, "number_key")
	}
}

// Message handlers for different message types

func (m *AppModel) handleModeTransition(msg ModeTransitionMsg) tea.Cmd {
	m.prevMode = m.currentMode
	m.currentMode = msg.NewMode

	// Update status bar with new mode
	statusMsg := NewStatusUpdateMsg(
		fmt.Sprintf("Mode: %s", m.currentMode.String()),
		StatusInfo,
		"mode_transition",
	)

	return func() tea.Msg { return statusMsg }
}

func (m *AppModel) handleFocusChange(msg FocusChangeMsg) tea.Cmd {
	m.focused = ParseFocusedComponent(msg.NewFocus)
	m.layoutInvalid = true // Redraw with new focus indicators

	return nil
}

func (m *AppModel) handleFilterUpdate(msg FilterUpdateMsg) tea.Cmd {
	// Convert ui.FilterSet to session.FilterSet
	sessionFilterSet := m.convertToSessionFilterSet(msg.FilterSet)
	// Update session with new filter set
	m.session.UpdateFilterSet(sessionFilterSet)

	// Clear and add patterns to filter engine
	m.filterEngine.ClearPatterns()

	// Convert core.Pattern to core.FilterPattern
	for _, pattern := range msg.FilterSet.Include {
		filterPattern := core.FilterPattern{
			ID:         pattern.ID,
			Expression: pattern.Expression,
			Type:       core.FilterInclude,
			MatchCount: pattern.MatchCount,
			Color:      pattern.Color,
			Created:    pattern.Created,
			IsValid:    pattern.IsValid,
		}
		m.filterEngine.AddPattern(filterPattern)
	}

	for _, pattern := range msg.FilterSet.Exclude {
		filterPattern := core.FilterPattern{
			ID:         pattern.ID,
			Expression: pattern.Expression,
			Type:       core.FilterExclude,
			MatchCount: pattern.MatchCount,
			Color:      pattern.Color,
			Created:    pattern.Created,
			IsValid:    pattern.IsValid,
		}
		m.filterEngine.AddPattern(filterPattern)
	}

	// Apply filters to current content
	return m.applyFiltersToCurrentContent()
}

func (m *AppModel) handleFileOpen(msg FileOpenMsg) tea.Cmd {
	if !msg.Success {
		return func() tea.Msg {
			return NewErrorMsg(
				fmt.Sprintf("Failed to open file: %s", msg.Error.Error()),
				"file_opening",
				"file_loader",
				true,
			)
		}
	}

	// File loaded successfully, update viewer with content
	// This would typically involve updating the viewer model
	statusMsg := NewStatusUpdateMsg(
		fmt.Sprintf("Loaded %s (%d lines)", filepath.Base(msg.FilePath), len(msg.Content)),
		StatusSuccess,
		"file_loaded",
	)

	return func() tea.Msg { return statusMsg }
}

func (m *AppModel) handleTabSwitch(msg TabSwitchMsg) tea.Cmd {
	err := m.session.SetActiveTab(msg.NewTabIndex)
	if err != nil {
		return func() tea.Msg {
			return NewErrorMsg(
				fmt.Sprintf("Failed to switch tab: %s", err.Error()),
				"tab_switching",
				"app",
				true,
			)
		}
	}

	// Update layout to reflect new active tab
	m.layoutInvalid = true

	return nil
}

func (m *AppModel) handleError(msg ErrorMsg) tea.Cmd {
	m.lastError = fmt.Errorf("%s: %s", msg.Context, msg.Message)

	// Show error in status bar
	statusMsg := NewStatusUpdateMsg(msg.Message, StatusError, "error_handler")

	return func() tea.Msg { return statusMsg }
}

func (m *AppModel) handleStatusUpdate(msg StatusUpdateMsg) tea.Cmd {
	// Status updates are handled by the status bar component
	return nil
}

func (m *AppModel) handleConfigReload(msg ConfigReloadMsg) tea.Cmd {
	if msg.Success {
		statusMsg := NewStatusUpdateMsg("Configuration reloaded", StatusSuccess, "config_reload")
		return func() tea.Msg { return statusMsg }
	} else {
		errorMsg := NewErrorMsg(
			fmt.Sprintf("Failed to reload configuration: %s", msg.Error.Error()),
			"config_reload",
			"config_watcher",
			true,
		)
		return func() tea.Msg { return errorMsg }
	}
}

func (m *AppModel) handleQuit(msg QuitMsg) tea.Cmd {
	m.quitting = true

	if msg.SaveFirst {
		// Save session before quitting
		if err := m.session.SaveSession(); err != nil {
			m.lastError = err
		}
	}

	// Cancel background operations
	m.backgroundCancel()

	// Stop config watcher
	if m.configWatcher != nil {
		m.configWatcher.Stop()
	}

	return tea.Quit
}

// Utility functions

func (m *AppModel) propagateToComponents(msg tea.Msg) tea.Cmd {
	// This is a simple implementation. A more sophisticated version
	// would use the MessagePropagator interface
	var cmds []tea.Cmd

	for _, component := range m.components {
		if component.IsMessageSupported(msg) {
			_, cmd := component.HandleMessage(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return tea.Batch(cmds...)
}

func (m *AppModel) saveSession() tea.Cmd {
	return func() tea.Msg {
		err := m.session.SaveSession()
		return NewSessionSaveMsg(m.session.Name, err == nil, err)
	}
}

func (m *AppModel) applyFiltersToCurrentContent() tea.Cmd {
	activeTab := m.session.GetActiveTab()
	if activeTab == nil {
		return nil
	}

	// Get content from active tab
	// This would require converting file.Line to []string
	var lines []string
	// For now, return empty command - this would be implemented
	// when the file tab content is properly integrated
	_ = lines

	return nil
}

func (m *AppModel) setupAutoSave() tea.Cmd {
	interval := m.session.Settings.AutoSaveInterval
	return tea.Tick(interval, func(time.Time) tea.Msg {
		err := m.session.SaveSession()
		return NewSessionSaveMsg(m.session.Name, err == nil, err)
	})
}

func (m *AppModel) openFileDialog() tea.Cmd {
	// This would typically open a file picker overlay
	// For now, return a placeholder command
	return nil
}

func (m *AppModel) closeCurrentTab() tea.Cmd {
	activeTab := m.session.GetActiveTab()
	if activeTab == nil {
		return func() tea.Msg {
			return NewErrorMsg("No active tab to close", "tab_management", "app", true)
		}
	}

	return func() tea.Msg {
		err := m.session.RemoveFileTab(activeTab.ID)
		if err != nil {
			return NewErrorMsg(
				fmt.Sprintf("Failed to close tab: %s", err.Error()),
				"tab_management",
				"app",
				true,
			)
		}
		return NewStatusUpdateMsg("Tab closed", StatusInfo, "tab_management")
	}
}

// View implements tea.Model interface with comprehensive layout management
func (m *AppModel) View() string {
	if m.quitting {
		return "Saving session and exiting...\n"
	}

	// Recalculate layout if needed
	if m.layoutInvalid {
		m.calculateLayout()
		m.layoutInvalid = false
	}

	// Build the layout using Lipgloss
	var sections []string

	// Title bar (if space allows)
	if m.height > 10 {
		title := m.renderTitleBar()
		sections = append(sections, title)
	}

	// Tab bar (if multiple tabs are open)
	if len(m.session.OpenFiles) > 1 {
		tabBar := m.renderTabBar()
		sections = append(sections, tabBar)
	}

	// Main content area (split between filter pane and viewer)
	mainContent := m.renderMainContent()
	sections = append(sections, mainContent)

	// Status bar
	statusBar := m.renderStatusBar()
	sections = append(sections, statusBar)

	// Overlay (if visible)
	if m.overlay.IsVisible() {
		overlay := m.overlay.View()
		// Position overlay in center
		overlayView := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			overlay,
		)
		return overlayView
	}

	// Join all sections
	return strings.Join(sections, "\n")
}

func (m *AppModel) renderTitleBar() string {
	sessionName := m.session.Name
	if m.lastError != nil {
		sessionName += " (ERROR)"
	}

	title := fmt.Sprintf(" qf - %s ", sessionName)

	return m.styles.TitleStyle.
		Width(m.width).
		Render(title)
}

func (m *AppModel) renderTabBar() string {
	if m.tabs == nil {
		return ""
	}

	tabsView := m.tabs.View()
	return lipgloss.NewStyle().
		Width(m.width).
		Render(tabsView)
}

func (m *AppModel) renderMainContent() string {
	// Calculate available height for main content
	usedHeight := 1 // Status bar
	if m.height > 10 {
		usedHeight += 1 // Title bar
	}
	if len(m.session.OpenFiles) > 1 {
		usedHeight += 1 // Tab bar
	}

	availableHeight := m.height - usedHeight
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Split height between filter pane and viewer
	var filterPaneHeight int
	showLineNumbers := m.config.GetUISettings().ShowLineNumbers // Use config for dynamic sizing
	if showLineNumbers {
		filterPaneHeight = availableHeight / 3 // 1/3 for filters
	} else {
		filterPaneHeight = availableHeight / 4 // 1/4 for filters
	}

	viewerHeight := availableHeight - filterPaneHeight

	// Render filter pane
	var filterPaneView string
	if m.filterPane != nil {
		// Create side-by-side filter panes
		includeView := m.renderFilterPane("include", filterPaneHeight)
		excludeView := m.renderFilterPane("exclude", filterPaneHeight)

		filterPaneView = lipgloss.JoinHorizontal(
			lipgloss.Top,
			includeView,
			excludeView,
		)
	}

	// Render viewer
	var viewerView string
	if m.viewer != nil {
		style := m.styles.InactiveBorder
		if m.focused == FocusViewer {
			style = m.styles.FocusedBorder
		}

		viewerContent := m.viewer.View()
		viewerView = style.
			Width(m.width - 2).
			Height(viewerHeight).
			Render(viewerContent)
	}

	// Combine filter pane and viewer vertically
	return lipgloss.JoinVertical(
		lipgloss.Left,
		filterPaneView,
		viewerView,
	)
}

func (m *AppModel) renderFilterPane(paneType string, height int) string {
	if m.filterPane == nil {
		return ""
	}

	// Determine focus state
	focused := (paneType == "include" && m.focused == FocusIncludeFilter) ||
		(paneType == "exclude" && m.focused == FocusExcludeFilter)

	style := m.styles.InactiveBorder
	if focused {
		style = m.styles.FocusedBorder
	}

	// Get content from filter pane component
	content := m.filterPane.View()

	title := strings.ToUpper(paneType) + " PATTERNS"
	titledContent := fmt.Sprintf("%s\n%s", title, content)

	return style.
		Width(m.width/2 - 1).
		Height(height).
		Render(titledContent)
}

func (m *AppModel) renderStatusBar() string {
	if m.statusBar == nil {
		return ""
	}

	statusContent := m.statusBar.View()
	return m.styles.StatusBar.
		Width(m.width).
		Render(statusContent)
}

func (m *AppModel) calculateLayout() {
	// Layout calculations would be performed here
	// This is where responsive layout logic would go
}

// convertToSessionFilterSet converts ui.FilterSet to session.FilterSet
func (m *AppModel) convertToSessionFilterSet(uiFilterSet FilterSet) session.FilterSet {
	sessionFilterSet := session.FilterSet{
		Name:    uiFilterSet.Name,
		Include: make([]session.FilterPattern, len(uiFilterSet.Include)),
		Exclude: make([]session.FilterPattern, len(uiFilterSet.Exclude)),
	}

	// Convert include patterns
	for i, pattern := range uiFilterSet.Include {
		sessionFilterSet.Include[i] = session.FilterPattern{
			ID:         pattern.ID,
			Expression: pattern.Expression,
			Type:       session.FilterInclude,
			MatchCount: pattern.MatchCount,
			Color:      pattern.Color,
			Created:    pattern.Created,
			IsValid:    pattern.IsValid,
		}
	}

	// Convert exclude patterns
	for i, pattern := range uiFilterSet.Exclude {
		sessionFilterSet.Exclude[i] = session.FilterPattern{
			ID:         pattern.ID,
			Expression: pattern.Expression,
			Type:       session.FilterExclude,
			MatchCount: pattern.MatchCount,
			Color:      pattern.Color,
			Created:    pattern.Created,
			IsValid:    pattern.IsValid,
		}
	}

	return sessionFilterSet
}

// Cleanup handles application shutdown
func (m *AppModel) Cleanup() {
	// Cancel background operations
	if m.backgroundCancel != nil {
		m.backgroundCancel()
	}

	// Stop config watcher
	if m.configWatcher != nil {
		m.configWatcher.Stop()
	}

	// Stop auto-save timer
	if m.autoSaveTimer != nil {
		m.autoSaveTimer.Stop()
	}

	// Save session if auto-save is enabled
	if m.session.Settings.AutoSave {
		m.session.SaveSession()
	}
}

// Interface compliance checks
var (
	_ tea.Model = (*AppModel)(nil)
)
