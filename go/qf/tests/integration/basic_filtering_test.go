// Package integration contains integration tests for the qf application.
//
// This test file focuses on end-to-end basic log filtering workflow testing,
// simulating user interactions with the terminal UI and verifying the complete
// filtering pipeline works as expected.
//
// INTEGRATION TEST REQUIREMENTS:
//
//  1. Application Lifecycle: Test complete application initialization, file loading,
//     UI component setup, and graceful shutdown
//
//  2. Modal Interface: Verify strict Normal/Insert mode transitions (Vim-style)
//     including proper key handling and state persistence across mode changes
//
//  3. Real-time Filtering: Test live preview updates during pattern entry,
//     immediate filter application on mode transitions, and performance
//
//  4. Component Integration: Verify message passing between UI components,
//     state synchronization, and proper component focus management
//
//  5. End-to-End Workflow: Simulate complete user scenarios from launch
//     to filter application, matching the quickstart documentation
//
// TEST DESIGN:
// This integration test MUST fail initially since no implementation exists.
// It defines the expected behavior for the complete qf application workflow
// and serves as a specification for the implementation.
//
// The test simulates the "Workflow 1: Basic Log Filtering" scenario:
// - Launch with test log file
// - Navigate to include pane and add ERROR pattern
// - Navigate to exclude pane and add "connection timeout" pattern
// - Verify real-time filtering and final results
package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestBasicFilteringWorkflow tests the complete basic log filtering workflow
// as described in the quickstart documentation. This test covers the full
// end-to-end user experience from application launch to filtered results.
//
// This test MUST fail initially since no implementation exists yet.
func TestBasicFilteringWorkflow(t *testing.T) {
	t.Run("Complete Basic Filtering Workflow", func(t *testing.T) {
		// This test will fail until the qf application is implemented
		testCompleteBasicFilteringWorkflow(t)
	})

	t.Run("Modal Interface Behavior", func(t *testing.T) {
		testModalInterfaceBehavior(t)
	})

	t.Run("Real-time Filtering", func(t *testing.T) {
		testRealTimeFiltering(t)
	})

	t.Run("Component Integration", func(t *testing.T) {
		testComponentIntegration(t)
	})

	t.Run("Error Handling and Recovery", func(t *testing.T) {
		testErrorHandlingAndRecovery(t)
	})
}

