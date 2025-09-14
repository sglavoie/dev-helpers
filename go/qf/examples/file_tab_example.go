// Package main provides a simple example demonstrating FileTab functionality
//
// This example shows how to create FileTab instances, load file content,
// manage view state, and work with line highlighting - core functionality
// that will be used by the qf application's UI components.
//
// Run with: go run examples/file_tab_example.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sglavoie/dev-helpers/go/qf/internal/file"
)

func main() {
	fmt.Println("FileTab Example - qf Interactive Log Filter Composer")
	fmt.Println("====================================================")

	// Create a temporary test file for demonstration
	tempFile, err := createTestFile()
	if err != nil {
		log.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tempFile)

	// Example 1: Create and load a FileTab
	fmt.Println("\n1. Creating and Loading FileTab")
	fmt.Println("-------------------------------")

	tab := file.NewFileTab(tempFile)
	fmt.Printf("Created FileTab with ID: %s\n", tab.ID)
	fmt.Printf("Path: %s\n", tab.Path)
	fmt.Printf("Display Name: %s\n", tab.GetDisplayName())
	fmt.Printf("Initially loaded: %t\n", tab.IsLoaded)

	// Load the file content
	ctx := context.Background()
	err = tab.LoadFromFile(ctx)
	if err != nil {
		log.Fatalf("Failed to load file: %v", err)
	}

	fmt.Printf("After loading - Line count: %d\n", tab.GetLineCount())
	fmt.Printf("Is loaded: %t\n", tab.IsLoaded)

	// Example 2: Display file content with line numbers
	fmt.Println("\n2. File Content Display")
	fmt.Println("----------------------")

	for i, line := range tab.Content {
		if i > 5 { // Show only first 6 lines
			fmt.Println("... (truncated)")
			break
		}
		fmt.Printf("Line %d: %s\n", line.Number, line.Content)
	}

	// Example 3: View state management
	fmt.Println("\n3. View State Management")
	fmt.Println("------------------------")

	fmt.Printf("Initial view state - Cursor: %d, Top visible: %d, Viewport height: %d\n",
		tab.ViewState.CursorLine, tab.ViewState.TopVisibleLine, tab.ViewState.ViewportHeight)

	// Scroll to middle of file
	tab.ScrollTo(6)
	fmt.Printf("After scrolling to line 6 - Cursor: %d, Top visible: %d\n",
		tab.ViewState.CursorLine, tab.ViewState.TopVisibleLine)

	// Get current view range
	viewRange := tab.GetViewRange()
	fmt.Printf("Current view shows %d lines\n", len(viewRange))

	// Example 4: Line highlighting (simulating filter matches)
	fmt.Println("\n4. Line Highlighting (Filter Simulation)")
	fmt.Println("----------------------------------------")

	// Simulate highlighting lines that match "ERROR" pattern
	for _, line := range tab.Content {
		if contains(line.Content, "ERROR") {
			tab.HighlightLine(line.Number, true)
			fmt.Printf("Highlighted line %d: %s\n", line.Number, line.Content)
		}
	}

	highlightedLines := tab.GetHighlightedLines()
	fmt.Printf("Total highlighted lines: %d\n", len(highlightedLines))

	// Example 5: Path utilities
	fmt.Println("\n5. Path Utilities")
	fmt.Println("----------------")

	// Test path expansion
	testPaths := []string{
		"~/test.log",
		"./relative.log",
		"/absolute/path/file.log",
		"simple-file.log",
	}

	for _, path := range testPaths {
		expanded := file.ExpandPath(path)
		fmt.Printf("'%s' -> '%s'\n", path, expanded)
	}

	// Example 6: File accessibility check
	fmt.Println("\n6. File Accessibility")
	fmt.Println("--------------------")

	err = file.IsFileAccessible(tempFile)
	if err != nil {
		fmt.Printf("File access error: %v\n", err)
	} else {
		fmt.Printf("File '%s' is accessible\n", filepath.Base(tempFile))
	}

	err = file.IsFileAccessible("/nonexistent/file.log")
	if err != nil {
		fmt.Printf("Expected error for nonexistent file: %v\n", err)
	}

	// Example 7: Modification tracking
	fmt.Println("\n7. Modification Tracking")
	fmt.Println("------------------------")

	fmt.Printf("Initially modified: %t\n", tab.IsModified())

	tab.SetModified()
	fmt.Printf("After SetModified(): %t\n", tab.IsModified())

	// Operations that set modified flag
	tab.Modified = false // Reset for demo
	tab.ScrollTo(1)
	fmt.Printf("After ScrollTo(): %t\n", tab.IsModified())

	fmt.Println("\n8. Advanced Features")
	fmt.Println("-------------------")

	// Get specific line
	if line, found := tab.GetLineAt(3); found {
		fmt.Printf("Line 3 content: '%s'\n", line.Content)
	}

	// Clear all highlights
	tab.ClearHighlights()
	highlightedAfterClear := tab.GetHighlightedLines()
	fmt.Printf("Highlighted lines after clear: %d\n", len(highlightedAfterClear))

	// File modification detection
	modified, err := tab.DetectFileModification()
	if err != nil {
		fmt.Printf("File modification check error: %v\n", err)
	} else {
		fmt.Printf("File modified since last access: %t\n", modified)
	}

	fmt.Println("\nFileTab example completed successfully!")
	fmt.Println("This demonstrates the core functionality that will be used")
	fmt.Println("by the qf application's UI components for file tab management.")
}

// createTestFile creates a temporary file with sample log content
func createTestFile() (string, error) {
	tempFile, err := os.CreateTemp("", "filetab_example_*.log")
	if err != nil {
		return "", err
	}

	// Write sample log content
	content := `2023-09-14 10:00:01 INFO Application started
2023-09-14 10:00:02 DEBUG Loading configuration
2023-09-14 10:00:03 INFO Database connection established
2023-09-14 10:00:04 WARNING High memory usage detected
2023-09-14 10:00:05 ERROR Failed to load user profile
2023-09-14 10:00:06 DEBUG Processing request queue
2023-09-14 10:00:07 INFO Request processed successfully
2023-09-14 10:00:08 ERROR Authentication failed
2023-09-14 10:00:09 WARNING Session timeout
2023-09-14 10:00:10 INFO Cleanup completed
2023-09-14 10:00:11 ERROR Database connection lost
2023-09-14 10:00:12 CRITICAL System shutdown initiated
`

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", err
	}

	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// contains checks if a string contains a substring (simple implementation)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

// findSubstring performs simple substring search
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
