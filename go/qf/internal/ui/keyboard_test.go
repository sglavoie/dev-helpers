package ui

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewKeyHandler(t *testing.T) {
	config := KeyboardConfig{
		SequenceTimeoutMs:   500,
		RepeatDelayMs:       250,
		RepeatRateMs:        50,
		EnableVimBindings:   true,
		CaseSensitive:       false,
		AllowCustomBindings: true,
		ShowHelpInStatusBar: true,
		HelpOverlayStyle:    "default",
	}

	kh := NewKeyHandler(config)

	if kh == nil {
		t.Fatal("KeyHandler should not be nil")
	}
	if kh.currentMode != ModeNormal {
		t.Errorf("Expected currentMode to be ModeNormal, got %v", kh.currentMode)
	}
	if kh.focused != FocusViewer {
		t.Errorf("Expected focused to be FocusViewer, got %v", kh.focused)
	}

	// Verify all binding maps are initialized
	if kh.normalBindings == nil {
		t.Error("normalBindings should not be nil")
	}
	if kh.insertBindings == nil {
		t.Error("insertBindings should not be nil")
	}
	if kh.commandBindings == nil {
		t.Error("commandBindings should not be nil")
	}
	if kh.globalBindings == nil {
		t.Error("globalBindings should not be nil")
	}
	if kh.componentBindings == nil {
		t.Error("componentBindings should not be nil")
	}

	// Verify component binding maps are initialized
	for comp := FocusViewer; comp <= FocusOverlay; comp++ {
		if kh.componentBindings[comp] == nil {
			t.Errorf("componentBindings[%v] should not be nil", comp)
		}
	}

	// Verify default bindings are loaded
	if len(kh.normalBindings) == 0 {
		t.Error("Normal mode bindings should be loaded")
	}
	if len(kh.insertBindings) == 0 {
		t.Error("Insert mode bindings should be loaded")
	}
}

func TestKeyHandler_HandleKey_NormalMode(t *testing.T) {
	kh := NewKeyHandler(KeyboardConfig{
		SequenceTimeoutMs: 500,
		EnableVimBindings: true,
	})

	tests := []struct {
		name        string
		key         tea.KeyMsg
		mode        Mode
		focused     FocusedComponent
		expectCmd   bool
		description string
	}{
		{
			name:        "j navigation down",
			key:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
			mode:        ModeNormal,
			focused:     FocusViewer,
			expectCmd:   true,
			description: "j should trigger navigate down action",
		},
		{
			name:        "k navigation up",
			key:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}},
			mode:        ModeNormal,
			focused:     FocusViewer,
			expectCmd:   true,
			description: "k should trigger navigate up action",
		},
		{
			name:        "i insert mode",
			key:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}},
			mode:        ModeNormal,
			focused:     FocusViewer,
			expectCmd:   true,
			description: "i should trigger insert mode transition",
		},
		{
			name:        "tab focus next",
			key:         tea.KeyMsg{Type: tea.KeyTab},
			mode:        ModeNormal,
			focused:     FocusViewer,
			expectCmd:   true,
			description: "tab should focus next component",
		},
		{
			name:        "q quit",
			key:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
			mode:        ModeNormal,
			focused:     FocusViewer,
			expectCmd:   true,
			description: "q should trigger quit action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := kh.HandleKey(tt.key, tt.mode, tt.focused)
			if tt.expectCmd && cmd == nil {
				t.Errorf("%s: expected command but got nil", tt.description)
			}
			if !tt.expectCmd && cmd != nil {
				t.Errorf("%s: expected nil command but got %v", tt.description, cmd)
			}
		})
	}
}

func TestKeyHandler_HandleKey_InsertMode(t *testing.T) {
	kh := NewKeyHandler(KeyboardConfig{
		SequenceTimeoutMs: 500,
		EnableVimBindings: true,
	})

	tests := []struct {
		name        string
		key         tea.KeyMsg
		mode        Mode
		focused     FocusedComponent
		expectCmd   bool
		description string
	}{
		{
			name:        "escape to normal",
			key:         tea.KeyMsg{Type: tea.KeyEsc},
			mode:        ModeInsert,
			focused:     FocusIncludeFilter,
			expectCmd:   true,
			description: "escape should transition to normal mode",
		},
		{
			name:        "text input",
			key:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
			mode:        ModeInsert,
			focused:     FocusIncludeFilter,
			expectCmd:   true,
			description: "regular text should generate text input message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := kh.HandleKey(tt.key, tt.mode, tt.focused)
			if tt.expectCmd && cmd == nil {
				t.Errorf("%s: expected command but got nil", tt.description)
			}
			if !tt.expectCmd && cmd != nil {
				t.Errorf("%s: expected nil command but got %v", tt.description, cmd)
			}
		})
	}
}

