// Package file integration tests for FileReader with FileTab integration
package file

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFileReaderIntegrationWithFileTab(t *testing.T) {
	// This test verifies that FileReader works well with the existing FileTab system

	t.Run("FileReader_WithFileTab_Integration", func(t *testing.T) {
		// Create test content
		testContent := "Line 1\nLine 2 with special chars: 世界\nLine 3\n"

		// Create temporary file
		tempFile, err := os.CreateTemp("", "integration_test_*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		if _, err := tempFile.WriteString(testContent); err != nil {
			t.Fatalf("Failed to write test content: %v", err)
		}
		tempFile.Close()

		// Create FileReader
		reader := NewFileReader()

		// Create FileTab
		fileTab := NewFileTab(tempFile.Name())

		// Test that FileReader can read the same file that FileTab would load
		ctx := context.Background()
		lineChan, err := reader.ReadFile(ctx, tempFile.Name())
		if err != nil {
			t.Fatalf("FileReader failed to read file: %v", err)
		}

		// Collect lines from FileReader
		var readerLines []Line
		for line := range lineChan {
			readerLines = append(readerLines, line)
		}

		// Load file into FileTab
		if err := fileTab.LoadFromFile(ctx); err != nil {
			t.Fatalf("FileTab failed to load file: %v", err)
		}

		// Verify both approaches read the same content
		if len(readerLines) != len(fileTab.Content) {
			t.Errorf("FileReader got %d lines, FileTab got %d lines",
				len(readerLines), len(fileTab.Content))
		}

		// Verify line content matches
		expectedLines := []string{"Line 1", "Line 2 with special chars: 世界", "Line 3"}
		for i, expectedLine := range expectedLines {
			if i >= len(readerLines) {
				t.Errorf("FileReader missing line %d", i+1)
				continue
			}
			if i >= len(fileTab.Content) {
				t.Errorf("FileTab missing line %d", i+1)
				continue
			}

			if readerLines[i].Content != expectedLine {
				t.Errorf("FileReader line %d: expected %q, got %q",
					i+1, expectedLine, readerLines[i].Content)
			}
			if fileTab.Content[i].Content != expectedLine {
				t.Errorf("FileTab line %d: expected %q, got %q",
					i+1, expectedLine, fileTab.Content[i].Content)
			}

			// Verify line numbers match
			if readerLines[i].Number != i+1 {
				t.Errorf("FileReader line %d has wrong number: %d", i+1, readerLines[i].Number)
			}
			if fileTab.Content[i].Number != i+1 {
				t.Errorf("FileTab line %d has wrong number: %d", i+1, fileTab.Content[i].Number)
			}
		}

		// Clean up
		reader.Close()
	})

	t.Run("FileReader_StreamingMode_Compatible", func(t *testing.T) {
		// Test that FileReader's streaming mode detection works correctly
		reader := NewFileReader(WithStreamingThreshold(1)) // 1MB threshold

		// Create a small file - should not use streaming
		smallFile, err := os.CreateTemp("", "small_test_*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(smallFile.Name())

		smallContent := "Small file content\n"
		if _, err := smallFile.WriteString(smallContent); err != nil {
			t.Fatalf("Failed to write small content: %v", err)
		}
		smallFile.Close()

		isStreaming, err := reader.IsStreamingMode(smallFile.Name())
		if err != nil {
			t.Fatalf("IsStreamingMode failed: %v", err)
		}
		if isStreaming {
			t.Error("Small file should not use streaming mode")
		}

		// Create a larger file - should use streaming
		largeFile, err := os.CreateTemp("", "large_test_*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(largeFile.Name())

		// Create content larger than 1MB
		largeContent := strings.Repeat("This is a long line for testing streaming mode\n", 25000)
		if _, err := largeFile.WriteString(largeContent); err != nil {
			t.Fatalf("Failed to write large content: %v", err)
		}
		largeFile.Close()

		isStreaming, err = reader.IsStreamingMode(largeFile.Name())
		if err != nil {
			t.Fatalf("IsStreamingMode failed for large file: %v", err)
		}
		if !isStreaming {
			t.Error("Large file should use streaming mode")
		}

		// Clean up
		reader.Close()
	})

	t.Run("FileReader_ContextCancellation_DoesNotBlock", func(t *testing.T) {
		// Test that context cancellation works properly and doesn't block
		reader := NewFileReader()
		defer reader.Close()

		// Create a file with moderate content
		tempFile, err := os.CreateTemp("", "cancel_test_*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		content := strings.Repeat("Line content for cancellation test\n", 1000)
		if _, err := tempFile.WriteString(content); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
		tempFile.Close()

		ctx, cancel := context.WithCancel(context.Background())
		lineChan, err := reader.ReadFile(ctx, tempFile.Name())
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		// Read some lines then cancel
		linesRead := 0
		go func() {
			time.Sleep(5 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		for range lineChan {
			linesRead++
			if linesRead > 500 {
				t.Error("Context cancellation should stop reading")
				break
			}
		}

		elapsed := time.Since(start)
		if elapsed > 100*time.Millisecond {
			t.Errorf("Cancellation took too long: %v", elapsed)
		}

		if ctx.Err() == nil {
			t.Error("Context should be cancelled")
		}
	})
}

func TestFileReaderConfigurationCompatibility(t *testing.T) {
	// Test that FileReader works with different configuration values
	// that match the application's configuration system

	t.Run("FileReader_WithConfigValues", func(t *testing.T) {
		// Test with default config values (matching config.go defaults)
		reader := NewFileReaderFromConfig(100) // 100MB threshold from default config
		defer reader.Close()

		// Verify the reader was created successfully
		if reader == nil {
			t.Fatal("FileReader should be created with config values")
		}

		// Create a test file
		tempFile, err := os.CreateTemp("", "config_test_*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		content := "Test line for configuration compatibility\n"
		if _, err := tempFile.WriteString(content); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
		tempFile.Close()

		// Test that the reader works with the configuration
		ctx := context.Background()
		lineChan, err := reader.ReadFile(ctx, tempFile.Name())
		if err != nil {
			t.Fatalf("ReadFile failed with config: %v", err)
		}

		// Read the line
		var lines []Line
		for line := range lineChan {
			lines = append(lines, line)
		}

		if len(lines) != 1 {
			t.Errorf("Expected 1 line, got %d", len(lines))
		}

		if len(lines) > 0 && lines[0].Content != "Test line for configuration compatibility" {
			t.Errorf("Unexpected line content: %q", lines[0].Content)
		}
	})
}
