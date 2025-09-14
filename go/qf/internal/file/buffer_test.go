package file

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestCircularBuffer verifies circular buffer functionality
func TestCircularBuffer(t *testing.T) {
	t.Run("BasicOperations", func(t *testing.T) {
		cb := NewCircularBuffer(3)

		// Test empty buffer
		size, capacity, totalIn, totalOut := cb.GetStats()
		if size != 0 || capacity != 3 || totalIn != 0 || totalOut != 0 {
			t.Errorf("Expected empty buffer stats (0,3,0,0), got (%d,%d,%d,%d)", size, capacity, totalIn, totalOut)
		}

		// Add lines
		for i := 1; i <= 3; i++ {
			line := LineBuffer{
				Number:  int32(i),
				Content: fmt.Sprintf("Line %d", i),
				Offset:  int64(i * 10),
			}
			cb.AddLine(line)
		}

		// Verify stats
		size, capacity, totalIn, totalOut = cb.GetStats()
		if size != 3 || totalIn != 3 || totalOut != 0 {
			t.Errorf("Expected buffer stats (3,3,3,0), got (%d,%d,%d,%d)", size, capacity, totalIn, totalOut)
		}

		// Test retrieval
		line, found := cb.GetLine(2)
		if !found || line.Number != 2 || line.Content != "Line 2" {
			t.Errorf("Failed to retrieve line 2: found=%v, number=%d, content=%q", found, line.Number, line.Content)
		}
	})

	t.Run("CircularOverflow", func(t *testing.T) {
		cb := NewCircularBuffer(2)

		// Fill buffer beyond capacity
		for i := 1; i <= 4; i++ {
			line := LineBuffer{
				Number:  int32(i),
				Content: fmt.Sprintf("Line %d", i),
			}
			cb.AddLine(line)
		}

		// Should only contain last 2 lines
		size, _, totalIn, totalOut := cb.GetStats()
		if size != 2 || totalIn != 4 || totalOut != 2 {
			t.Errorf("Expected overflow stats (2,_,4,2), got (%d,_,%d,%d)", size, totalIn, totalOut)
		}

		// First two lines should be evicted
		if _, found := cb.GetLine(1); found {
			t.Error("Line 1 should have been evicted")
		}
		if _, found := cb.GetLine(2); found {
			t.Error("Line 2 should have been evicted")
		}

		// Last two lines should be present
		if _, found := cb.GetLine(3); !found {
			t.Error("Line 3 should be present")
		}
		if _, found := cb.GetLine(4); !found {
			t.Error("Line 4 should be present")
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		cb := NewCircularBuffer(100)

		// Start concurrent writers
		done := make(chan bool, 2)

		go func() {
			for i := 1; i <= 50; i++ {
				line := LineBuffer{
					Number:  int32(i),
					Content: fmt.Sprintf("Writer1 Line %d", i),
				}
				cb.AddLine(line)
			}
			done <- true
		}()

		go func() {
			for i := 51; i <= 100; i++ {
				line := LineBuffer{
					Number:  int32(i),
					Content: fmt.Sprintf("Writer2 Line %d", i),
				}
				cb.AddLine(line)
			}
			done <- true
		}()

		// Wait for writers to complete
		<-done
		<-done

		// Verify all lines were added
		size, _, totalIn, _ := cb.GetStats()
		if size != 100 || totalIn != 100 {
			t.Errorf("Expected 100 lines, got size=%d, totalIn=%d", size, totalIn)
		}
	})
}

// TestFileBuffer verifies memory-mapped file buffer functionality
func TestFileBuffer(t *testing.T) {
	t.Run("BasicFileOperations", func(t *testing.T) {
		// Create test file
		testFile := createTestFile(t, []string{
			"First line",
			"Second line with more content",
			"Third line",
		})
		defer os.Remove(testFile)

		// Create file buffer
		fb, err := NewFileBuffer(testFile)
		if err != nil {
			t.Fatalf("Failed to create file buffer: %v", err)
		}
		defer fb.Release()

		// Wait for line indexing to complete
		time.Sleep(100 * time.Millisecond)

		// Test line retrieval
		line, err := fb.GetLine(1)
		if err != nil {
			t.Fatalf("Failed to get line 1: %v", err)
		}
		if line.Content != "First line" {
			t.Errorf("Expected 'First line', got %q", line.Content)
		}

		line, err = fb.GetLine(2)
		if err != nil {
			t.Fatalf("Failed to get line 2: %v", err)
		}
		if line.Content != "Second line with more content" {
			t.Errorf("Expected 'Second line with more content', got %q", line.Content)
		}

		// Test range retrieval
		lines, err := fb.GetRange(1, 3)
		if err != nil {
			t.Fatalf("Failed to get line range: %v", err)
		}
		if len(lines) != 3 {
			t.Errorf("Expected 3 lines, got %d", len(lines))
		}
	})

	t.Run("EmptyFile", func(t *testing.T) {
		testFile := createTestFile(t, []string{})
		defer os.Remove(testFile)

		fb, err := NewFileBuffer(testFile)
		if err != nil {
			t.Fatalf("Failed to create file buffer for empty file: %v", err)
		}
		defer fb.Release()

		// Should handle empty file gracefully
		_, err = fb.GetLine(1)
		if err == nil {
			t.Error("Expected error for line 1 in empty file")
		}
	})

	t.Run("ReferenceCountingAndCleanup", func(t *testing.T) {
		testFile := createTestFile(t, []string{"Test line"})
		defer os.Remove(testFile)

		fb, err := NewFileBuffer(testFile)
		if err != nil {
			t.Fatalf("Failed to create file buffer: %v", err)
		}

		// Add reference
		fb.AddRef()

		// First release should not clean up
		err = fb.Release()
		if err != nil {
			t.Errorf("First release failed: %v", err)
		}

		// Second release should clean up
		err = fb.Release()
		if err != nil {
			t.Errorf("Second release failed: %v", err)
		}

		// Should be closed now
		_, err = fb.GetLine(1)
		if err == nil {
			t.Error("Expected error accessing closed file buffer")
		}
	})
}

// TestBufferManager verifies the integrated buffer manager functionality
func TestBufferManager(t *testing.T) {
	t.Run("SmallFileHandling", func(t *testing.T) {
		config := DefaultBufferConfig
		config.MemoryMapThresholdBytes = 1024 * 1024 // 1MB threshold
		bm := NewBufferManager(config)
		defer bm.Close()

		// Create small test file
		testLines := make([]string, 100)
		for i := 0; i < 100; i++ {
			testLines[i] = fmt.Sprintf("Test line %d with some content", i+1)
		}
		testFile := createTestFile(t, testLines)
		defer os.Remove(testFile)

		// Load file
		ctx := context.Background()
		var progressUpdates []string
		progressCallback := func(loaded, total int64, phase string) {
			progressUpdates = append(progressUpdates, phase)
		}

		err := bm.OpenFile(ctx, testFile, progressCallback)
		if err != nil {
			t.Fatalf("Failed to open file: %v", err)
		}

		// Verify not using streaming mode for small file
		if bm.IsStreaming() {
			t.Error("Should not use streaming mode for small file")
		}

		// Test line retrieval
		line, err := bm.GetLine(50)
		if err != nil {
			t.Fatalf("Failed to get line 50: %v", err)
		}
		expected := "Test line 50 with some content"
		if line.Content != expected {
			t.Errorf("Expected %q, got %q", expected, line.Content)
		}

		// Verify progress updates occurred
		if len(progressUpdates) == 0 {
			t.Error("Expected progress updates")
		}

		// Check final progress
		progress := bm.GetProgress()
		if progress.Phase != "complete" {
			t.Errorf("Expected complete phase, got %q", progress.Phase)
		}
	})

	t.Run("LargeFileHandling", func(t *testing.T) {
		config := DefaultBufferConfig
		config.MemoryMapThresholdBytes = 100 // Very low threshold for testing
		config.CircularBufferLines = 10
		config.EnableAsync = false // Synchronous for testing
		bm := NewBufferManager(config)
		defer bm.Close()

		// Create "large" test file (larger than threshold)
		testLines := make([]string, 50)
		for i := 0; i < 50; i++ {
			testLines[i] = fmt.Sprintf("Large file line %d with substantial content to exceed threshold", i+1)
		}
		testFile := createTestFile(t, testLines)
		defer os.Remove(testFile)

		// Load file
		ctx := context.Background()
		progressUpdates := make(map[string]int)
		progressCallback := func(loaded, total int64, phase string) {
			progressUpdates[phase]++
		}

		err := bm.OpenFile(ctx, testFile, progressCallback)
		if err != nil {
			t.Fatalf("Failed to open file: %v", err)
		}

		// Should use streaming mode
		if !bm.IsStreaming() {
			t.Error("Should use streaming mode for large file")
		}

		// Wait for initial loading to complete
		time.Sleep(200 * time.Millisecond)

		// Test line retrieval
		line, err := bm.GetLine(5)
		if err != nil {
			t.Fatalf("Failed to get line 5: %v", err)
		}
		expected := "Large file line 5 with substantial content to exceed threshold"
		if line.Content != expected {
			t.Errorf("Expected %q, got %q", expected, line.Content)
		}

		// Verify progress phases
		if progressUpdates["loading"] == 0 {
			t.Error("Expected loading phase updates")
		}
	})

	t.Run("ContextLoading", func(t *testing.T) {
		bm := NewBufferManager(DefaultBufferConfig)
		defer bm.Close()

		// Create test file
		testLines := make([]string, 20)
		for i := 0; i < 20; i++ {
			testLines[i] = fmt.Sprintf("Context line %d", i+1)
		}
		testFile := createTestFile(t, testLines)
		defer os.Remove(testFile)

		// Load file
		ctx := context.Background()
		err := bm.OpenFile(ctx, testFile, nil)
		if err != nil {
			t.Fatalf("Failed to open file: %v", err)
		}

		// Test context loading around line 10
		contextLines, err := bm.LoadContext(ctx, 10, 6)
		if err != nil {
			t.Fatalf("Failed to load context: %v", err)
		}

		// Should get lines around line 10 (7-13 with window size 6)
		if len(contextLines) == 0 {
			t.Error("Expected context lines")
		}

		// Verify context contains the center line
		foundCenter := false
		for _, line := range contextLines {
			if line.Number == 10 {
				foundCenter = true
				if line.Content != "Context line 10" {
					t.Errorf("Expected 'Context line 10', got %q", line.Content)
				}
				break
			}
		}
		if !foundCenter {
			t.Error("Context should contain the center line")
		}
	})

	t.Run("MemoryUsageTracking", func(t *testing.T) {
		bm := NewBufferManager(DefaultBufferConfig)
		defer bm.Close()

		initialMemory := bm.GetMemoryUsage()
		if initialMemory != 0 {
			t.Errorf("Expected 0 initial memory usage, got %d", initialMemory)
		}

		// Create test file
		testLines := make([]string, 100)
		for i := 0; i < 100; i++ {
			testLines[i] = strings.Repeat("A", 100) // 100 chars per line
		}
		testFile := createTestFile(t, testLines)
		defer os.Remove(testFile)

		// Load file
		ctx := context.Background()
		err := bm.OpenFile(ctx, testFile, nil)
		if err != nil {
			t.Fatalf("Failed to open file: %v", err)
		}

		// Memory usage should increase
		finalMemory := bm.GetMemoryUsage()
		if finalMemory <= initialMemory {
			t.Errorf("Expected memory usage to increase from %d to >%d", initialMemory, finalMemory)
		}

		// Check stats
		stats := bm.GetStats()
		if memUsage, exists := stats["memory_usage_mb"]; !exists || memUsage.(int64) < 0 {
			t.Error("Expected valid memory usage in stats")
		}
	})

	t.Run("CancellationSupport", func(t *testing.T) {
		bm := NewBufferManager(DefaultBufferConfig)
		defer bm.Close()

		// Create large test file
		testLines := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			testLines[i] = fmt.Sprintf("Cancellation test line %d", i+1)
		}
		testFile := createTestFile(t, testLines)
		defer os.Remove(testFile)

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		// Should be cancelled due to timeout
		err := bm.OpenFile(ctx, testFile, nil)
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context deadline exceeded, got: %v", err)
		}
	})
}

