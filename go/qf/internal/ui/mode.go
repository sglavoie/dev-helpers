// Package ui provides the modal state manager for the qf Interactive Log Filter Composer.
//
// This package implements the core modal interface state management following strict
// Vim-style discipline with Normal/Insert/Command modes and focus management between
// UI components. It provides the foundation for the interactive terminal interface.
package ui

import (
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ModeTransition represents a mode change with validation and context
type ModeTransition struct {
	From      Mode
	To        Mode
	Component FocusedComponent
	Allowed   bool
	Reason    string
}

// ModeConstraints defines which modes are allowed for each component
type ModeConstraints struct {
	AllowedModes    map[FocusedComponent][]Mode
	DefaultModes    map[FocusedComponent]Mode
	TransitionRules map[Mode][]Mode // Which modes can transition to which others
}

// ModeHistory tracks mode change history for debugging and statistics
type ModeHistory struct {
	Transitions []ModeTransition
	MaxEntries  int
	StartTime   time.Time
}

// ModeStatistics tracks usage statistics for modes and components
type ModeStatistics struct {
	ModeUsage      map[Mode]time.Duration             // Time spent in each mode
	ComponentFocus map[FocusedComponent]time.Duration // Time spent focused on each component
	Transitions    map[string]int                     // Count of transition types
	SessionStart   time.Time
	LastUpdate     time.Time
}

// ComponentRegistry manages registered components that can receive focus
type ComponentRegistry struct {
	components map[FocusedComponent]MessageHandler
	focusOrder []FocusedComponent
	mutex      sync.RWMutex
}

// ModeManager manages the modal interface state for the qf application
// It provides strict Vim-style mode discipline and focus management between UI components
type ModeManager struct {
	// Current state
	currentMode Mode
	prevMode    Mode
	focused     FocusedComponent
	prevFocused FocusedComponent

	// Mode management
	constraints ModeConstraints
	history     ModeHistory
	statistics  ModeStatistics
	components  ComponentRegistry

	// State tracking
	modeStartTime   time.Time
	focusStartTime  time.Time
	transitionCount int
	mutex           sync.RWMutex

	// Configuration
	maxHistoryEntries int
	trackStatistics   bool
	allowModeOverride bool
}

// ModeManagerConfig provides configuration options for the ModeManager
type ModeManagerConfig struct {
	MaxHistoryEntries int              // Maximum number of history entries to keep
	TrackStatistics   bool             // Whether to track usage statistics
	AllowModeOverride bool             // Allow forced mode transitions (for emergency recovery)
	InitialMode       Mode             // Initial mode on startup
	InitialFocus      FocusedComponent // Initial focus on startup
}

// NewModeManager creates a new ModeManager with default configuration
func NewModeManager() *ModeManager {
	return NewModeManagerWithConfig(ModeManagerConfig{
		MaxHistoryEntries: 100,
		TrackStatistics:   true,
		AllowModeOverride: false,
		InitialMode:       ModeNormal,
		InitialFocus:      FocusViewer,
	})
}

// NewModeManagerWithConfig creates a new ModeManager with custom configuration
func NewModeManagerWithConfig(config ModeManagerConfig) *ModeManager {
	now := time.Now()

	mm := &ModeManager{
		currentMode:       config.InitialMode,
		prevMode:          config.InitialMode,
		focused:           config.InitialFocus,
		prevFocused:       config.InitialFocus,
		modeStartTime:     now,
		focusStartTime:    now,
		maxHistoryEntries: config.MaxHistoryEntries,
		trackStatistics:   config.TrackStatistics,
		allowModeOverride: config.AllowModeOverride,

		constraints: ModeConstraints{
			AllowedModes: map[FocusedComponent][]Mode{
				FocusViewer:        {ModeNormal},
				FocusIncludeFilter: {ModeNormal, ModeInsert},
				FocusExcludeFilter: {ModeNormal, ModeInsert},
				FocusTabs:          {ModeNormal},
				FocusStatusBar:     {ModeNormal},
				FocusOverlay:       {ModeNormal, ModeInsert, ModeCommand},
			},
			DefaultModes: map[FocusedComponent]Mode{
				FocusViewer:        ModeNormal,
				FocusIncludeFilter: ModeNormal,
				FocusExcludeFilter: ModeNormal,
				FocusTabs:          ModeNormal,
				FocusStatusBar:     ModeNormal,
				FocusOverlay:       ModeNormal,
			},
			TransitionRules: map[Mode][]Mode{
				ModeNormal:  {ModeInsert, ModeCommand},
				ModeInsert:  {ModeNormal},
				ModeCommand: {ModeNormal},
			},
		},

		history: ModeHistory{
			Transitions: make([]ModeTransition, 0, config.MaxHistoryEntries),
			MaxEntries:  config.MaxHistoryEntries,
			StartTime:   now,
		},

		statistics: ModeStatistics{
			ModeUsage:      make(map[Mode]time.Duration),
			ComponentFocus: make(map[FocusedComponent]time.Duration),
			Transitions:    make(map[string]int),
			SessionStart:   now,
			LastUpdate:     now,
		},

		components: ComponentRegistry{
			components: make(map[FocusedComponent]MessageHandler),
			focusOrder: []FocusedComponent{
				FocusViewer,
				FocusIncludeFilter,
				FocusExcludeFilter,
				FocusTabs,
				FocusStatusBar,
			},
		},
	}

	return mm
}

// Mode Transition Methods

// EnterInsert transitions to Insert mode if allowed for the current component
func (mm *ModeManager) EnterInsert() tea.Cmd {
	return mm.transitionToMode(ModeInsert, "enter_insert")
}

// ExitToNormal transitions back to Normal mode from any other mode
func (mm *ModeManager) ExitToNormal() tea.Cmd {
	return mm.transitionToMode(ModeNormal, "exit_to_normal")
}

// EnterCommand transitions to Command mode if allowed for the current component
func (mm *ModeManager) EnterCommand() tea.Cmd {
	return mm.transitionToMode(ModeCommand, "enter_command")
}

// HandleEscape processes the Escape key, which always returns to Normal mode
func (mm *ModeManager) HandleEscape() tea.Cmd {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	// Special handling for Escape key - always goes to Normal mode
	// and may also change focus to viewer if in a filter pane
	cmds := []tea.Cmd{}

	if mm.currentMode != ModeNormal {
		cmds = append(cmds, mm.transitionToModeUnsafe(ModeNormal, "escape_key"))
	}

	// If focused on a filter pane and in Normal mode, return focus to viewer
	if mm.focused == FocusIncludeFilter || mm.focused == FocusExcludeFilter {
		cmds = append(cmds, mm.switchFocusUnsafe(FocusViewer, "escape_key"))
	}

	return tea.Batch(cmds...)
}

// transitionToMode safely transitions to a new mode with validation
func (mm *ModeManager) transitionToMode(newMode Mode, reason string) tea.Cmd {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	return mm.transitionToModeUnsafe(newMode, reason)
}

// transitionToModeUnsafe transitions to a mode without locking (internal use)
func (mm *ModeManager) transitionToModeUnsafe(newMode Mode, reason string) tea.Cmd {
	if !mm.isModeTransitionAllowed(mm.currentMode, newMode, mm.focused) && !mm.allowModeOverride {
		// Invalid transition - return error message
		return func() tea.Msg {
			return NewErrorMsg(
				fmt.Sprintf("Invalid mode transition from %s to %s for component %s",
					mm.currentMode.String(), newMode.String(), mm.focused.String()),
				"mode_transition",
				"mode_manager",
				true,
			)
		}
	}

	// Update statistics before transition
	if mm.trackStatistics {
		mm.updateModeStatistics()
	}

	// Record transition
	transition := ModeTransition{
		From:      mm.currentMode,
		To:        newMode,
		Component: mm.focused,
		Allowed:   true,
		Reason:    reason,
	}
	mm.addToHistory(transition)

	// Update state
	mm.prevMode = mm.currentMode
	mm.currentMode = newMode
	mm.modeStartTime = time.Now()
	mm.transitionCount++

	// Return mode transition message
	return func() tea.Msg {
		return NewModeTransitionMsg(newMode, mm.prevMode, fmt.Sprintf("%s:%s", mm.focused.String(), reason))
	}
}

// Focus Management Methods

// SwitchFocus changes focus to the specified component
func (mm *ModeManager) SwitchFocus(component FocusedComponent) tea.Cmd {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	return mm.switchFocusUnsafe(component, "explicit_switch")
}

// CycleFocus moves focus to the next component in the focus order
func (mm *ModeManager) CycleFocus() tea.Cmd {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	nextFocus := mm.getNextFocusComponent()
	return mm.switchFocusUnsafe(nextFocus, "tab_key")
}

// HandleTabKey processes the Tab key for focus cycling
func (mm *ModeManager) HandleTabKey() tea.Cmd {
	// Tab key behavior depends on current mode
	if mm.GetCurrentMode() != ModeNormal {
		// In Insert/Command mode, Tab might be handled by the component
		return nil
	}

	return mm.CycleFocus()
}

// switchFocusUnsafe changes focus without locking (internal use)
func (mm *ModeManager) switchFocusUnsafe(newFocus FocusedComponent, reason string) tea.Cmd {
	if newFocus == mm.focused {
		return nil // No change needed
	}

	// Update statistics before focus change
	if mm.trackStatistics {
		mm.updateFocusStatistics()
	}

	// Check if current mode is allowed for new component
	defaultMode := mm.constraints.DefaultModes[newFocus]
	cmds := []tea.Cmd{}

	if !mm.isModeAllowedForComponent(mm.currentMode, newFocus) {
		// Need to switch to default mode for the new component
		cmds = append(cmds, mm.transitionToModeUnsafe(defaultMode, "focus_change"))
	}

	// Update focus state
	mm.prevFocused = mm.focused
	mm.focused = newFocus
	mm.focusStartTime = time.Now()

	// Return focus change message
	cmds = append(cmds, func() tea.Msg {
		return NewFocusChangeMsg(newFocus.String(), mm.prevFocused.String(), reason)
	})

	return tea.Batch(cmds...)
}

// getNextFocusComponent returns the next component in the focus order
func (mm *ModeManager) getNextFocusComponent() FocusedComponent {
	for i, component := range mm.components.focusOrder {
		if component == mm.focused {
			nextIndex := (i + 1) % len(mm.components.focusOrder)
			return mm.components.focusOrder[nextIndex]
		}
	}
	// Fallback to first component if current focus not found
	return mm.components.focusOrder[0]
}

// State Query Methods

// GetCurrentMode returns the current mode
func (mm *ModeManager) GetCurrentMode() Mode {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.currentMode
}

// GetPreviousMode returns the previous mode
func (mm *ModeManager) GetPreviousMode() Mode {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.prevMode
}

// GetFocusedComponent returns the currently focused component
func (mm *ModeManager) GetFocusedComponent() FocusedComponent {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.focused
}

// GetPreviousFocus returns the previously focused component
func (mm *ModeManager) GetPreviousFocus() FocusedComponent {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.prevFocused
}

// IsComponentFocused returns true if the specified component is currently focused
func (mm *ModeManager) IsComponentFocused(component FocusedComponent) bool {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.focused == component
}

// Component Registration Methods

// RegisterComponent registers a component handler for focus management
func (mm *ModeManager) RegisterComponent(component FocusedComponent, handler MessageHandler) {
	mm.components.mutex.Lock()
	defer mm.components.mutex.Unlock()

	mm.components.components[component] = handler
}

// UnregisterComponent removes a component from focus management
func (mm *ModeManager) UnregisterComponent(component FocusedComponent) {
	mm.components.mutex.Lock()
	defer mm.components.mutex.Unlock()

	delete(mm.components.components, component)
}

// GetRegisteredComponent returns the handler for a component, if registered
func (mm *ModeManager) GetRegisteredComponent(component FocusedComponent) (MessageHandler, bool) {
	mm.components.mutex.RLock()
	defer mm.components.mutex.RUnlock()

	handler, exists := mm.components.components[component]
	return handler, exists
}

// SetFocusOrder sets the order in which components receive focus during cycling
func (mm *ModeManager) SetFocusOrder(order []FocusedComponent) {
	mm.components.mutex.Lock()
	defer mm.components.mutex.Unlock()

	// Validate that all components in order are registered
	for _, component := range order {
		if _, exists := mm.components.components[component]; !exists {
			// Skip unregistered components
			continue
		}
	}

	mm.components.focusOrder = order
}

// Validation Methods

// isModeTransitionAllowed checks if a mode transition is allowed
func (mm *ModeManager) isModeTransitionAllowed(from, to Mode, component FocusedComponent) bool {
	// Check if target mode is allowed for component
	if !mm.isModeAllowedForComponent(to, component) {
		return false
	}

	// Check transition rules
	allowedTransitions, exists := mm.constraints.TransitionRules[from]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == to {
			return true
		}
	}

	return false
}

