// Package ui implements the keyboard shortcut handler for qf Interactive Log Filter Composer.
//
// This package provides a comprehensive keyboard input system that supports vim-style modal interface,
// component-specific key bindings, customizable shortcuts, and contextual help generation. It integrates
// with the existing Bubble Tea architecture and message system.
package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// KeyHandler provides mode-aware keyboard input handling with component routing
type KeyHandler struct {
	// Current application state
	currentMode Mode
	focused     FocusedComponent

	// Key binding mappings
	normalBindings  map[string]KeyBinding
	insertBindings  map[string]KeyBinding
	commandBindings map[string]KeyBinding
	globalBindings  map[string]KeyBinding

	// Component-specific bindings
	componentBindings map[FocusedComponent]map[string]KeyBinding

	// Configuration and customization
	config      KeyboardConfig
	customBinds map[string]string

	// State management
	mutex       sync.RWMutex
	lastKeyTime time.Time
	keySequence string

	// Help system integration
	helpContext map[Mode][]KeyHelpEntry
	contextHelp map[FocusedComponent][]KeyHelpEntry
}

// KeyBinding represents a keyboard shortcut with its associated action
type KeyBinding struct {
	// Shortcut definition
	Key         string    // Key combination (e.g., "ctrl+c", "j", "gg")
	Description string    // Human-readable description
	Action      KeyAction // Action to perform
	Context     string    // Context where this binding is active

	// Behavior configuration
	Mode      Mode             // Mode where this binding is active
	Component FocusedComponent // Component where this binding is active (empty for global)
	Priority  int              // Priority for conflict resolution (higher wins)

	// Advanced features
	Repeatable  bool           // Whether this action can be repeated with numeric prefix
	RequiresArg bool           // Whether this action requires additional input
	Conditions  []KeyCondition // Conditions that must be met for activation
}

// KeyAction defines the type of action a key binding should perform
type KeyAction struct {
	Type      ActionType       // Type of action to perform
	Command   string           // Command to execute (for command actions)
	Message   tea.Msg          // Message to send (for message actions)
	Handler   KeyActionHandler // Custom handler function (for custom actions)
	Component FocusedComponent // Target component for the action
}

// ActionType defines the different types of actions that can be performed
type ActionType int

const (
	// Navigation actions
	ActionNavigateUp ActionType = iota
	ActionNavigateDown
	ActionNavigateLeft
	ActionNavigateRight
	ActionPageUp
	ActionPageDown
	ActionGoToTop
	ActionGoToBottom
	ActionGoToLine

	// Mode transitions
	ActionEnterInsertMode
	ActionEnterNormalMode
	ActionEnterCommandMode

	// Pane management
	ActionFocusNext
	ActionFocusPrev
	ActionExpandPane
	ActionCollapsePane
	ActionTogglePane

	// Pattern management
	ActionAddPattern
	ActionDeletePattern
	ActionCopyPattern
	ActionPastePattern
	ActionClearPattern

	// File operations
	ActionOpenFile
	ActionCloseTab
	ActionSwitchTab
	ActionNextTab
	ActionPrevTab

	// Search operations
	ActionSearch
	ActionSearchNext
	ActionSearchPrev
	ActionClearSearch

	// Application control
	ActionQuit
	ActionSave
	ActionHelp
	ActionShowShortcuts
	ActionCommand

	// Custom actions
	ActionCustom
	ActionMessage
)

// KeyActionHandler defines a custom key action handler function
type KeyActionHandler func(context KeyContext) tea.Cmd

// KeyContext provides context information for key action handlers
type KeyContext struct {
	Mode      Mode
	Component FocusedComponent
	KeyMsg    tea.KeyMsg
	Sequence  string
	Args      map[string]interface{}
}

// KeyCondition defines a condition that must be met for a key binding to activate
type KeyCondition struct {
	Type      ConditionType
	Component FocusedComponent
	Value     interface{}
	Negate    bool
}

// ConditionType defines the types of conditions that can be checked
type ConditionType int

const (
	ConditionMode ConditionType = iota
	ConditionFocus
	ConditionHasContent
	ConditionHasSelection
	ConditionCustom
)

// FocusedComponent represents the currently focused UI component
type FocusedComponent int

const (
	FocusViewer FocusedComponent = iota
	FocusIncludeFilter
	FocusExcludeFilter
	FocusTabs
	FocusStatusBar
	FocusOverlay
	FocusGlobal // For global actions that work regardless of focus
)

