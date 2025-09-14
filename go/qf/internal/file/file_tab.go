// Package file provides file-related data structures and operations for the qf application.
//
// This package contains the FileTab model and related functionality for managing
// file tabs in the interactive log filter composer. It supports multi-file handling,
// streaming for large files, and view state management.
package file

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Line represents a single line from a file with metadata
type Line struct {
	Number      int    // 1-based line number in the file
	Content     string // Line text content without newline
	Offset      int64  // Byte offset in the file
	Highlighted bool   // Whether this line matches current filters and should be highlighted
}

// ViewState represents the current view state of a file tab
type ViewState struct {
	ScrollPosition int // Current vertical scroll position
	CursorLine     int // Current line under cursor (1-based)
	ViewportHeight int // Number of visible lines in viewport
	TopVisibleLine int // First visible line number (1-based)
}

// FileTab represents a file tab in the application with content and metadata
type FileTab struct {
	ID           string    // Unique identifier for this tab
	Path         string    // Absolute file path
	DisplayName  string    // Tab display name (computed from path)
	Content      []Line    // File lines with metadata
	IsLoaded     bool      // Whether file content has been loaded
	Modified     bool      // Whether the tab state has been modified
	LastAccessed time.Time // Last time this tab was accessed
	ViewState    ViewState // Current view state and cursor position
}

// NewFileTab creates a new FileTab instance for the specified file path
func NewFileTab(path string) *FileTab {
	// Generate unique ID
	id := generateUUID()

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path // fallback to original path
	}

	// Generate display name
	displayName := generateDisplayName(absPath)

	return &FileTab{
		ID:           id,
		Path:         absPath,
		DisplayName:  displayName,
		Content:      []Line{},
		IsLoaded:     false,
		Modified:     false,
		LastAccessed: time.Now(),
		ViewState: ViewState{
			ScrollPosition: 0,
			CursorLine:     1,
			ViewportHeight: 25, // Default terminal height
			TopVisibleLine: 1,
		},
	}
}

// LoadFromFile loads the file content into the tab
// Returns error if file cannot be read
func (ft *FileTab) LoadFromFile(ctx context.Context) error {
	if ft.IsLoaded {
		return nil // Already loaded
	}

	file, err := os.Open(ft.Path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", ft.Path, err)
	}
	defer file.Close()

	// Get file info for streaming decision
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", ft.Path, err)
	}

	// Clear existing content
	ft.Content = []Line{}

	// Use streaming for large files (>10MB threshold for now)
	const streamingThreshold = 10 * 1024 * 1024
	useStreaming := fileInfo.Size() > streamingThreshold

	if useStreaming {
		return ft.loadWithStreaming(ctx, file)
	}

	return ft.loadInMemory(ctx, file)
}

// loadInMemory loads the entire file into memory
func (ft *FileTab) loadInMemory(ctx context.Context, file *os.File) error {
	scanner := bufio.NewScanner(file)
	lineNumber := 1
	var offset int64

	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		content := scanner.Text()

		ft.Content = append(ft.Content, Line{
			Number:      lineNumber,
			Content:     content,
			Offset:      offset,
			Highlighted: false,
		})

		// Update offset for next line (content + newline)
		offset += int64(len(content) + 1)
		lineNumber++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file %s: %w", ft.Path, err)
	}

	ft.IsLoaded = true
	ft.UpdateLastAccessed()
	return nil
}

// loadWithStreaming loads file using streaming for large files
func (ft *FileTab) loadWithStreaming(ctx context.Context, file *os.File) error {
	// For streaming, we'll load the first chunk of lines for immediate display
	// and mark as loaded. The full streaming implementation would be handled
	// by a separate streaming reader component.

	scanner := bufio.NewScanner(file)
	lineNumber := 1
	var offset int64
	const initialLoadLines = 1000 // Load first 1000 lines for immediate display

	for scanner.Scan() && lineNumber <= initialLoadLines {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		content := scanner.Text()

		ft.Content = append(ft.Content, Line{
			Number:      lineNumber,
			Content:     content,
			Offset:      offset,
			Highlighted: false,
		})

		offset += int64(len(content) + 1)
		lineNumber++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file %s: %w", ft.Path, err)
	}

	ft.IsLoaded = true
	ft.UpdateLastAccessed()
	return nil
}

// GetDisplayName returns the display name for the tab
func (ft *FileTab) GetDisplayName() string {
	return ft.DisplayName
}

// SetModified marks the tab as modified
func (ft *FileTab) SetModified() {
	ft.Modified = true
}

