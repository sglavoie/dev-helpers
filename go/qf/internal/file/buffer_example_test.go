package file

import (
	"context"
	"fmt"
	"os"
	"time"
)

// ExampleCircularBuffer demonstrates basic circular buffer usage
func ExampleCircularBuffer() {
	// Create a circular buffer with capacity for 5 lines
	cb := NewCircularBuffer(5)

	// Add some lines
	for i := 1; i <= 7; i++ {
		line := LineBuffer{
			Number:  int32(i),
			Content: fmt.Sprintf("Line %d", i),
			Offset:  int64(i * 10),
		}
		cb.AddLine(line)
	}

	// Buffer only keeps last 5 lines (3-7), first 2 are evicted
	size, capacity, totalIn, totalOut := cb.GetStats()
	fmt.Printf("Buffer stats: size=%d, capacity=%d, totalIn=%d, totalOut=%d\n",
		size, capacity, totalIn, totalOut)

	// Retrieve a line
	if line, found := cb.GetLine(5); found {
		fmt.Printf("Found line %d: %s\n", line.Number, line.Content)
	}

	// First lines are evicted
	if _, found := cb.GetLine(1); !found {
		fmt.Println("Line 1 was evicted from circular buffer")
	}

	// Output:
	// Buffer stats: size=5, capacity=5, totalIn=7, totalOut=2
	// Found line 5: Line 5
	// Line 1 was evicted from circular buffer
}

// ExampleBufferManager demonstrates complete buffer manager usage
func ExampleBufferManager() {
	// Create buffer manager with custom configuration
	config := DefaultBufferConfig
	config.CircularBufferLines = 1000 // Keep last 1000 lines
	config.ContextWindowSize = 20     // 20 lines context window
	config.MaxMemoryUsageMB = 50      // 50MB memory limit

	bm := NewBufferManager(config)
	defer bm.Close()

	// Create a sample file
	tmpFile, _ := os.CreateTemp("", "example_*.log")
	defer os.Remove(tmpFile.Name())

	// Write sample content
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(tmpFile, "Log entry %d: Some important information here\n", i)
	}
	tmpFile.Close()

	// Progress tracking callback
	progressCallback := func(loaded, total int64, phase string) {
		if phase == "complete" {
			fmt.Printf("Loading complete: %d lines processed\n", loaded)
		}
	}

	// Open and load the file
	ctx := context.Background()
	err := bm.OpenFile(ctx, tmpFile.Name(), progressCallback)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}

	// Access a specific line
	line, err := bm.GetLine(50)
	if err != nil {
		fmt.Printf("Error getting line: %v\n", err)
		return
	}

	fmt.Printf("Line 50: %s\n", line.Content)

	// Load context around a line
	contextLines, err := bm.LoadContext(ctx, 50, 5)
	if err == nil {
		fmt.Printf("Context around line 50: %d lines loaded\n", len(contextLines))
	}

	// Get memory usage statistics
	memUsage := bm.GetMemoryUsage()
	fmt.Printf("Memory usage: %d bytes\n", memUsage)

	// Get detailed statistics
	stats := bm.GetStats()
	fmt.Printf("Circular buffer size: %v\n", stats["circular_buffer_size"])

	// Output:
	// Loading complete: 100 lines processed
	// Line 50: Log entry 50: Some important information here
	// Context around line 50: 5 lines loaded
	// Memory usage: [varies]
	// Circular buffer size: 100
}

// ExampleFileBuffer demonstrates memory-mapped file buffer usage
func ExampleFileBuffer() {
	// Create a sample file
	tmpFile, _ := os.CreateTemp("", "filebuffer_*.txt")
	defer os.Remove(tmpFile.Name())

	// Write sample content
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(tmpFile, "Memory mapped line %d\n", i)
	}
	tmpFile.Close()

	// Create memory-mapped file buffer
	fb, err := NewFileBuffer(tmpFile.Name())
	if err != nil {
		fmt.Printf("Error creating file buffer: %v\n", err)
		return
	}
	defer fb.Release()

	// Wait for line indexing to complete
	time.Sleep(50 * time.Millisecond)

	// Access specific lines
	line, err := fb.GetLine(3)
	if err != nil {
		fmt.Printf("Error getting line: %v\n", err)
		return
	}

	fmt.Printf("Line 3: %s\n", line.Content)

	// Get a range of lines
	lines, err := fb.GetRange(5, 7)
	if err != nil {
		fmt.Printf("Error getting range: %v\n", err)
		return
	}

	fmt.Printf("Lines 5-7: %d lines retrieved\n", len(lines))
	for _, l := range lines {
		fmt.Printf("  Line %d: %s\n", l.Number, l.Content)
	}

	// Output:
	// Line 3: Memory mapped line 3
	// Lines 5-7: 3 lines retrieved
	//   Line 5: Memory mapped line 5
	//   Line 6: Memory mapped line 6
	//   Line 7: Memory mapped line 7
}