// String returns a string representation of FocusedComponent
func (f FocusedComponent) String() string {
	switch f {
	case FocusViewer:
		return "viewer"
	case FocusIncludeFilter:
		return "include_pane"
	case FocusExcludeFilter:
		return "exclude_pane"
	case FocusTabs:
		return "tabs"
	case FocusStatusBar:
		return "status_bar"
	case FocusOverlay:
		return "overlay"
	case FocusGlobal:
		return "global"
	default:
		return "unknown"
	}
}

// ComponentName returns the component name as expected by the message system
func (f FocusedComponent) ComponentName() string {
	return f.String()
}

// ParseFocusedComponent parses a string back to a FocusedComponent
func ParseFocusedComponent(s string) FocusedComponent {
	switch s {
	case "viewer":
		return FocusViewer
	case "include_pane":
		return FocusIncludeFilter
	case "exclude_pane":
		return FocusExcludeFilter
	case "tabs":
		return FocusTabs
	case "status_bar":
		return FocusStatusBar
	case "overlay":
		return FocusOverlay
	case "global":
		return FocusGlobal
	default:
		return FocusViewer // Default to viewer
	}
}

// KeyHelpEntry represents a help entry for keyboard shortcuts
type KeyHelpEntry struct {
	Category    string // Help category (e.g., "Navigation", "Editing")
	Key         string // Key combination display
	Description string // Action description
	Context     string // When this shortcut is available
}

// KeyboardConfig defines configuration options for keyboard handling
type KeyboardConfig struct {
	// Timing configuration
	SequenceTimeoutMs int // Time to wait for multi-key sequences
	RepeatDelayMs     int // Delay before key repeat kicks in
	RepeatRateMs      int // Rate of key repetition

	// Behavior configuration
	EnableVimBindings   bool // Enable vim-style key bindings
	CaseSensitive       bool // Whether key bindings are case sensitive
	AllowCustomBindings bool // Whether custom bindings are allowed

	// Help system configuration
	ShowHelpInStatusBar bool   // Show context help in status bar
	HelpOverlayStyle    string // Style for help overlay
}

// KeyHandlerMsg represents keyboard handler messages
type KeyHandlerMsg struct {
	Action  ActionType
	Context KeyContext
	Result  interface{}
	Error   error
}

// NewKeyHandler creates a new keyboard handler with default configuration
func NewKeyHandler(config KeyboardConfig) *KeyHandler {
	kh := &KeyHandler{
		currentMode:       ModeNormal,
		focused:           FocusViewer,
		normalBindings:    make(map[string]KeyBinding),
		insertBindings:    make(map[string]KeyBinding),
		commandBindings:   make(map[string]KeyBinding),
		globalBindings:    make(map[string]KeyBinding),
		componentBindings: make(map[FocusedComponent]map[string]KeyBinding),
		config:            config,
		customBinds:       make(map[string]string),
		helpContext:       make(map[Mode][]KeyHelpEntry),
		contextHelp:       make(map[FocusedComponent][]KeyHelpEntry),
		lastKeyTime:       time.Now(),
	}

	// Initialize component binding maps
	for comp := FocusViewer; comp <= FocusOverlay; comp++ {
		kh.componentBindings[comp] = make(map[string]KeyBinding)
	}

	// Load default key bindings
	kh.loadDefaultBindings()

	return kh
}

// HandleKey processes a keyboard input and returns appropriate commands
func (kh *KeyHandler) HandleKey(keyMsg tea.KeyMsg, mode Mode, focused FocusedComponent) tea.Cmd {
	kh.mutex.Lock()
	defer kh.mutex.Unlock()

	kh.currentMode = mode
	kh.focused = focused

	// Build key string from the tea.KeyMsg
	keyStr := kh.keyMsgToString(keyMsg)
	if keyStr == "" {
		return nil
	}

	// Handle multi-key sequences
	now := time.Now()
	if now.Sub(kh.lastKeyTime) > time.Duration(kh.config.SequenceTimeoutMs)*time.Millisecond {
		kh.keySequence = ""
	}
	kh.lastKeyTime = now

	// Add current key to sequence
	if kh.keySequence != "" {
		kh.keySequence += keyStr
	} else {
		kh.keySequence = keyStr
	}

	// Try to match key bindings
	binding, matched := kh.findBinding(kh.keySequence)
	if matched {
		kh.keySequence = "" // Reset sequence on successful match
		return kh.executeBinding(binding, KeyContext{
			Mode:      mode,
			Component: focused,
			KeyMsg:    keyMsg,
			Sequence:  kh.keySequence,
		})
	}

	// Check if this could be part of a longer sequence
	if kh.hasPotentialMatch(kh.keySequence) {
		// Wait for more keys
		return nil
	}

	// No match found, reset sequence and try single key
	kh.keySequence = keyStr
	binding, matched = kh.findBinding(keyStr)
	if matched {
		kh.keySequence = ""
		return kh.executeBinding(binding, KeyContext{
			Mode:      mode,
			Component: focused,
			KeyMsg:    keyMsg,
			Sequence:  keyStr,
		})
	}

	// Handle default actions for unbound keys
	return kh.handleUnboundKey(keyMsg, mode, focused)
}