// IsModified returns whether the tab has been modified
func (ft *FileTab) IsModified() bool {
	return ft.Modified
}

// UpdateLastAccessed updates the last accessed timestamp
func (ft *FileTab) UpdateLastAccessed() {
	ft.LastAccessed = time.Now()
}

// GetLineCount returns the total number of lines in the file
func (ft *FileTab) GetLineCount() int {
	return len(ft.Content)
}

// GetViewRange returns the lines that should be visible given the current view state
func (ft *FileTab) GetViewRange() []Line {
	if len(ft.Content) == 0 {
		return []Line{}
	}

	// Convert to 0-based indexing
	startIndex := ft.ViewState.TopVisibleLine - 1
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex >= len(ft.Content) {
		startIndex = len(ft.Content) - 1
	}

	endIndex := startIndex + ft.ViewState.ViewportHeight
	if endIndex > len(ft.Content) {
		endIndex = len(ft.Content)
	}

	return ft.Content[startIndex:endIndex]
}

// UpdateViewState updates the view state parameters
func (ft *FileTab) UpdateViewState(scrollPos, cursorLine, viewportHeight int) {
	ft.ViewState.ScrollPosition = scrollPos
	ft.ViewState.CursorLine = cursorLine
	ft.ViewState.ViewportHeight = viewportHeight

	// Update top visible line based on scroll position
	ft.ViewState.TopVisibleLine = scrollPos + 1

	ft.SetModified()
}

// ScrollTo scrolls to a specific line number (1-based)
func (ft *FileTab) ScrollTo(lineNumber int) {
	if lineNumber < 1 {
		lineNumber = 1
	}

	totalLines := len(ft.Content)
	if lineNumber > totalLines {
		lineNumber = totalLines
	}

	// Center the line in the viewport if possible
	centerOffset := ft.ViewState.ViewportHeight / 2
	scrollPosition := lineNumber - centerOffset - 1

	if scrollPosition < 0 {
		scrollPosition = 0
	}

	ft.ViewState.ScrollPosition = scrollPosition
	ft.ViewState.TopVisibleLine = scrollPosition + 1
	ft.ViewState.CursorLine = lineNumber
	ft.SetModified()
}

// GetLineAt returns the line at the specified 1-based line number
func (ft *FileTab) GetLineAt(lineNumber int) (Line, bool) {
	if lineNumber < 1 || lineNumber > len(ft.Content) {
		return Line{}, false
	}

	return ft.Content[lineNumber-1], true
}

// HighlightLine sets the highlighted state for a specific line
func (ft *FileTab) HighlightLine(lineNumber int, highlighted bool) {
	if lineNumber < 1 || lineNumber > len(ft.Content) {
		return
	}

	ft.Content[lineNumber-1].Highlighted = highlighted
	ft.SetModified()
}

// ClearHighlights removes highlighting from all lines
func (ft *FileTab) ClearHighlights() {
	for i := range ft.Content {
		ft.Content[i].Highlighted = false
	}
	ft.SetModified()
}

// GetHighlightedLines returns all lines that are currently highlighted
func (ft *FileTab) GetHighlightedLines() []Line {
	var highlighted []Line
	for _, line := range ft.Content {
		if line.Highlighted {
			highlighted = append(highlighted, line)
		}
	}
	return highlighted
}

// Helper functions

// generateUUID creates a simple UUID-like string using crypto/rand
func generateUUID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("tab-%d", time.Now().UnixNano())
	}

	// Format as UUID-like string: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// generateDisplayName creates a smart display name from a file path
func generateDisplayName(path string) string {
	// Get the base filename
	base := filepath.Base(path)

	// If the path is short, use the full path
	if len(path) <= 30 {
		return path
	}

	// For longer paths, show the filename and parent directory
	dir := filepath.Dir(path)
	parentDir := filepath.Base(dir)

	if parentDir == "." || parentDir == "/" {
		return base
	}

	displayName := filepath.Join(parentDir, base)

	// If still too long, truncate the beginning
	if len(displayName) > 30 {
		return "..." + displayName[len(displayName)-27:]
	}

	return displayName
}

// Note: ExpandPath and IsFileAccessible functions have been moved to error_handling.go
// to avoid duplication and provide centralized file utilities.

// DetectFileModification checks if a file has been modified since last access
func (ft *FileTab) DetectFileModification() (bool, error) {
	fileInfo, err := os.Stat(ft.Path)
	if err != nil {
		return false, fmt.Errorf("failed to stat file %s: %w", ft.Path, err)
	}

	modTime := fileInfo.ModTime()
	return modTime.After(ft.LastAccessed), nil
}