// isModeAllowedForComponent checks if a mode is allowed for a specific component
func (mm *ModeManager) isModeAllowedForComponent(mode Mode, component FocusedComponent) bool {
	allowedModes, exists := mm.constraints.AllowedModes[component]
	if !exists {
		return false
	}

	for _, allowed := range allowedModes {
		if allowed == mode {
			return true
		}
	}

	return false
}

// CanEnterInsertMode returns true if Insert mode is allowed for current component
func (mm *ModeManager) CanEnterInsertMode() bool {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	return mm.isModeAllowedForComponent(ModeInsert, mm.focused)
}

// CanEnterCommandMode returns true if Command mode is allowed for current component
func (mm *ModeManager) CanEnterCommandMode() bool {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	return mm.isModeAllowedForComponent(ModeCommand, mm.focused)
}

// Statistics and History Methods

// GetStatistics returns current usage statistics
func (mm *ModeManager) GetStatistics() ModeStatistics {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// Update current session statistics
	if mm.trackStatistics {
		mm.updateStatisticsUnsafe()
	}

	// Return a copy of statistics
	stats := mm.statistics
	stats.ModeUsage = make(map[Mode]time.Duration)
	stats.ComponentFocus = make(map[FocusedComponent]time.Duration)
	stats.Transitions = make(map[string]int)

	for k, v := range mm.statistics.ModeUsage {
		stats.ModeUsage[k] = v
	}
	for k, v := range mm.statistics.ComponentFocus {
		stats.ComponentFocus[k] = v
	}
	for k, v := range mm.statistics.Transitions {
		stats.Transitions[k] = v
	}

	return stats
}