// testCompleteBasicFilteringWorkflow simulates the complete workflow described
// in the quickstart documentation: "Workflow 1: Basic Log Filtering"
func testCompleteBasicFilteringWorkflow(t *testing.T) {
	// Step 1: Create test log file with sample content
	testLogFile, err := createTestLogFile()
	if err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}
	defer os.Remove(testLogFile)

	// Step 2: Launch qf application with the test file
	// This will fail since no QfApplication exists yet
	app, err := NewQfApplication(QfConfig{
		FilePath: testLogFile,
		TestMode: true, // Enable test mode for programmatic control
	})
	if err != nil {
		t.Fatalf("QfApplication implementation not found - this test should fail until main application is implemented in internal/ui/app.go: %v", err)
	}
	defer app.Shutdown()

	// Step 3: Verify initial application state
	// - File loads successfully
	// - All lines are visible initially (no filters)
	// - Cursor starts in content viewer
	// - Status shows file info
	initialState := app.GetCurrentState()
	if initialState.LoadedFile != testLogFile {
		t.Errorf("Expected loaded file %s, got %s", testLogFile, initialState.LoadedFile)
	}
	if len(initialState.DisplayedLines) == 0 {
		t.Error("Expected initial content to be displayed")
	}
	if initialState.CurrentMode != ModeNormal {
		t.Errorf("Expected initial mode to be Normal, got %v", initialState.CurrentMode)
	}
	if initialState.FocusedComponent != "viewer" {
		t.Errorf("Expected initial focus on viewer, got %s", initialState.FocusedComponent)
	}

	// Step 4: Navigate to include pane (Tab key)
	// This simulates pressing Tab to focus the include filter pane
	err = app.SendKeyPress(tea.Key{Type: tea.KeyTab})
	if err != nil {
		t.Fatalf("Failed to send Tab key: %v", err)
	}

	// Verify focus moved to include pane
	state := app.GetCurrentState()
	if state.FocusedComponent != "include_pane" {
		t.Errorf("Expected focus on include_pane after Tab, got %s", state.FocusedComponent)
	}
	if state.CurrentMode != ModeNormal {
		t.Errorf("Expected mode to remain Normal after focus change, got %v", state.CurrentMode)
	}

	// Step 5: Enter Insert mode in include pane ('i' key)
	err = app.SendKeyPress(tea.Key{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if err != nil {
		t.Fatalf("Failed to send 'i' key: %v", err)
	}

	// Verify mode transition to Insert
	state = app.GetCurrentState()
	if state.CurrentMode != ModeInsert {
		t.Errorf("Expected mode transition to Insert after 'i', got %v", state.CurrentMode)
	}
	if state.FocusedComponent != "include_pane" {
		t.Errorf("Expected focus to remain on include_pane in Insert mode, got %s", state.FocusedComponent)
	}

	// Step 6: Type "ERROR" pattern with real-time validation
	// Test each character to verify real-time updates
	errorPattern := "ERROR"
	for i, char := range errorPattern {
		err = app.SendKeyPress(tea.Key{Type: tea.KeyRunes, Runes: []rune{char}})
		if err != nil {
			t.Fatalf("Failed to send character '%c': %v", char, err)
		}

		// Verify real-time pattern validation and preview
		state = app.GetCurrentState()
		expectedPartialPattern := errorPattern[:i+1]
		if len(state.IncludePatterns) == 0 {
			t.Error("Expected include pattern to be created during typing")
		} else {
			currentPattern := state.IncludePatterns[0].Expression
			if currentPattern != expectedPartialPattern {
				t.Errorf("Expected partial pattern '%s', got '%s'", expectedPartialPattern, currentPattern)
			}
			// Verify pattern is marked as valid (ERROR is a valid regex)
			if !state.IncludePatterns[0].IsValid {
				t.Errorf("Expected pattern '%s' to be valid", expectedPartialPattern)
			}
		}

		// Verify real-time filtering preview
		// Content should update to show only lines matching the partial pattern
		if i == len(errorPattern)-1 { // Complete pattern
			filteredLineCount := 0
			for _, line := range state.DisplayedLines {
				if strings.Contains(line, "ERROR") {
					filteredLineCount++
				}
			}
			if filteredLineCount != len(state.DisplayedLines) {
				t.Error("Expected all displayed lines to contain ERROR pattern")
			}
		}
	}

	// Step 7: Exit Insert mode (Escape key) and apply filter
	err = app.SendKeyPress(tea.Key{Type: tea.KeyEsc})
	if err != nil {
		t.Fatalf("Failed to send Escape key: %v", err)
	}

	// Verify mode transition back to Normal
	state = app.GetCurrentState()
	if state.CurrentMode != ModeNormal {
		t.Errorf("Expected mode transition to Normal after Escape, got %v", state.CurrentMode)
	}

	// Verify filter was applied immediately
	if len(state.IncludePatterns) != 1 {
		t.Errorf("Expected 1 include pattern after Escape, got %d", len(state.IncludePatterns))
	}
	if state.IncludePatterns[0].Expression != "ERROR" {
		t.Errorf("Expected include pattern 'ERROR', got '%s'", state.IncludePatterns[0].Expression)
	}

	// Verify content filtering - only ERROR lines should be displayed
	errorLineCount := 0
	for _, line := range state.DisplayedLines {
		if strings.Contains(line, "ERROR") {
			errorLineCount++
		} else {
			t.Errorf("Unexpected line in filtered results: %s", line)
		}
	}
	if errorLineCount == 0 {
		t.Error("Expected at least one ERROR line in filtered results")
	}

	// Step 8: Navigate to exclude pane (Tab twice)
	// First Tab should go to exclude pane (or intermediate component)
	err = app.SendKeyPress(tea.Key{Type: tea.KeyTab})
	if err != nil {
		t.Fatalf("Failed to send first Tab for exclude navigation: %v", err)
	}

	// Second Tab should reach exclude pane
	err = app.SendKeyPress(tea.Key{Type: tea.KeyTab})
	if err != nil {
		t.Fatalf("Failed to send second Tab for exclude navigation: %v", err)
	}

	// Verify focus is on exclude pane
	state = app.GetCurrentState()
	if state.FocusedComponent != "exclude_pane" {
		t.Errorf("Expected focus on exclude_pane after navigation, got %s", state.FocusedComponent)
	}

	// Step 9: Enter Insert mode in exclude pane ('i' key)
	err = app.SendKeyPress(tea.Key{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if err != nil {
		t.Fatalf("Failed to enter Insert mode in exclude pane: %v", err)
	}

	// Verify mode transition
	state = app.GetCurrentState()
	if state.CurrentMode != ModeInsert {
		t.Errorf("Expected Insert mode in exclude pane, got %v", state.CurrentMode)
	}

	// Step 10: Type "connection timeout" exclusion pattern
	excludePattern := "connection timeout"
	for _, char := range excludePattern {
		err = app.SendKeyPress(tea.Key{Type: tea.KeyRunes, Runes: []rune{char}})
		if err != nil {
			t.Fatalf("Failed to send character '%c' for exclude pattern: %v", char, err)
		}
	}

	// Verify exclude pattern was created
	state = app.GetCurrentState()
	if len(state.ExcludePatterns) != 1 {
		t.Errorf("Expected 1 exclude pattern, got %d", len(state.ExcludePatterns))
	}
	if state.ExcludePatterns[0].Expression != excludePattern {
		t.Errorf("Expected exclude pattern '%s', got '%s'", excludePattern, state.ExcludePatterns[0].Expression)
	}
	if !state.ExcludePatterns[0].IsValid {
		t.Errorf("Expected exclude pattern '%s' to be valid", excludePattern)
	}

	// Step 11: Exit Insert mode and apply both filters
	err = app.SendKeyPress(tea.Key{Type: tea.KeyEsc})
	if err != nil {
		t.Fatalf("Failed to exit Insert mode in exclude pane: %v", err)
	}

	// Verify final filtered state
	state = app.GetCurrentState()
	if state.CurrentMode != ModeNormal {
		t.Errorf("Expected Normal mode after final Escape, got %v", state.CurrentMode)
	}

	// Verify both filters are active
	if len(state.IncludePatterns) != 1 || len(state.ExcludePatterns) != 1 {
		t.Errorf("Expected 1 include and 1 exclude pattern, got %d include, %d exclude",
			len(state.IncludePatterns), len(state.ExcludePatterns))
	}

	// Verify final filtering logic:
	// - Lines must contain "ERROR" (include filter)
	// - Lines must NOT contain "connection timeout" (exclude filter)
	finalResults := state.DisplayedLines
	if len(finalResults) == 0 {
		t.Error("Expected some lines to remain after filtering")
	}

	for _, line := range finalResults {
		if !strings.Contains(line, "ERROR") {
			t.Errorf("Final result missing ERROR pattern: %s", line)
		}
		if strings.Contains(line, "connection timeout") {
			t.Errorf("Final result should not contain excluded pattern: %s", line)
		}
	}

	// Step 12: Verify status updates reflect current filter state
	statusInfo := state.StatusInfo
	if statusInfo.FilterCount != 2 {
		t.Errorf("Expected status to show 2 active filters, got %d", statusInfo.FilterCount)
	}
	if statusInfo.MatchedLines != len(finalResults) {
		t.Errorf("Expected status to show %d matched lines, got %d", len(finalResults), statusInfo.MatchedLines)
	}

	// Step 13: Verify performance requirements
	// Filter application should be responsive (<150ms as per contract)
	if state.LastFilterDuration > 150*time.Millisecond {
		t.Errorf("Filter application too slow: %v (expected <150ms)", state.LastFilterDuration)
	}

	t.Logf("Basic filtering workflow completed successfully")
	t.Logf("Final results: %d lines (from %d total) with filters: include='%s', exclude='%s'",
		len(finalResults), state.TotalLines,
		state.IncludePatterns[0].Expression,
		state.ExcludePatterns[0].Expression)
}

// testModalInterfaceBehavior tests the Vim-style modal interface requirements
func testModalInterfaceBehavior(t *testing.T) {
	// This test will fail until modal interface is implemented
	t.Error("Modal interface implementation does not exist yet - needed in internal/ui/ components")

	// Test Normal mode behavior
	// - Navigation keys (Tab, Arrow keys, etc.)
	// - Mode transition keys ('i' for Insert)
	// - Application commands ('q' for quit)

	// Test Insert mode behavior
	// - Text input and editing
	// - Real-time validation
	// - Escape to exit to Normal mode
	// - No navigation in Insert mode

	// Test mode persistence across component focus changes
	// Test invalid mode transitions (should be rejected)
	// Test mode display in status bar

	t.Fatal("Modal interface system needs implementation - this test defines the expected behavior")
}

// testRealTimeFiltering tests real-time filter updates during pattern entry
func testRealTimeFiltering(t *testing.T) {
	// This test will fail until real-time filtering is implemented
	t.Error("Real-time filtering implementation does not exist yet - needed in internal/core/filter.go")

	// Test live preview during pattern typing
	// - Each keystroke updates filter preview
	// - Invalid patterns show error state but don't crash
	// - Partial matches update in real-time
	// - Performance remains responsive during typing

	// Test debouncing for performance
	// - Rapid keystrokes don't cause excessive filtering
	// - Configurable debounce delay (default 150ms)
	// - Final filter application on mode exit

	t.Fatal("Real-time filtering system needs implementation - this test defines the expected behavior")
}

// testComponentIntegration tests message passing and state sync between UI components
func testComponentIntegration(t *testing.T) {
	// This test will fail until component integration is implemented
	t.Error("Component integration system does not exist yet - needed in internal/ui/app.go")

	// Test FilterUpdateMsg propagation
	// - Filter changes in panes update viewer immediately
	// - Status bar reflects current filter state
	// - Tab updates show correct match counts

	// Test ModeTransitionMsg handling
	// - All components update their display for current mode
	// - Component-specific mode behaviors activate
	// - Proper focus management across mode changes

	// Test ErrorMsg display
	// - Validation errors show in status bar
	// - File loading errors display appropriately
	// - Recoverable vs non-recoverable error handling

	// Test component registration and lifecycle
	// - Components can be added/removed dynamically
	// - Message routing works correctly
	// - Component cleanup on shutdown

	t.Fatal("Component integration system needs implementation - this test defines the expected behavior")
}

// testErrorHandlingAndRecovery tests error conditions and recovery mechanisms
func testErrorHandlingAndRecovery(t *testing.T) {
	// This test will fail until error handling is implemented
	t.Error("Error handling and recovery system does not exist yet")

	// Test invalid regex patterns
	// - Show validation error in real-time
	// - Allow correction without losing other filters
	// - Don't crash on invalid patterns

	// Test file operations errors
	// - Handle missing files gracefully
	// - Show appropriate error messages
	// - Allow retry mechanisms

	// Test memory/performance errors
	// - Handle large files appropriately
	// - Graceful degradation on memory pressure
	// - Cancel long-running operations

	// Test recovery scenarios
	// - Restore previous state on errors
	// - Clear invalid patterns automatically
	// - Maintain application stability

	t.Fatal("Error handling system needs implementation - this test defines the expected behavior")
}

// Helper function to create a test log file with sample content
func createTestLogFile() (string, error) {
	tempDir, err := ioutil.TempDir("", "qf-test")
	if err != nil {
		return "", err
	}

	testLogPath := filepath.Join(tempDir, "application.log")

	// Sample log content that matches the quickstart scenario
	logContent := []string{
		"2025-09-14 10:00:01 INFO Application started successfully",
		"2025-09-14 10:00:02 INFO Loading configuration from config.yml",
		"2025-09-14 10:00:03 DEBUG Database connection pool initialized",
		"2025-09-14 10:00:04 INFO Server listening on port 8080",
		"2025-09-14 10:00:05 ERROR Failed to connect to database: connection timeout",
		"2025-09-14 10:00:06 ERROR Authentication failed for user 'admin'",
		"2025-09-14 10:00:07 WARN Session expired for user 'johndoe'",
		"2025-09-14 10:00:08 INFO Processing user request: GET /api/users",
		"2025-09-14 10:00:09 ERROR Invalid request format in POST /api/data",
		"2025-09-14 10:00:10 DEBUG Request processing completed in 45ms",
		"2025-09-14 10:00:11 ERROR Network connection timeout during sync",
		"2025-09-14 10:00:12 INFO User 'alice' logged in successfully",
		"2025-09-14 10:00:13 ERROR Failed to save user data: disk full",
		"2025-09-14 10:00:14 INFO Background cleanup job started",
		"2025-09-14 10:00:15 DEBUG Memory usage: 125MB / 512MB",
	}

	content := strings.Join(logContent, "\n")
	err = ioutil.WriteFile(testLogPath, []byte(content), 0644)
	if err != nil {
		return "", err
	}

	return testLogPath, nil
}

// All types are now defined in shared_types.go to avoid duplication

// QfConfig defines configuration options for the qf application
type QfConfig struct {
	FilePath string // Path to log file to open
	TestMode bool   // Enable test mode for programmatic control
}

// ApplicationState represents the current state of the qf application
type ApplicationState struct {
	// File state
	LoadedFile     string
	TotalLines     int
	DisplayedLines []string

	// Filter state
	IncludePatterns []FilterPattern
	ExcludePatterns []FilterPattern

	// UI state
	CurrentMode        Mode
	FocusedComponent   string
	LastFilterDuration time.Duration

	// Status information
	StatusInfo StatusInfo
}

// StatusInfo represents information displayed in the status bar
type StatusInfo struct {
	FilterCount  int
	MatchedLines int
	CurrentFile  string
	CurrentMode  string
}

// QfApplication interface defines the expected application interface for basic filtering testing
// This interface MUST be implemented for the integration tests to pass
type QfApplication interface {
	// GetCurrentState returns the current application state for verification
	GetCurrentState() ApplicationState

	// SendKeyPress simulates a key press for testing
	SendKeyPress(key tea.Key) error

	// Shutdown gracefully shuts down the application
	Shutdown() error
}

// NewQfApplication creates a new qf application instance
// This function MUST be implemented for the integration tests to pass
func NewQfApplication(config QfConfig) (QfApplication, error) {
	// This will fail until implementation exists
	return nil, fmt.Errorf("QfApplication implementation not found - needed in cmd/qf/main.go and internal/ui/app.go")
}

// TestDocumentationExistence verifies that integration test documentation exists
func TestDocumentationExistence(t *testing.T) {
	t.Log("Integration test requirements:")
	t.Log("1. Complete workflow simulation from launch to filtered results")
	t.Log("2. Vim-style modal interface with strict Normal/Insert mode discipline")
	t.Log("3. Real-time filtering with live preview during pattern entry")
	t.Log("4. Component integration via message passing (FilterUpdateMsg, ModeTransitionMsg, etc.)")
	t.Log("5. Error handling and recovery for invalid patterns and file operations")
	t.Log("6. Performance requirements: <150ms filter application, responsive UI")
	t.Log("7. State verification at each step of the user workflow")

	// This test always passes - it's just for documentation
	t.Log("Integration test framework is ready - implementation needed to make tests pass")
}