// SetMode updates the current mode
func (kh *KeyHandler) SetMode(mode Mode) {
	kh.mutex.Lock()
	defer kh.mutex.Unlock()
	kh.currentMode = mode
}

// SetFocus updates the currently focused component
func (kh *KeyHandler) SetFocus(component FocusedComponent) {
	kh.mutex.Lock()
	defer kh.mutex.Unlock()
	kh.focused = component
}

// AddBinding adds or updates a key binding
func (kh *KeyHandler) AddBinding(binding KeyBinding) error {
	kh.mutex.Lock()
	defer kh.mutex.Unlock()

	if binding.Key == "" {
		return fmt.Errorf("key binding cannot have empty key")
	}

	// Validate the binding
	if err := kh.validateBinding(binding); err != nil {
		return fmt.Errorf("invalid key binding: %w", err)
	}

	// Add to appropriate map
	targetMap := kh.getBindingMap(binding.Mode)
	if binding.Component != FocusGlobal {
		// Component-specific binding
		if kh.componentBindings[binding.Component] == nil {
			kh.componentBindings[binding.Component] = make(map[string]KeyBinding)
		}
		kh.componentBindings[binding.Component][binding.Key] = binding
	} else {
		// Mode or global binding
		targetMap[binding.Key] = binding
	}

	// Update help context
	kh.updateHelpContext(binding)

	return nil
}

// RemoveBinding removes a key binding
func (kh *KeyHandler) RemoveBinding(key string, mode Mode, component FocusedComponent) {
	kh.mutex.Lock()
	defer kh.mutex.Unlock()

	if component != FocusGlobal {
		if bindings, exists := kh.componentBindings[component]; exists {
			delete(bindings, key)
		}
	} else {
		targetMap := kh.getBindingMap(mode)
		delete(targetMap, key)
	}

	// Refresh help context
	kh.refreshHelpContext()
}

// GetHelp returns context-sensitive help information
func (kh *KeyHandler) GetHelp(mode Mode, component FocusedComponent) []KeyHelpEntry {
	kh.mutex.RLock()
	defer kh.mutex.RUnlock()

	var help []KeyHelpEntry

	// Add mode-specific help
	if modeHelp, exists := kh.helpContext[mode]; exists {
		help = append(help, modeHelp...)
	}

	// Add component-specific help
	if compHelp, exists := kh.contextHelp[component]; exists {
		help = append(help, compHelp...)
	}

	return help
}

// GetAllBindings returns all current key bindings for debugging/configuration
func (kh *KeyHandler) GetAllBindings() map[string][]KeyBinding {
	kh.mutex.RLock()
	defer kh.mutex.RUnlock()

	result := make(map[string][]KeyBinding)

	// Add mode bindings
	result["normal"] = kh.mapToSlice(kh.normalBindings)
	result["insert"] = kh.mapToSlice(kh.insertBindings)
	result["command"] = kh.mapToSlice(kh.commandBindings)
	result["global"] = kh.mapToSlice(kh.globalBindings)

	// Add component bindings
	for comp, bindings := range kh.componentBindings {
		result[comp.String()] = kh.mapToSlice(bindings)
	}

	return result
}

// LoadCustomBindings loads custom key bindings from configuration
func (kh *KeyHandler) LoadCustomBindings(customBindings map[string]string) error {
	kh.mutex.Lock()
	defer kh.mutex.Unlock()

	for key, action := range customBindings {
		binding, err := kh.parseCustomBinding(key, action)
		if err != nil {
			return fmt.Errorf("failed to parse custom binding %s: %w", key, err)
		}

		// Store custom binding
		kh.customBinds[key] = action

		// Add to appropriate binding map
		if err := kh.addBindingUnsafe(binding); err != nil {
			return fmt.Errorf("failed to add custom binding %s: %w", key, err)
		}
	}

	return nil
}