// GetHistory returns the mode transition history
func (mm *ModeManager) GetHistory() []ModeTransition {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// Return a copy of history
	history := make([]ModeTransition, len(mm.history.Transitions))
	copy(history, mm.history.Transitions)
	return history
}

// ClearHistory clears the mode transition history
func (mm *ModeManager) ClearHistory() {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	mm.history.Transitions = mm.history.Transitions[:0]
}

// ResetStatistics resets usage statistics
func (mm *ModeManager) ResetStatistics() {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	now := time.Now()
	mm.statistics = ModeStatistics{
		ModeUsage:      make(map[Mode]time.Duration),
		ComponentFocus: make(map[FocusedComponent]time.Duration),
		Transitions:    make(map[string]int),
		SessionStart:   now,
		LastUpdate:     now,
	}
	mm.modeStartTime = now
	mm.focusStartTime = now
}

// Internal helper methods

// addToHistory adds a transition to the history, maintaining size limits
func (mm *ModeManager) addToHistory(transition ModeTransition) {
	if len(mm.history.Transitions) >= mm.history.MaxEntries {
		// Remove oldest entry
		copy(mm.history.Transitions, mm.history.Transitions[1:])
		mm.history.Transitions = mm.history.Transitions[:len(mm.history.Transitions)-1]
	}

	mm.history.Transitions = append(mm.history.Transitions, transition)
}