// TestPerformanceRequirements verifies performance meets specifications
func TestPerformanceRequirements(t *testing.T) {
	t.Run("CircularBufferPerformance", func(t *testing.T) {
		cb := NewCircularBuffer(10000)

		// Measure time to add 10K lines
		start := time.Now()
		for i := 1; i <= 10000; i++ {
			line := LineBuffer{
				Number:  int32(i),
				Content: fmt.Sprintf("Performance test line %d", i),
			}
			cb.AddLine(line)
		}
		addTime := time.Since(start)

		// Should be fast (arbitrary limit for testing)
		if addTime > 100*time.Millisecond {
			t.Errorf("Adding 10K lines took too long: %v", addTime)
		}

		// Measure retrieval time
		start = time.Now()
		for i := 1; i <= 1000; i++ {
			cb.GetLine(i)
		}
		retrievalTime := time.Since(start)

		// Should be fast
		if retrievalTime > 50*time.Millisecond {
			t.Errorf("Retrieving 1K lines took too long: %v", retrievalTime)
		}

		t.Logf("Performance: Add 10K lines: %v, Retrieve 1K lines: %v", addTime, retrievalTime)
	})

	t.Run("BufferManagerResponseTime", func(t *testing.T) {
		bm := NewBufferManager(DefaultBufferConfig)
		defer bm.Close()

		// Create moderate test file
		testLines := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			testLines[i] = fmt.Sprintf("Response time test line %d", i+1)
		}
		testFile := createTestFile(t, testLines)
		defer os.Remove(testFile)

		// Measure file opening time
		ctx := context.Background()
		start := time.Now()
		err := bm.OpenFile(ctx, testFile, nil)
		if err != nil {
			t.Fatalf("Failed to open file: %v", err)
		}
		openTime := time.Since(start)

		// Should open reasonably quickly
		if openTime > 1*time.Second {
			t.Errorf("File opening took too long: %v", openTime)
		}

		// Measure line access time
		start = time.Now()
		for i := 1; i <= 100; i++ {
			bm.GetLine(i)
		}
		accessTime := time.Since(start)

		// Should access lines quickly
		if accessTime > 50*time.Millisecond {
			t.Errorf("Line access took too long: %v", accessTime)
		}

		t.Logf("Performance: File open: %v, Access 100 lines: %v", openTime, accessTime)
	})
}

// Helper function to create test files
func createTestFile(t *testing.T, lines []string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "buffer_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(tmpFile, line); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			t.Fatalf("Failed to write line: %v", err)
		}
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}
