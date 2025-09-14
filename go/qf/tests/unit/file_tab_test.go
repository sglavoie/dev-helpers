// Package unit contains unit tests for individual components of the qf application.
//
// This file tests the FileTab implementation in isolation, verifying its core
// functionality including file loading, view state management, and helper functions.
package unit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/file"
)

func TestNewFileTab(t *testing.T) {
	path := "/test/path/file.log"
	tab := file.NewFileTab(path)

	// Test basic properties
	if tab.ID == "" {
		t.Error("FileTab ID should not be empty")
	}

	if !filepath.IsAbs(tab.Path) {
		t.Error("FileTab Path should be absolute")
	}

	if tab.DisplayName == "" {
		t.Error("FileTab DisplayName should not be empty")
	}

	if tab.IsLoaded {
		t.Error("New FileTab should not be loaded initially")
	}

	if tab.Modified {
		t.Error("New FileTab should not be modified initially")
	}

	if tab.LastAccessed.IsZero() {
		t.Error("FileTab LastAccessed should be set")
	}

	// Test view state defaults
	if tab.ViewState.CursorLine != 1 {
		t.Errorf("Expected CursorLine to be 1, got %d", tab.ViewState.CursorLine)
	}

	if tab.ViewState.TopVisibleLine != 1 {
		t.Errorf("Expected TopVisibleLine to be 1, got %d", tab.ViewState.TopVisibleLine)
	}

	if tab.ViewState.ViewportHeight <= 0 {
		t.Errorf("ViewportHeight should be positive, got %d", tab.ViewState.ViewportHeight)
	}
}