// ExampleBufferManager_progressTracking demonstrates progress tracking during file loading
func ExampleBufferManager_progressTracking() {
	bm := NewBufferManager(DefaultBufferConfig)
	defer bm.Close()

	// Create a sample file
	tmpFile, _ := os.CreateTemp("", "progress_*.log")
	defer os.Remove(tmpFile.Name())

	// Write sample content
	for i := 1; i <= 1000; i++ {
		fmt.Fprintf(tmpFile, "Progress tracking line %d with content\n", i)
	}
	tmpFile.Close()

	// Progress tracking with detailed callbacks
	var progressUpdates []string
	progressCallback := func(loaded, total int64, phase string) {
		progressUpdates = append(progressUpdates,
			fmt.Sprintf("Phase: %s, Progress: %d/%d", phase, loaded, total))
	}

	// Load file with progress tracking
	ctx := context.Background()
	err := bm.OpenFile(ctx, tmpFile.Name(), progressCallback)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Show progress updates
	fmt.Printf("Total progress updates: %d\n", len(progressUpdates))
	if len(progressUpdates) > 0 {
		fmt.Printf("First update: %s\n", progressUpdates[0])
		fmt.Printf("Last update: %s\n", progressUpdates[len(progressUpdates)-1])
	}

	// Get final progress status
	progress := bm.GetProgress()
	fmt.Printf("Final status: Phase=%s, Lines=%d\n", progress.Phase, progress.LoadedLines)

	// Output:
	// Total progress updates: [varies]
	// First update: Phase: loading, Progress: [varies]
	// Last update: Phase: complete, Progress: 1000/[varies]
	// Final status: Phase=complete, Lines=1000
}

// ExampleBufferConfig demonstrates different configuration options
func ExampleBufferConfig() {
	// Configuration for small files with high responsiveness
	smallFileConfig := BufferConfig{
		CircularBufferLines:     5000,                   // Keep 5K recent lines
		MemoryMapThresholdBytes: 500 * 1024 * 1024,      // 500MB threshold
		ContextWindowSize:       25,                     // 25 lines context
		MaxMemoryUsageMB:        50,                     // 50MB limit
		ProgressUpdateInterval:  100 * time.Millisecond, // Frequent updates
		EnableAsync:             true,
	}

	// Configuration for large files with memory efficiency
	largeFileConfig := BufferConfig{
		CircularBufferLines:     20000,                  // Keep 20K recent lines
		MemoryMapThresholdBytes: 100 * 1024 * 1024,      // 100MB threshold
		ContextWindowSize:       100,                    // 100 lines context
		MaxMemoryUsageMB:        200,                    // 200MB limit
		ProgressUpdateInterval:  500 * time.Millisecond, // Less frequent updates
		EnableAsync:             true,
	}

	// Create buffer managers with different configs
	smallBM := NewBufferManager(smallFileConfig)
	defer smallBM.Close()

	largeBM := NewBufferManager(largeFileConfig)
	defer largeBM.Close()

	fmt.Printf("Small file config - Buffer size: %d, Memory map threshold: %d MB\n",
		smallFileConfig.CircularBufferLines,
		smallFileConfig.MemoryMapThresholdBytes/1024/1024)

	fmt.Printf("Large file config - Buffer size: %d, Memory map threshold: %d MB\n",
		largeFileConfig.CircularBufferLines,
		largeFileConfig.MemoryMapThresholdBytes/1024/1024)

	// Output:
	// Small file config - Buffer size: 5000, Memory map threshold: 500 MB
	// Large file config - Buffer size: 20000, Memory map threshold: 100 MB
}