// Private helper methods

func (kh *KeyHandler) loadDefaultBindings() {
	// Normal mode bindings - Navigation
	kh.mustAddBinding(KeyBinding{
		Key: "j", Description: "Move cursor down", Mode: ModeNormal,
		Action: KeyAction{Type: ActionNavigateDown}, Priority: 100,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "k", Description: "Move cursor up", Mode: ModeNormal,
		Action: KeyAction{Type: ActionNavigateUp}, Priority: 100,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "h", Description: "Show help", Mode: ModeNormal,
		Action: KeyAction{Type: ActionHelp}, Priority: 100,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "l", Description: "Move cursor right", Mode: ModeNormal,
		Action: KeyAction{Type: ActionNavigateRight}, Priority: 100,
	})

	// Page navigation
	kh.mustAddBinding(KeyBinding{
		Key: "ctrl+d", Description: "Page down", Mode: ModeNormal,
		Action: KeyAction{Type: ActionPageDown}, Priority: 100,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "ctrl+u", Description: "Page up", Mode: ModeNormal,
		Action: KeyAction{Type: ActionPageUp}, Priority: 100,
	})

	// Go to top/bottom
	kh.mustAddBinding(KeyBinding{
		Key: "gg", Description: "Go to top", Mode: ModeNormal,
		Action: KeyAction{Type: ActionGoToTop}, Priority: 100,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "G", Description: "Go to bottom", Mode: ModeNormal,
		Action: KeyAction{Type: ActionGoToBottom}, Priority: 100,
	})

	// Pane control
	kh.mustAddBinding(KeyBinding{
		Key: "tab", Description: "Cycle through panes", Mode: ModeNormal, Component: FocusGlobal,
		Action: KeyAction{Type: ActionFocusNext}, Priority: 200,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "m", Description: "Expand/collapse pane", Mode: ModeNormal,
		Action: KeyAction{Type: ActionTogglePane}, Priority: 100,
	})

	// Mode transitions
	kh.mustAddBinding(KeyBinding{
		Key: "i", Description: "Enter insert mode", Mode: ModeNormal,
		Action: KeyAction{Type: ActionEnterInsertMode}, Priority: 200,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "esc", Description: "Enter normal mode", Mode: ModeInsert, Component: FocusGlobal,
		Action: KeyAction{Type: ActionEnterNormalMode}, Priority: 200,
	})
	kh.mustAddBinding(KeyBinding{
		Key: ":", Description: "Enter command mode", Mode: ModeNormal,
		Action: KeyAction{Type: ActionEnterCommandMode}, Priority: 200,
	})

	// Pattern management - works on both include and exclude filter panes
	kh.mustAddBinding(KeyBinding{
		Key: "a", Description: "Add pattern", Mode: ModeNormal, Component: FocusIncludeFilter,
		Action: KeyAction{Type: ActionAddPattern}, Priority: 150,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "a", Description: "Add pattern", Mode: ModeNormal, Component: FocusExcludeFilter,
		Action: KeyAction{Type: ActionAddPattern}, Priority: 150,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "d", Description: "Delete pattern", Mode: ModeNormal, Component: FocusIncludeFilter,
		Action: KeyAction{Type: ActionDeletePattern}, Priority: 150,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "d", Description: "Delete pattern", Mode: ModeNormal, Component: FocusExcludeFilter,
		Action: KeyAction{Type: ActionDeletePattern}, Priority: 150,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "y", Description: "Copy pattern", Mode: ModeNormal, Component: FocusIncludeFilter,
		Action: KeyAction{Type: ActionCopyPattern}, Priority: 150,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "y", Description: "Copy pattern", Mode: ModeNormal, Component: FocusExcludeFilter,
		Action: KeyAction{Type: ActionCopyPattern}, Priority: 150,
	})

	// File operations
	kh.mustAddBinding(KeyBinding{
		Key: "o", Description: "Open file", Mode: ModeNormal,
		Action: KeyAction{Type: ActionOpenFile}, Priority: 150,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "w", Description: "Close current tab", Mode: ModeNormal, Component: FocusTabs,
		Action: KeyAction{Type: ActionCloseTab}, Priority: 150,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "ctrl+s", Description: "Save session", Mode: ModeNormal, Component: FocusGlobal,
		Action: KeyAction{Type: ActionSave}, Priority: 200,
	})

	// Tab navigation with number keys
	for i := 1; i <= 9; i++ {
		num := strconv.Itoa(i)
		kh.mustAddBinding(KeyBinding{
			Key: num, Description: fmt.Sprintf("Switch to tab %d", i), Mode: ModeNormal,
			Action: KeyAction{Type: ActionSwitchTab, Command: num}, Priority: 150,
		})
	}

	// Search operations
	kh.mustAddBinding(KeyBinding{
		Key: "n", Description: "Next search match", Mode: ModeNormal, Component: FocusViewer,
		Action: KeyAction{Type: ActionSearchNext}, Priority: 150,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "N", Description: "Previous search match", Mode: ModeNormal, Component: FocusViewer,
		Action: KeyAction{Type: ActionSearchPrev}, Priority: 150,
	})

	// Application control
	kh.mustAddBinding(KeyBinding{
		Key: "q", Description: "Quit application", Mode: ModeNormal, Component: FocusGlobal,
		Action: KeyAction{Type: ActionQuit}, Priority: 200,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "?", Description: "Show keyboard shortcuts", Mode: ModeNormal, Component: FocusGlobal,
		Action: KeyAction{Type: ActionShowShortcuts}, Priority: 200,
	})

	// Insert mode bindings
	kh.mustAddBinding(KeyBinding{
		Key: "enter", Description: "Apply pattern", Mode: ModeInsert,
		Action: KeyAction{Type: ActionCustom, Handler: kh.defaultInsertModeHandler}, Priority: 150,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "ctrl+a", Description: "Move to line start", Mode: ModeInsert,
		Action: KeyAction{Type: ActionCustom, Handler: kh.defaultInsertModeHandler}, Priority: 100,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "ctrl+e", Description: "Move to line end", Mode: ModeInsert,
		Action: KeyAction{Type: ActionCustom, Handler: kh.defaultInsertModeHandler}, Priority: 100,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "ctrl+w", Description: "Delete word", Mode: ModeInsert,
		Action: KeyAction{Type: ActionCustom, Handler: kh.defaultInsertModeHandler}, Priority: 100,
	})

	// Arrow key navigation
	kh.mustAddBinding(KeyBinding{
		Key: "up", Description: "History previous", Mode: ModeInsert,
		Action: KeyAction{Type: ActionCustom, Handler: kh.defaultInsertModeHandler}, Priority: 100,
	})
	kh.mustAddBinding(KeyBinding{
		Key: "down", Description: "History next", Mode: ModeInsert,
		Action: KeyAction{Type: ActionCustom, Handler: kh.defaultInsertModeHandler}, Priority: 100,
	})
}