// updateModeStatistics updates mode usage statistics
func (mm *ModeManager) updateModeStatistics() {
	if !mm.trackStatistics {
		return
	}

	now := time.Now()
	duration := now.Sub(mm.modeStartTime)
	mm.statistics.ModeUsage[mm.currentMode] += duration
	mm.statistics.LastUpdate = now
}

// updateFocusStatistics updates focus usage statistics
func (mm *ModeManager) updateFocusStatistics() {
	if !mm.trackStatistics {
		return
	}

	now := time.Now()
	duration := now.Sub(mm.focusStartTime)
	mm.statistics.ComponentFocus[mm.focused] += duration
	mm.statistics.LastUpdate = now
}

// updateStatisticsUnsafe updates all statistics without locking
func (mm *ModeManager) updateStatisticsUnsafe() {
	now := time.Now()

	// Update mode usage
	modeDuration := now.Sub(mm.modeStartTime)
	mm.statistics.ModeUsage[mm.currentMode] += modeDuration
	mm.modeStartTime = now

	// Update focus usage
	focusDuration := now.Sub(mm.focusStartTime)
	mm.statistics.ComponentFocus[mm.focused] += focusDuration
	mm.focusStartTime = now

	mm.statistics.LastUpdate = now
}

// Configuration Methods

// SetConstraints updates the mode constraints
func (mm *ModeManager) SetConstraints(constraints ModeConstraints) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	mm.constraints = constraints
}

// GetConstraints returns the current mode constraints
func (mm *ModeManager) GetConstraints() ModeConstraints {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	return mm.constraints
}

// SetTrackStatistics enables or disables statistics tracking
func (mm *ModeManager) SetTrackStatistics(track bool) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if mm.trackStatistics && !track {
		// Final update before disabling
		mm.updateStatisticsUnsafe()
	}

	mm.trackStatistics = track
}

// ForceMode forcibly sets the mode (emergency use only)
func (mm *ModeManager) ForceMode(mode Mode, reason string) tea.Cmd {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if !mm.allowModeOverride {
		return func() tea.Msg {
			return NewErrorMsg("Mode override not allowed", "force_mode", "mode_manager", false)
		}
	}

	return mm.transitionToModeUnsafe(mode, fmt.Sprintf("forced:%s", reason))
}

// ForceFocus forcibly sets the focus (emergency use only)
func (mm *ModeManager) ForceFocus(component FocusedComponent, reason string) tea.Cmd {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if !mm.allowModeOverride {
		return func() tea.Msg {
			return NewErrorMsg("Focus override not allowed", "force_focus", "mode_manager", false)
		}
	}

	return mm.switchFocusUnsafe(component, fmt.Sprintf("forced:%s", reason))
}