func TestKeyHandler_MultiKeySequences(t *testing.T) {
	kh := NewKeyHandler(KeyboardConfig{
		SequenceTimeoutMs: 500,
		EnableVimBindings: true,
	})

	// Test "gg" sequence (go to top)
	t.Run("gg sequence", func(t *testing.T) {
		// First 'g' - should not trigger action yet
		cmd1 := kh.HandleKey(
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}},
			ModeNormal,
			FocusViewer,
		)
		if cmd1 != nil {
			t.Error("First 'g' should not trigger action")
		}

		// Second 'g' - should trigger go to top
		cmd2 := kh.HandleKey(
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}},
			ModeNormal,
			FocusViewer,
		)
		if cmd2 == nil {
			t.Error("Second 'g' should trigger go to top action")
		}
	})

	// Test sequence timeout
	t.Run("sequence timeout", func(t *testing.T) {
		// Start sequence
		cmd1 := kh.HandleKey(
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}},
			ModeNormal,
			FocusViewer,
		)
		if cmd1 != nil {
			t.Error("First 'g' should not trigger action")
		}

		// Simulate timeout by manipulating last key time
		kh.mutex.Lock()
		kh.lastKeyTime = time.Now().Add(-time.Hour)
		kh.mutex.Unlock()

		// Next key should reset sequence
		cmd2 := kh.HandleKey(
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
			ModeNormal,
			FocusViewer,
		)
		if cmd2 == nil {
			t.Error("After timeout, single key should work normally")
		}
	})
}

func TestKeyHandler_TabSwitching(t *testing.T) {
	kh := NewKeyHandler(KeyboardConfig{
		SequenceTimeoutMs: 500,
		EnableVimBindings: true,
	})

	// Test number key tab switching
	for i := 1; i <= 3; i++ { // Test fewer to keep test simpler
		t.Run(fmt.Sprintf("tab %d", i), func(t *testing.T) {
			key := tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{rune('0' + i)},
			}

			cmd := kh.HandleKey(key, ModeNormal, FocusViewer)
			if cmd == nil {
				t.Error("Number key should trigger tab switch")
				return
			}

			// Execute command to get message
			msg := cmd()
			tabMsg, ok := msg.(TabSwitchMsg)
			if !ok {
				t.Error("Should generate TabSwitchMsg")
				return
			}
			if tabMsg.TabNumber != i {
				t.Errorf("Tab number should be %d, got %d", i, tabMsg.TabNumber)
			}
		})
	}
}

func TestKeyHandler_AddBinding(t *testing.T) {
	kh := NewKeyHandler(KeyboardConfig{
		SequenceTimeoutMs: 500,
		EnableVimBindings: true,
	})

	// Test adding valid binding
	t.Run("valid binding", func(t *testing.T) {
		binding := KeyBinding{
			Key:         "ctrl+x",
			Description: "Custom action",
			Mode:        ModeNormal,
			Component:   FocusViewer,
			Action:      KeyAction{Type: ActionCustom, Handler: func(ctx KeyContext) tea.Cmd { return nil }},
			Priority:    100,
		}

		err := kh.AddBinding(binding)
		if err != nil {
			t.Errorf("AddBinding should not return error: %v", err)
		}

		// Verify binding was added
		bindings := kh.componentBindings[FocusViewer]
		if _, exists := bindings["ctrl+x"]; !exists {
			t.Error("Binding should be added to component bindings")
		}
	})

	// Test adding invalid binding
	t.Run("invalid binding - empty key", func(t *testing.T) {
		binding := KeyBinding{
			Key:         "",
			Description: "Invalid binding",
			Mode:        ModeNormal,
			Action:      KeyAction{Type: ActionNavigateUp},
		}

		err := kh.AddBinding(binding)
		if err == nil {
			t.Error("AddBinding should return error for empty key")
		}
	})
}

func TestKeyHandler_GetHelp(t *testing.T) {
	kh := NewKeyHandler(KeyboardConfig{
		SequenceTimeoutMs: 500,
		EnableVimBindings: true,
	})

	// Get help for normal mode and viewer
	help := kh.GetHelp(ModeNormal, FocusViewer)
	if len(help) == 0 {
		t.Error("Should return help entries")
	}

	// Verify help entries have required fields
	for _, entry := range help {
		if entry.Key == "" {
			t.Error("Help entry should have key")
		}
		if entry.Description == "" {
			t.Error("Help entry should have description")
		}
		if entry.Category == "" {
			t.Error("Help entry should have category")
		}
	}
}