// mustAddBinding adds a binding and panics on error (for default bindings)
func (kh *KeyHandler) mustAddBinding(binding KeyBinding) {
	if err := kh.AddBinding(binding); err != nil {
		panic(fmt.Sprintf("failed to add default binding %s: %v", binding.Key, err))
	}
}

// defaultInsertModeHandler provides default handling for insert mode actions
func (kh *KeyHandler) defaultInsertModeHandler(context KeyContext) tea.Cmd {
	switch context.KeyMsg.String() {
	case "enter":
		return tea.Cmd(func() tea.Msg {
			return KeyActionMsg{Action: ActionCustom, Context: context}
		})
	case "ctrl+a", "ctrl+e", "ctrl+w":
		return tea.Cmd(func() tea.Msg {
			return KeyActionMsg{Action: ActionCustom, Context: context}
		})
	case "up", "down":
		return tea.Cmd(func() tea.Msg {
			return KeyActionMsg{Action: ActionCustom, Context: context}
		})
	default:
		return nil
	}
}

// keyMsgToString converts a tea.KeyMsg to a string representation
func (kh *KeyHandler) keyMsgToString(keyMsg tea.KeyMsg) string {
	switch keyMsg.Type {
	case tea.KeyRunes:
		return string(keyMsg.Runes)
	case tea.KeySpace:
		return "space"
	case tea.KeyEnter:
		return "enter"
	case tea.KeyTab:
		return "tab"
	case tea.KeyBackspace:
		return "backspace"
	case tea.KeyDelete:
		return "delete"
	case tea.KeyEsc:
		return "esc"
	case tea.KeyUp:
		return "up"
	case tea.KeyDown:
		return "down"
	case tea.KeyLeft:
		return "left"
	case tea.KeyRight:
		return "right"
	case tea.KeyHome:
		return "home"
	case tea.KeyEnd:
		return "end"
	case tea.KeyCtrlA:
		return "ctrl+a"
	case tea.KeyCtrlB:
		return "ctrl+b"
	case tea.KeyCtrlC:
		return "ctrl+c"
	case tea.KeyCtrlD:
		return "ctrl+d"
	case tea.KeyCtrlE:
		return "ctrl+e"
	case tea.KeyCtrlF:
		return "ctrl+f"
	case tea.KeyCtrlG:
		return "ctrl+g"
	case tea.KeyCtrlH:
		return "ctrl+h"
	case tea.KeyCtrlJ:
		return "ctrl+j"
	case tea.KeyCtrlK:
		return "ctrl+k"
	case tea.KeyCtrlL:
		return "ctrl+l"
	case tea.KeyCtrlN:
		return "ctrl+n"
	case tea.KeyCtrlO:
		return "ctrl+o"
	case tea.KeyCtrlP:
		return "ctrl+p"
	case tea.KeyCtrlQ:
		return "ctrl+q"
	case tea.KeyCtrlR:
		return "ctrl+r"
	case tea.KeyCtrlS:
		return "ctrl+s"
	case tea.KeyCtrlT:
		return "ctrl+t"
	case tea.KeyCtrlU:
		return "ctrl+u"
	case tea.KeyCtrlV:
		return "ctrl+v"
	case tea.KeyCtrlW:
		return "ctrl+w"
	case tea.KeyCtrlX:
		return "ctrl+x"
	case tea.KeyCtrlY:
		return "ctrl+y"
	case tea.KeyCtrlZ:
		return "ctrl+z"
	default:
		return ""
	}
}

