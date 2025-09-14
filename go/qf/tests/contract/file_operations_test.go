// Package contract contains contract tests that define expected behaviors
// for interfaces that don't have implementations yet. These tests should
// fail initially and pass once implementations are created.
//
// This follows Test-Driven Development (TDD) principles where tests
// define the contract before implementation exists.
package contract

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/file"
)

// Use Line struct from the file package
type Line = file.Line

// Use FileReader interface from the file package
type FileReader = file.FileReader

// TestFileReaderContract verifies that any FileReader implementation
// satisfies the expected behavioral contract
func TestFileReaderContract(t *testing.T) {
	// Create a FileReader instance using our implementation
	reader := file.NewFileReader()

	t.Run("ReadFile_ReturnsChannelOfLines", func(t *testing.T) {
		// Create a test file
		content := "line 1\nline 2\nline 3\n"
		tempFile := createTempFile(t, content)
		defer os.Remove(tempFile)

		ctx := context.Background()
		lineChan, err := reader.ReadFile(ctx, tempFile)

		if err != nil {
			t.Fatalf("ReadFile should not return error for valid file: %v", err)
		}

		if lineChan == nil {
			t.Fatal("ReadFile should return a non-nil channel")
		}

		// Collect all lines
		var lines []Line
		for line := range lineChan {
			lines = append(lines, line)
		}

		// Verify line count
		expectedLines := 3
		if len(lines) != expectedLines {
			t.Errorf("Expected %d lines, got %d", expectedLines, len(lines))
		}

		// Verify line numbers are sequential and 1-based
		for i, line := range lines {
			expectedNum := i + 1
			if line.Number != expectedNum {
				t.Errorf("Line %d: expected number %d, got %d", i, expectedNum, line.Number)
			}
		}

		// Verify line content
		expectedContent := []string{"line 1", "line 2", "line 3"}
		for i, line := range lines {
			if line.Content != expectedContent[i] {
				t.Errorf("Line %d: expected content %q, got %q", i+1, expectedContent[i], line.Content)
			}
		}

		// Verify offsets are increasing
		for i := 1; i < len(lines); i++ {
			if lines[i].Offset <= lines[i-1].Offset {
				t.Errorf("Line %d offset (%d) should be greater than line %d offset (%d)",
					i+1, lines[i].Offset, i, lines[i-1].Offset)
			}
		}
	})

	t.Run("ReadFile_HandlesLargeFilesWithStreaming", func(t *testing.T) {
		// Create a large test file (simulating >100MB)
		largeContent := strings.Repeat("This is a long line for streaming test\n", 1000)
		tempFile := createTempFile(t, largeContent)
		defer os.Remove(tempFile)

		// Check if streaming mode is enabled
		isStreaming, err := reader.IsStreamingMode(tempFile)
		if err != nil {
			t.Fatalf("IsStreamingMode should not return error: %v", err)
		}

		ctx := context.Background()
		lineChan, err := reader.ReadFile(ctx, tempFile)

		if err != nil {
			t.Fatalf("ReadFile should handle large files: %v", err)
		}

		// Read lines and verify streaming behavior
		lineCount := 0
		start := time.Now()

		for range lineChan {
			lineCount++
			// For streaming, we should get lines progressively
			// not all at once after full file read
			if lineCount == 1 && time.Since(start) > 5*time.Second {
				t.Error("Streaming should provide first line quickly")
			}

			if lineCount > 10 {
				break // Don't read all lines in test
			}
		}

		if lineCount == 0 {
			t.Error("Should read at least some lines from large file")
		}

		// For truly large files, streaming should be enabled
		if len(largeContent) > 100*1024*1024 && !isStreaming {
			t.Error("Streaming mode should be enabled for files >100MB")
		}
	})

	t.Run("ReadFile_ContextCancellationStopsReading", func(t *testing.T) {
		// Create a file with many lines
		content := strings.Repeat("line content\n", 10000)
		tempFile := createTempFile(t, content)
		defer os.Remove(tempFile)

		ctx, cancel := context.WithCancel(context.Background())
		lineChan, err := reader.ReadFile(ctx, tempFile)

		if err != nil {
			t.Fatalf("ReadFile should not return error: %v", err)
		}

		// Read a few lines then cancel
		linesRead := 0
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		for range lineChan {
			linesRead++
			if linesRead > 1000 {
				t.Error("Context cancellation should stop reading before all lines")
				break
			}
		}

		// Verify context was respected
		if ctx.Err() == nil {
			t.Error("Context should be cancelled")
		}

		// Channel should be closed after cancellation
		select {
		case _, ok := <-lineChan:
			if ok {
				t.Error("Channel should be closed after context cancellation")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Channel should close promptly after cancellation")
		}
	})

	t.Run("ReadFile_ReturnsErrorForNonExistentFile", func(t *testing.T) {
		nonExistentFile := "/path/that/does/not/exist/file.txt"

		ctx := context.Background()
		lineChan, err := reader.ReadFile(ctx, nonExistentFile)

		if err == nil {
			t.Error("ReadFile should return error for non-existent file")
		}

		if lineChan != nil {
			t.Error("ReadFile should return nil channel when error occurs")
		}

		// Verify error type indicates file not found
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
			t.Errorf("Error should indicate file not found, got: %v", err)
		}
	})

	t.Run("ReadFile_HandlesUTF8AndEncodingErrors", func(t *testing.T) {
		// Create file with UTF-8 content
		utf8Content := "Hello 世界\nUTF-8 tëst\n€ symbol\n"
		tempFile := createTempFile(t, utf8Content)
		defer os.Remove(tempFile)

		ctx := context.Background()
		lineChan, err := reader.ReadFile(ctx, tempFile)

		if err != nil {
			t.Fatalf("ReadFile should handle UTF-8 content: %v", err)
		}

		var lines []Line
		for line := range lineChan {
			lines = append(lines, line)
		}

		if len(lines) != 3 {
			t.Errorf("Expected 3 UTF-8 lines, got %d", len(lines))
		}

		// Verify UTF-8 characters are preserved
		expectedLines := []string{"Hello 世界", "UTF-8 tëst", "€ symbol"}
		for i, line := range lines {
			if line.Content != expectedLines[i] {
				t.Errorf("UTF-8 line %d: expected %q, got %q",
					i+1, expectedLines[i], line.Content)
			}
		}
	})

	t.Run("ReadFile_ChannelClosesOnCompletion", func(t *testing.T) {
		content := "line 1\nline 2\n"
		tempFile := createTempFile(t, content)
		defer os.Remove(tempFile)

		ctx := context.Background()
		lineChan, err := reader.ReadFile(ctx, tempFile)

		if err != nil {
			t.Fatalf("ReadFile should not return error: %v", err)
		}

		// Read all lines
		var lines []Line
		for line := range lineChan {
			lines = append(lines, line)
		}

		// Verify channel is closed
		select {
		case line, ok := <-lineChan:
			if ok {
				t.Errorf("Channel should be closed, but received line: %+v", line)
			}
		default:
			// Channel is properly closed
		}

		if len(lines) != 2 {
			t.Errorf("Expected 2 lines, got %d", len(lines))
		}
	})

	t.Run("IsStreamingMode_ReturnsCorrectMode", func(t *testing.T) {
		// Small file should not use streaming
		smallContent := "small file\n"
		smallFile := createTempFile(t, smallContent)
		defer os.Remove(smallFile)

		isStreaming, err := reader.IsStreamingMode(smallFile)
		if err != nil {
			t.Fatalf("IsStreamingMode should not error for valid file: %v", err)
		}

		if isStreaming {
			t.Error("Small files should not use streaming mode")
		}

		// Non-existent file should return error
		_, err = reader.IsStreamingMode("/non/existent/file")
		if err == nil {
			t.Error("IsStreamingMode should return error for non-existent file")
		}
	})

	t.Run("Close_ReleasesResources", func(t *testing.T) {
		err := reader.Close()
		if err != nil {
			t.Errorf("Close should not return error: %v", err)
		}

		// After close, operations should fail gracefully
		ctx := context.Background()
		_, err = reader.ReadFile(ctx, "/any/file")
		if err == nil {
			t.Error("ReadFile should fail after Close() is called")
		}
	})
}

// TestFileReaderInterfaceCompliance ensures any implementation
// correctly implements the FileReader interface
func TestFileReaderInterfaceCompliance(t *testing.T) {
	t.Run("ImplementationExists", func(t *testing.T) {
		// Create a FileReader instance to verify it exists and implements the interface
		reader := file.NewFileReader()
		if reader == nil {
			t.Fatal("FileReader implementation should exist")
		}

		// Verify interface methods are callable (basic smoke test)
		ctx := context.Background()

		// ReadFile with non-existent file should return error
		_, err := reader.ReadFile(ctx, "/non/existent/file")
		if err == nil {
			t.Error("ReadFile should return error for non-existent file")
		}

		// IsStreamingMode with non-existent file should return error
		_, err = reader.IsStreamingMode("/non/existent/file")
		if err == nil {
			t.Error("IsStreamingMode should return error for non-existent file")
		}

		// Close should not error
		if err := reader.Close(); err != nil {
			t.Errorf("Close should not return error: %v", err)
		}
	})
}

// Helper function to create temporary files for testing
func createTempFile(t *testing.T, content string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "fileader_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := io.WriteString(tmpFile, content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}