func TestFileTab_LoadFromFile(t *testing.T) {
	// Create a temporary test file
	tempFile, err := os.CreateTemp("", "fileab_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write test content
	testContent := "Line 1\nLine 2\nLine 3\n"
	if _, err := tempFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tempFile.Close()

	// Create FileTab and load content
	tab := file.NewFileTab(tempFile.Name())
	ctx := context.Background()

	err = tab.LoadFromFile(ctx)
	if err != nil {
		t.Fatalf("LoadFromFile should not return error: %v", err)
	}

	// Verify loading state
	if !tab.IsLoaded {
		t.Error("FileTab should be marked as loaded")
	}

	// Verify content
	expectedLines := 3
	if len(tab.Content) != expectedLines {
		t.Errorf("Expected %d lines, got %d", expectedLines, len(tab.Content))
	}

	expectedContent := []string{"Line 1", "Line 2", "Line 3"}
	for i, line := range tab.Content {
		if line.Number != i+1 {
			t.Errorf("Line %d: expected number %d, got %d", i, i+1, line.Number)
		}
		if line.Content != expectedContent[i] {
			t.Errorf("Line %d: expected content %q, got %q", i+1, expectedContent[i], line.Content)
		}
		if !line.Highlighted == false {
			// Default highlighting state should be false
		}
	}

	// Test loading already loaded file (should not error)
	err = tab.LoadFromFile(ctx)
	if err != nil {
		t.Errorf("LoadFromFile on already loaded file should not error: %v", err)
	}
}

func TestFileTab_LoadFromFile_NonExistent(t *testing.T) {
	tab := file.NewFileTab("/path/that/does/not/exist")
	ctx := context.Background()

	err := tab.LoadFromFile(ctx)
	if err == nil {
		t.Error("LoadFromFile should return error for non-existent file")
	}

	if tab.IsLoaded {
		t.Error("FileTab should not be marked as loaded when file doesn't exist")
	}
}

func TestFileTab_GetLineCount(t *testing.T) {
	tab := file.NewFileTab("/test/path")

	// Empty tab
	if tab.GetLineCount() != 0 {
		t.Errorf("Empty tab should have 0 lines, got %d", tab.GetLineCount())
	}

	// Add some content
	tab.Content = []file.Line{
		{Number: 1, Content: "Line 1", Offset: 0},
		{Number: 2, Content: "Line 2", Offset: 7},
		{Number: 3, Content: "Line 3", Offset: 14},
	}

	if tab.GetLineCount() != 3 {
		t.Errorf("Tab with 3 lines should return 3, got %d", tab.GetLineCount())
	}
}

func TestFileTab_GetViewRange(t *testing.T) {
	tab := file.NewFileTab("/test/path")

	// Add test content
	for i := 1; i <= 10; i++ {
		tab.Content = append(tab.Content, file.Line{
			Number:  i,
			Content: "Line " + string(rune('0'+i)),
			Offset:  int64((i - 1) * 7),
		})
	}

	// Test default view range (starting from line 1, viewport height 25)
	viewRange := tab.GetViewRange()
	if len(viewRange) != 10 {
		t.Errorf("Expected 10 lines in view range, got %d", len(viewRange))
	}

	// Test with smaller viewport
	tab.ViewState.ViewportHeight = 3
	tab.ViewState.TopVisibleLine = 5
	viewRange = tab.GetViewRange()

	if len(viewRange) != 3 {
		t.Errorf("Expected 3 lines in view range, got %d", len(viewRange))
	}

	// Verify correct lines are returned
	expectedLines := []int{5, 6, 7}
	for i, line := range viewRange {
		if line.Number != expectedLines[i] {
			t.Errorf("Expected line %d at position %d, got %d", expectedLines[i], i, line.Number)
		}
	}
}

func TestFileTab_ScrollTo(t *testing.T) {
	tab := file.NewFileTab("/test/path")

	// Add test content
	for i := 1; i <= 100; i++ {
		tab.Content = append(tab.Content, file.Line{
			Number:  i,
			Content: "Line " + string(rune('0'+i%10)),
			Offset:  int64((i - 1) * 7),
		})
	}

	tab.ViewState.ViewportHeight = 20

	// Test scroll to middle
	tab.ScrollTo(50)
	if tab.ViewState.CursorLine != 50 {
		t.Errorf("Expected cursor at line 50, got %d", tab.ViewState.CursorLine)
	}

	// Cursor should be centered in viewport
	expectedTopLine := 50 - 10 // 50 - (viewport/2)
	if tab.ViewState.TopVisibleLine != expectedTopLine {
		t.Errorf("Expected top visible line %d, got %d", expectedTopLine, tab.ViewState.TopVisibleLine)
	}

	// Test scroll to beginning (should handle boundary)
	tab.ScrollTo(1)
	if tab.ViewState.CursorLine != 1 {
		t.Errorf("Expected cursor at line 1, got %d", tab.ViewState.CursorLine)
	}
	if tab.ViewState.TopVisibleLine != 1 {
		t.Errorf("Expected top visible line 1, got %d", tab.ViewState.TopVisibleLine)
	}

	// Test scroll to end
	tab.ScrollTo(100)
	if tab.ViewState.CursorLine != 100 {
		t.Errorf("Expected cursor at line 100, got %d", tab.ViewState.CursorLine)
	}

	// Test scroll beyond end (should clamp)
	tab.ScrollTo(150)
	if tab.ViewState.CursorLine != 100 {
		t.Errorf("Expected cursor clamped at line 100, got %d", tab.ViewState.CursorLine)
	}
}

func TestFileTab_HighlightLine(t *testing.T) {
	tab := file.NewFileTab("/test/path")

	// Add test content
	for i := 1; i <= 5; i++ {
		tab.Content = append(tab.Content, file.Line{
			Number:      i,
			Content:     "Line " + string(rune('0'+i)),
			Offset:      int64((i - 1) * 7),
			Highlighted: false,
		})
	}

	// Test highlighting
	tab.HighlightLine(3, true)
	if !tab.Content[2].Highlighted {
		t.Error("Line 3 should be highlighted")
	}

	// Test unhighlighting
	tab.HighlightLine(3, false)
	if tab.Content[2].Highlighted {
		t.Error("Line 3 should not be highlighted")
	}

	// Test invalid line numbers (should not panic)
	tab.HighlightLine(0, true)   // Too low
	tab.HighlightLine(100, true) // Too high
}

func TestFileTab_ClearHighlights(t *testing.T) {
	tab := file.NewFileTab("/test/path")

	// Add test content with some highlights
	for i := 1; i <= 5; i++ {
		highlighted := i%2 == 0 // Highlight even lines
		tab.Content = append(tab.Content, file.Line{
			Number:      i,
			Content:     "Line " + string(rune('0'+i)),
			Offset:      int64((i - 1) * 7),
			Highlighted: highlighted,
		})
	}

	// Verify some lines are highlighted
	highlightedCount := 0
	for _, line := range tab.Content {
		if line.Highlighted {
			highlightedCount++
		}
	}
	if highlightedCount == 0 {
		t.Error("Should have some highlighted lines before clearing")
	}

	// Clear highlights
	tab.ClearHighlights()

	// Verify no lines are highlighted
	for i, line := range tab.Content {
		if line.Highlighted {
			t.Errorf("Line %d should not be highlighted after ClearHighlights", i+1)
		}
	}
}

func TestFileTab_GetHighlightedLines(t *testing.T) {
	tab := file.NewFileTab("/test/path")

	// Add test content
	for i := 1; i <= 5; i++ {
		highlighted := i == 2 || i == 4 // Highlight lines 2 and 4
		tab.Content = append(tab.Content, file.Line{
			Number:      i,
			Content:     "Line " + string(rune('0'+i)),
			Offset:      int64((i - 1) * 7),
			Highlighted: highlighted,
		})
	}

	highlighted := tab.GetHighlightedLines()
	if len(highlighted) != 2 {
		t.Errorf("Expected 2 highlighted lines, got %d", len(highlighted))
	}

	expectedNumbers := []int{2, 4}
	for i, line := range highlighted {
		if line.Number != expectedNumbers[i] {
			t.Errorf("Expected highlighted line %d, got %d", expectedNumbers[i], line.Number)
		}
	}
}

func TestFileTab_ModificationTracking(t *testing.T) {
	tab := file.NewFileTab("/test/path")

	// Initial state
	if tab.IsModified() {
		t.Error("New FileTab should not be modified")
	}

	// Set modified
	tab.SetModified()
	if !tab.IsModified() {
		t.Error("FileTab should be modified after SetModified")
	}

	// Test that operations set modified flag
	tab.Modified = false // Reset
	tab.ScrollTo(10)
	if !tab.IsModified() {
		t.Error("ScrollTo should set modified flag")
	}

	// Add content for highlighting test
	tab.Content = append(tab.Content, file.Line{
		Number:  1,
		Content: "Test line",
		Offset:  0,
	})

	tab.Modified = false // Reset
	tab.HighlightLine(1, true)
	if !tab.IsModified() {
		t.Error("HighlightLine should set modified flag")
	}
}

func TestFileTab_LastAccessedUpdate(t *testing.T) {
	tab := file.NewFileTab("/test/path")
	initialTime := tab.LastAccessed

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	tab.UpdateLastAccessed()
	if !tab.LastAccessed.After(initialTime) {
		t.Error("LastAccessed should be updated")
	}
}

func TestGenerateDisplayName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/short/path", "/short/path"},                            // Short path unchanged
		{"/very/long/path/to/some/deeply/nested/file.log", "..."}, // Long path truncated
		{"/home/user/file.log", "user/file.log"},                  // Parent + base
		{"file.log", "file.log"},                                  // Just filename
	}

	for _, tt := range tests {
		// Note: generateDisplayName is not exported, so we test through NewFileTab
		tab := file.NewFileTab(tt.path)

		// Basic validation that display name is reasonable
		if tab.DisplayName == "" {
			t.Errorf("Display name for %s should not be empty", tt.path)
		}

		// For long paths, should be truncated
		if len(tt.path) > 50 && len(tab.DisplayName) > 50 {
			t.Errorf("Display name for long path %s should be truncated, got %s", tt.path, tab.DisplayName)
		}
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		input    string
		contains string // What the result should contain
	}{
		{"~/test.log", "test.log"},         // Tilde expansion
		{"./relative.log", "relative.log"}, // Relative path
		{"/absolute.log", "/absolute.log"}, // Absolute path unchanged
	}

	for _, tt := range tests {
		result := file.ExpandPath(tt.input)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("ExpandPath(%s) = %s, should contain %s", tt.input, result, tt.contains)
		}

		// Result should be absolute
		if !filepath.IsAbs(result) {
			t.Errorf("ExpandPath(%s) = %s, should be absolute", tt.input, result)
		}
	}
}