// findBinding searches for a key binding in the appropriate maps
func (kh *KeyHandler) findBinding(keyStr string) (KeyBinding, bool) {
	// Check component-specific bindings first (highest priority)
	if bindings, exists := kh.componentBindings[kh.focused]; exists {
		if binding, found := bindings[keyStr]; found && kh.checkConditions(binding) {
			return binding, true
		}
	}

	// Check mode-specific bindings
	modeMap := kh.getBindingMap(kh.currentMode)
	if binding, found := modeMap[keyStr]; found && kh.checkConditions(binding) {
		return binding, true
	}

	// Check global bindings (lowest priority)
	if binding, found := kh.globalBindings[keyStr]; found && kh.checkConditions(binding) {
		return binding, true
	}

	return KeyBinding{}, false
}

// hasPotentialMatch checks if the current key sequence could lead to a valid binding
func (kh *KeyHandler) hasPotentialMatch(sequence string) bool {
	// Check all binding maps for potential matches
	maps := []map[string]KeyBinding{
		kh.getBindingMap(kh.currentMode),
		kh.globalBindings,
	}

	if bindings, exists := kh.componentBindings[kh.focused]; exists {
		maps = append(maps, bindings)
	}

	for _, bindingMap := range maps {
		for key := range bindingMap {
			if strings.HasPrefix(key, sequence) && len(key) > len(sequence) {
				return true
			}
		}
	}

	return false
}

// executeBinding executes a key binding action
func (kh *KeyHandler) executeBinding(binding KeyBinding, context KeyContext) tea.Cmd {
	switch binding.Action.Type {
	case ActionCustom:
		if binding.Action.Handler != nil {
			return binding.Action.Handler(context)
		}
		return nil

	case ActionMessage:
		if binding.Action.Message != nil {
			return tea.Cmd(func() tea.Msg { return binding.Action.Message })
		}
		return nil

	default:
		return kh.executeDefaultAction(binding.Action, context)
	}
}