func TestKeyHandler_KeyMsgToString(t *testing.T) {
	kh := NewKeyHandler(KeyboardConfig{})

	tests := []struct {
		input    tea.KeyMsg
		expected string
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, "j"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")}, "hello"},
		{tea.KeyMsg{Type: tea.KeySpace}, "space"},
		{tea.KeyMsg{Type: tea.KeyEnter}, "enter"},
		{tea.KeyMsg{Type: tea.KeyTab}, "tab"},
		{tea.KeyMsg{Type: tea.KeyEsc}, "esc"},
		{tea.KeyMsg{Type: tea.KeyUp}, "up"},
		{tea.KeyMsg{Type: tea.KeyDown}, "down"},
		{tea.KeyMsg{Type: tea.KeyCtrlA}, "ctrl+a"},
		{tea.KeyMsg{Type: tea.KeyCtrlC}, "ctrl+c"},
		{tea.KeyMsg{Type: tea.KeyCtrlS}, "ctrl+s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := kh.keyMsgToString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestKeyHandler_FocusedComponent_String(t *testing.T) {
	tests := []struct {
		component FocusedComponent
		expected  string
	}{
		{FocusIncludeFilter, "include_pane"},
		{FocusViewer, "viewer"},
		{FocusTabs, "tabs"},
		{FocusStatusBar, "status_bar"},
		{FocusOverlay, "overlay"},
		{FocusGlobal, "global"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.component.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestKeyHandler_SetModeAndFocus(t *testing.T) {
	kh := NewKeyHandler(KeyboardConfig{})

	// Test SetMode
	if kh.currentMode != ModeNormal {
		t.Errorf("Expected initial mode to be ModeNormal, got %v", kh.currentMode)
	}

	kh.SetMode(ModeInsert)
	if kh.currentMode != ModeInsert {
		t.Errorf("Expected mode to be ModeInsert after SetMode, got %v", kh.currentMode)
	}

	// Test SetFocus
	if kh.focused != FocusViewer {
		t.Errorf("Expected initial focus to be FocusViewer, got %v", kh.focused)
	}

	kh.SetFocus(FocusIncludeFilter)
	if kh.focused != FocusIncludeFilter {
		t.Errorf("Expected focus to be FocusIncludeFilter after SetFocus, got %v", kh.focused)
	}
}

func TestKeyHandler_GetAllBindings(t *testing.T) {
	kh := NewKeyHandler(KeyboardConfig{
		SequenceTimeoutMs: 500,
		EnableVimBindings: true,
	})

	bindings := kh.GetAllBindings()

	// Verify structure
	if _, exists := bindings["normal"]; !exists {
		t.Error("Should contain normal bindings")
	}
	if _, exists := bindings["insert"]; !exists {
		t.Error("Should contain insert bindings")
	}
	if _, exists := bindings["command"]; !exists {
		t.Error("Should contain command bindings")
	}
	if _, exists := bindings["global"]; !exists {
		t.Error("Should contain global bindings")
	}

	// Verify component bindings
	if _, exists := bindings[FocusIncludeFilter.String()]; !exists {
		t.Error("Should contain include filter bindings")
	}
	if _, exists := bindings[FocusViewer.String()]; !exists {
		t.Error("Should contain viewer bindings")
	}

	// Verify we have some bindings loaded
	if len(bindings["normal"]) == 0 {
		t.Error("Normal mode should have bindings")
	}
	if len(bindings["insert"]) == 0 {
		t.Error("Insert mode should have bindings")
	}
}

// Benchmark tests
func BenchmarkKeyHandler_HandleKey(b *testing.B) {
	kh := NewKeyHandler(KeyboardConfig{
		SequenceTimeoutMs: 500,
		EnableVimBindings: true,
	})

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kh.HandleKey(key, ModeNormal, FocusViewer)
	}
}

func BenchmarkKeyHandler_FindBinding(b *testing.B) {
	kh := NewKeyHandler(KeyboardConfig{
		SequenceTimeoutMs: 500,
		EnableVimBindings: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kh.findBinding("j")
	}
}

// Example test showing integration with Bubble Tea
func ExampleKeyHandler() {
	config := KeyboardConfig{
		SequenceTimeoutMs:   500,
		RepeatDelayMs:       250,
		RepeatRateMs:        50,
		EnableVimBindings:   true,
		CaseSensitive:       false,
		AllowCustomBindings: true,
		ShowHelpInStatusBar: true,
		HelpOverlayStyle:    "default",
	}

	kh := NewKeyHandler(config)

	// Handle a key press
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	cmd := kh.HandleKey(keyMsg, ModeNormal, FocusViewer)

	if cmd != nil {
		// Execute command to get the resulting message
		msg := cmd()
		switch m := msg.(type) {
		case KeyActionMsg:
			fmt.Printf("Key action: %v", m.Action)
		case ModeTransitionMsg:
			fmt.Printf("Mode transition to: %v", m.NewMode)
		}
	}
}