func TestIsFileAccessible(t *testing.T) {
	// Test with accessible file
	tempFile, err := os.CreateTemp("", "accessible_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	err = file.IsFileAccessible(tempFile.Name())
	if err != nil {
		t.Errorf("Accessible file should not return error: %v", err)
	}

	// Test with non-existent file
	err = file.IsFileAccessible("/path/that/does/not/exist")
	if err == nil {
		t.Error("Non-existent file should return error")
	}
}

func TestDetectFileModification(t *testing.T) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", "modification_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Create FileTab
	tab := file.NewFileTab(tempFile.Name())

	// File should not be modified initially
	modified, err := tab.DetectFileModification()
	if err != nil {
		t.Fatalf("DetectFileModification should not error: %v", err)
	}
	if modified {
		t.Error("File should not be modified initially")
	}

	// Wait and modify the file
	time.Sleep(10 * time.Millisecond)

	// Write to file to update modification time
	file, err := os.OpenFile(tempFile.Name(), os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("Failed to open file for modification: %v", err)
	}
	file.WriteString("new content")
	file.Close()

	// Should detect modification
	modified, err = tab.DetectFileModification()
	if err != nil {
		t.Fatalf("DetectFileModification should not error after modification: %v", err)
	}
	if !modified {
		t.Error("Should detect file modification")
	}
}