// executeDefaultAction executes built-in actions
func (kh *KeyHandler) executeDefaultAction(action KeyAction, context KeyContext) tea.Cmd {
	switch action.Type {
	case ActionEnterNormalMode:
		return tea.Cmd(func() tea.Msg {
			return NewModeTransitionMsg(ModeNormal, context.Mode, "keyboard")
		})

	case ActionEnterInsertMode:
		return tea.Cmd(func() tea.Msg {
			return NewModeTransitionMsg(ModeInsert, context.Mode, "keyboard")
		})

	case ActionEnterCommandMode:
		return tea.Cmd(func() tea.Msg {
			return NewModeTransitionMsg(ModeCommand, context.Mode, "keyboard")
		})

	case ActionFocusNext:
		return tea.Cmd(func() tea.Msg {
			return FocusMsg{
				Component: kh.getNextFocus(context.Component),
				PrevFocus: context.Component.String(),
				Reason:    "tab navigation",
			}
		})

	case ActionSwitchTab:
		tabNum := 1
		if action.Command != "" {
			if num, err := strconv.Atoi(action.Command); err == nil {
				tabNum = num
			}
		}
		return tea.Cmd(func() tea.Msg {
			return NewTabSwitchMsg("", "", tabNum-1, "keyboard_shortcut")
		})

	case ActionQuit:
		return tea.Quit

	case ActionShowShortcuts:
		return tea.Cmd(func() tea.Msg {
			return ShowHelpMsg{
				Mode:      context.Mode,
				Component: context.Component,
				Context:   "keyboard shortcuts",
			}
		})

	default:
		// Return a generic action message that components can handle
		return tea.Cmd(func() tea.Msg {
			return KeyActionMsg{
				Action:  action.Type,
				Context: context,
			}
		})
	}
}

// handleUnboundKey handles keys that don't have explicit bindings
func (kh *KeyHandler) handleUnboundKey(keyMsg tea.KeyMsg, mode Mode, component FocusedComponent) tea.Cmd {
	// In insert mode, most keys should be passed through for text input
	if mode == ModeInsert && keyMsg.Type == tea.KeyRunes {
		return tea.Cmd(func() tea.Msg {
			return TextInputMsg{
				Text:      string(keyMsg.Runes),
				Component: component,
			}
		})
	}

	return nil
}

// getBindingMap returns the appropriate binding map for a given mode
func (kh *KeyHandler) getBindingMap(mode Mode) map[string]KeyBinding {
	switch mode {
	case ModeNormal:
		return kh.normalBindings
	case ModeInsert:
		return kh.insertBindings
	case ModeCommand:
		return kh.commandBindings
	default:
		return kh.globalBindings
	}
}

// validateBinding validates a key binding for correctness
func (kh *KeyHandler) validateBinding(binding KeyBinding) error {
	if binding.Key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	if binding.Description == "" {
		return fmt.Errorf("description cannot be empty")
	}

	if binding.Action.Type == ActionCustom && binding.Action.Handler == nil {
		return fmt.Errorf("custom action must have a handler")
	}

	if binding.Action.Type == ActionMessage && binding.Action.Message == nil {
		return fmt.Errorf("message action must have a message")
	}

	return nil
}

// addBindingUnsafe adds a binding without locking (internal use)
func (kh *KeyHandler) addBindingUnsafe(binding KeyBinding) error {
	if err := kh.validateBinding(binding); err != nil {
		return err
	}

	targetMap := kh.getBindingMap(binding.Mode)
	if binding.Component != FocusGlobal {
		if kh.componentBindings[binding.Component] == nil {
			kh.componentBindings[binding.Component] = make(map[string]KeyBinding)
		}
		kh.componentBindings[binding.Component][binding.Key] = binding
	} else {
		targetMap[binding.Key] = binding
	}

	return nil
}

// checkConditions checks if all conditions for a binding are met
func (kh *KeyHandler) checkConditions(binding KeyBinding) bool {
	for _, condition := range binding.Conditions {
		if !kh.evaluateCondition(condition) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func (kh *KeyHandler) evaluateCondition(condition KeyCondition) bool {
	result := false

	switch condition.Type {
	case ConditionMode:
		if expectedMode, ok := condition.Value.(Mode); ok {
			result = kh.currentMode == expectedMode
		}

	case ConditionFocus:
		if expectedFocus, ok := condition.Value.(FocusedComponent); ok {
			result = kh.focused == expectedFocus
		}

	// Add more condition types as needed
	default:
		result = true
	}

	if condition.Negate {
		result = !result
	}

	return result
}

// updateHelpContext updates the help context when bindings change
func (kh *KeyHandler) updateHelpContext(binding KeyBinding) {
	helpEntry := KeyHelpEntry{
		Category:    kh.categorizeBinding(binding),
		Key:         binding.Key,
		Description: binding.Description,
		Context:     binding.Context,
	}

	if binding.Component != FocusGlobal {
		kh.contextHelp[binding.Component] = append(kh.contextHelp[binding.Component], helpEntry)
	} else {
		kh.helpContext[binding.Mode] = append(kh.helpContext[binding.Mode], helpEntry)
	}
}

// refreshHelpContext rebuilds the help context from current bindings
func (kh *KeyHandler) refreshHelpContext() {
	// Clear existing help
	kh.helpContext = make(map[Mode][]KeyHelpEntry)
	kh.contextHelp = make(map[FocusedComponent][]KeyHelpEntry)

	// Rebuild from all bindings
	allMaps := []map[string]KeyBinding{
		kh.normalBindings,
		kh.insertBindings,
		kh.commandBindings,
		kh.globalBindings,
	}

	for _, bindingMap := range allMaps {
		for _, binding := range bindingMap {
			kh.updateHelpContext(binding)
		}
	}

	for _, componentBindings := range kh.componentBindings {
		for _, binding := range componentBindings {
			kh.updateHelpContext(binding)
		}
	}
}

// categorizeBinding determines the help category for a binding
func (kh *KeyHandler) categorizeBinding(binding KeyBinding) string {
	switch binding.Action.Type {
	case ActionNavigateUp, ActionNavigateDown, ActionNavigateLeft, ActionNavigateRight,
		ActionPageUp, ActionPageDown, ActionGoToTop, ActionGoToBottom:
		return "Navigation"

	case ActionAddPattern, ActionDeletePattern, ActionCopyPattern, ActionPastePattern:
		return "Pattern Management"

	case ActionOpenFile, ActionCloseTab, ActionSwitchTab, ActionSave:
		return "File Operations"

	case ActionEnterInsertMode, ActionEnterNormalMode, ActionEnterCommandMode:
		return "Mode Control"

	case ActionSearch, ActionSearchNext, ActionSearchPrev:
		return "Search"

	case ActionQuit, ActionHelp, ActionShowShortcuts:
		return "Application"

	default:
		return "Other"
	}
}

// getNextFocus determines the next component to focus
func (kh *KeyHandler) getNextFocus(current FocusedComponent) string {
	switch current {
	case FocusViewer:
		return FocusIncludeFilter.String()
	case FocusIncludeFilter:
		return FocusExcludeFilter.String()
	case FocusExcludeFilter:
		return FocusTabs.String()
	case FocusTabs:
		return FocusStatusBar.String()
	case FocusStatusBar:
		return FocusViewer.String()
	default:
		return FocusViewer.String()
	}
}

// mapToSlice converts a binding map to a slice for serialization
func (kh *KeyHandler) mapToSlice(bindingMap map[string]KeyBinding) []KeyBinding {
	result := make([]KeyBinding, 0, len(bindingMap))
	for _, binding := range bindingMap {
		result = append(result, binding)
	}
	return result
}

// parseCustomBinding parses a custom binding from configuration
func (kh *KeyHandler) parseCustomBinding(key, action string) (KeyBinding, error) {
	// This would parse the action string and create appropriate binding
	// For now, return a basic custom binding
	return KeyBinding{
		Key:         key,
		Description: fmt.Sprintf("Custom: %s", action),
		Mode:        ModeNormal,
		Action:      KeyAction{Type: ActionCustom},
		Priority:    50, // Lower priority for custom bindings
	}, nil
}

// Additional message types for keyboard handling

// Note: TabSwitchMsg is defined in messages.go

// ShowHelpMsg requests showing help information
type ShowHelpMsg struct {
	Mode      Mode
	Component FocusedComponent
	Context   string
}

// KeyActionMsg represents a generic key action
type KeyActionMsg struct {
	Action  ActionType
	Context KeyContext
}

// TextInputMsg represents text input in insert mode
type TextInputMsg struct {
	Text      string
	Component FocusedComponent
}

// GetDefaultKeyBindings returns a map of default key bindings for the application
func GetDefaultKeyBindings() map[string]KeyBinding {
	// Create default keyboard configuration
	config := KeyboardConfig{
		SequenceTimeoutMs:   1000,
		RepeatDelayMs:       500,
		RepeatRateMs:        100,
		EnableVimBindings:   true,
		CaseSensitive:       false,
		AllowCustomBindings: true,
		ShowHelpInStatusBar: true,
		HelpOverlayStyle:    "default",
	}

	// Create key handler with default configuration
	keyHandler := NewKeyHandler(config)

	// Get all default bindings and flatten into a single map
	allBindings := keyHandler.GetAllBindings()
	result := make(map[string]KeyBinding)

	// Merge all binding categories into a single map
	for category, bindings := range allBindings {
		for _, binding := range bindings {
			// Use a composite key that includes the category for uniqueness
			key := fmt.Sprintf("%s:%s", category, binding.Key)
			result[key] = binding
		}
	}

	return result
}
