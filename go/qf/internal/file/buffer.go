// Package file provides buffer management for efficient file streaming and content access.
//
// This package implements circular buffers, memory-mapped files, and context-aware loading
// for the qf Interactive Log Filter Composer. It provides memory-efficient access to
// large files while maintaining performance requirements for interactive usage.
package file

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// BufferConfig defines configuration parameters for buffer management
type BufferConfig struct {
	// CircularBufferLines is the maximum number of lines to keep in the circular buffer
	CircularBufferLines int

	// MemoryMapThresholdBytes is the file size threshold for using memory mapping
	MemoryMapThresholdBytes int64

	// ContextWindowSize is the number of lines to load around matches for context
	ContextWindowSize int

	// MaxMemoryUsageMB is the maximum memory usage limit in megabytes
	MaxMemoryUsageMB int

	// ProgressUpdateInterval controls how often progress callbacks are called
	ProgressUpdateInterval time.Duration

	// EnableAsync enables asynchronous background loading
	EnableAsync bool
}

// DefaultBufferConfig provides sensible defaults for buffer management
var DefaultBufferConfig = BufferConfig{
	CircularBufferLines:     10000,              // 10K lines circular buffer
	MemoryMapThresholdBytes: 1024 * 1024 * 1024, // 1GB threshold for memory mapping
	ContextWindowSize:       50,                 // 50 lines context window
	MaxMemoryUsageMB:        100,                // 100MB memory limit
	ProgressUpdateInterval:  250 * time.Millisecond,
	EnableAsync:             true,
}

// ProgressCallback is called during loading operations to report progress
type ProgressCallback func(loaded, total int64, phase string)

// LineBuffer represents an individual line with metadata optimized for memory efficiency
type LineBuffer struct {
	Number    int32  // 1-based line number (4 bytes)
	Offset    int64  // Byte offset in file (8 bytes)
	Length    uint16 // Content length in bytes (2 bytes, max 65KB per line)
	Content   string // Line content without newline
	Timestamp int64  // Load timestamp for LRU eviction
}

// CircularBuffer implements a thread-safe circular buffer for recent lines
type CircularBuffer struct {
	lines    []LineBuffer // Fixed-size ring buffer
	capacity int32        // Maximum number of lines
	head     int32        // Next write position
	tail     int32        // Oldest line position
	size     int32        // Current number of lines
	mu       sync.RWMutex // Reader-writer lock for thread safety
	totalIn  int64        // Total lines added (for statistics)
	totalOut int64        // Total lines evicted
}

// NewCircularBuffer creates a new circular buffer with the specified capacity
func NewCircularBuffer(capacity int) *CircularBuffer {
	if capacity <= 0 {
		capacity = DefaultBufferConfig.CircularBufferLines
	}

	return &CircularBuffer{
		lines:    make([]LineBuffer, capacity),
		capacity: int32(capacity),
		head:     0,
		tail:     0,
		size:     0,
	}
}

// AddLine adds a line to the circular buffer, evicting the oldest if full
func (cb *CircularBuffer) AddLine(line LineBuffer) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	line.Timestamp = time.Now().UnixNano()

	// If buffer is full, advance tail (evict oldest)
	if cb.size == cb.capacity {
		cb.tail = (cb.tail + 1) % cb.capacity
		atomic.AddInt64(&cb.totalOut, 1)
	} else {
		cb.size++
	}

	// Add new line at head
	cb.lines[cb.head] = line
	cb.head = (cb.head + 1) % cb.capacity
	atomic.AddInt64(&cb.totalIn, 1)
}

// GetLine retrieves a line by its absolute line number (1-based)
func (cb *CircularBuffer) GetLine(lineNumber int) (LineBuffer, bool) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.size == 0 {
		return LineBuffer{}, false
	}

	// Search through the circular buffer
	for i := int32(0); i < cb.size; i++ {
		idx := (cb.tail + i) % cb.capacity
		if cb.lines[idx].Number == int32(lineNumber) {
			return cb.lines[idx], true
		}
	}

	return LineBuffer{}, false
}

// GetRange retrieves a range of lines from the buffer
func (cb *CircularBuffer) GetRange(startLine, endLine int) []LineBuffer {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.size == 0 || startLine > endLine {
		return nil
	}

	var result []LineBuffer
	for i := int32(0); i < cb.size; i++ {
		idx := (cb.tail + i) % cb.capacity
		lineNum := int(cb.lines[idx].Number)
		if lineNum >= startLine && lineNum <= endLine {
			result = append(result, cb.lines[idx])
		}
	}

	return result
}

// GetStats returns buffer statistics
func (cb *CircularBuffer) GetStats() (size, capacity int, totalIn, totalOut int64) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return int(cb.size), int(cb.capacity), cb.totalIn, cb.totalOut
}

// Clear removes all lines from the buffer
func (cb *CircularBuffer) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.head = 0
	cb.tail = 0
	cb.size = 0
	// Don't reset totalIn/totalOut for statistics tracking
}

// FileBuffer manages memory-mapped access to large files
type FileBuffer struct {
	path     string          // File path
	file     *os.File        // File handle
	data     []byte          // Memory-mapped data
	size     int64           // File size in bytes
	lineMap  map[int32]int64 // Line number to byte offset mapping
	mu       sync.RWMutex    // Thread-safe access
	refCount int32           // Reference count for safe cleanup
	closed   int32           // Atomic flag for closed state
}

// NewFileBuffer creates a new memory-mapped file buffer
func NewFileBuffer(path string) (*FileBuffer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	size := stat.Size()
	if size == 0 {
		file.Close()
		return &FileBuffer{
			path:     path,
			size:     0,
			lineMap:  make(map[int32]int64),
			refCount: 1,
		}, nil
	}

	// Memory map the file
	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to mmap file %s: %w", path, err)
	}

	fb := &FileBuffer{
		path:     path,
		file:     file,
		data:     data,
		size:     size,
		lineMap:  make(map[int32]int64),
		refCount: 1,
	}

	// Build line index in background
	go fb.buildLineIndex()

	return fb, nil
}

// buildLineIndex builds a map from line numbers to byte offsets
func (fb *FileBuffer) buildLineIndex() {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if atomic.LoadInt32(&fb.closed) != 0 || len(fb.data) == 0 {
		return
	}

	lineNum := int32(1)
	offset := int64(0)
	fb.lineMap[lineNum] = offset

	// Scan for newlines to build line index
	for i, b := range fb.data {
		if b == '\n' {
			lineNum++
			offset = int64(i + 1)
			if offset < fb.size {
				fb.lineMap[lineNum] = offset
			}
		}
	}
}

// GetLine retrieves a line by number from the memory-mapped file
func (fb *FileBuffer) GetLine(lineNumber int) (LineBuffer, error) {
	if atomic.LoadInt32(&fb.closed) != 0 {
		return LineBuffer{}, fmt.Errorf("file buffer is closed")
	}

	fb.mu.RLock()
	defer fb.mu.RUnlock()

	offset, exists := fb.lineMap[int32(lineNumber)]
	if !exists {
		return LineBuffer{}, fmt.Errorf("line %d not found", lineNumber)
	}

	// Find end of line
	start := offset
	end := fb.size
	for i := start; i < fb.size; i++ {
		if fb.data[i] == '\n' {
			end = i
			break
		}
	}

	// Extract line content
	content := string(fb.data[start:end])
	if len(content) > 0 && content[len(content)-1] == '\r' {
		content = content[:len(content)-1] // Remove carriage return
	}

	return LineBuffer{
		Number:    int32(lineNumber),
		Offset:    start,
		Length:    uint16(len(content)),
		Content:   content,
		Timestamp: time.Now().UnixNano(),
	}, nil
}

// GetRange retrieves a range of lines from the memory-mapped file
func (fb *FileBuffer) GetRange(startLine, endLine int) ([]LineBuffer, error) {
	if atomic.LoadInt32(&fb.closed) != 0 {
		return nil, fmt.Errorf("file buffer is closed")
	}

	if startLine > endLine || startLine < 1 {
		return nil, fmt.Errorf("invalid line range: %d-%d", startLine, endLine)
	}

	var result []LineBuffer
	for lineNum := startLine; lineNum <= endLine; lineNum++ {
		line, err := fb.GetLine(lineNum)
		if err != nil {
			break // End of file or invalid line
		}
		result = append(result, line)
	}

	return result, nil
}

// AddRef increments the reference count
func (fb *FileBuffer) AddRef() {
	atomic.AddInt32(&fb.refCount, 1)
}

// Release decrements reference count and cleans up if zero
func (fb *FileBuffer) Release() error {
	if atomic.AddInt32(&fb.refCount, -1) == 0 {
		return fb.cleanup()
	}
	return nil
}

// cleanup releases file buffer resources
func (fb *FileBuffer) cleanup() error {
	if !atomic.CompareAndSwapInt32(&fb.closed, 0, 1) {
		return nil // Already closed
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	var errs []error

	// Unmap memory
	if len(fb.data) > 0 {
		if err := syscall.Munmap(fb.data); err != nil {
			errs = append(errs, fmt.Errorf("failed to munmap: %w", err))
		}
		fb.data = nil
	}

	// Close file
	if fb.file != nil {
		if err := fb.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close file: %w", err))
		}
		fb.file = nil
	}

	// Clear line map
	fb.lineMap = nil

	// Return first error if any
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// BufferManager coordinates between circular buffers, file buffers, and provides unified access
type BufferManager struct {
	config       BufferConfig
	circularBuf  *CircularBuffer
	fileBuf      *FileBuffer
	contextCache map[int32][]LineBuffer // Cache for context lines around matches
	mu           sync.RWMutex
	memoryUsage  int64 // Current memory usage estimate
	loadProgress atomic.Value
	closed       int32
}

// LoadProgress tracks loading operation progress
type LoadProgress struct {
	Phase       string // Current phase: "indexing", "loading", "complete"
	LoadedBytes int64  // Bytes processed so far
	TotalBytes  int64  // Total bytes to process
	LoadedLines int64  // Lines processed so far
	StartTime   time.Time
	LastUpdate  time.Time
}

// NewBufferManager creates a new buffer manager with the specified configuration
func NewBufferManager(config BufferConfig) *BufferManager {
	if config.CircularBufferLines <= 0 {
		config = DefaultBufferConfig
	}

	bm := &BufferManager{
		config:       config,
		circularBuf:  NewCircularBuffer(config.CircularBufferLines),
		contextCache: make(map[int32][]LineBuffer),
	}

	// Initialize progress
	bm.loadProgress.Store(&LoadProgress{
		Phase:     "ready",
		StartTime: time.Now(),
	})

	return bm
}

// OpenFile initializes the buffer manager for the specified file
func (bm *BufferManager) OpenFile(ctx context.Context, path string, progressCallback ProgressCallback) error {
	if atomic.LoadInt32(&bm.closed) != 0 {
		return fmt.Errorf("buffer manager is closed")
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Clear existing state
	bm.circularBuf.Clear()
	bm.contextCache = make(map[int32][]LineBuffer)

	// Get file info
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	fileSize := stat.Size()

	// Initialize progress tracking
	progress := &LoadProgress{
		Phase:      "opening",
		TotalBytes: fileSize,
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
	}
	bm.loadProgress.Store(progress)

	// Determine buffer strategy based on file size
	if fileSize > bm.config.MemoryMapThresholdBytes {
		// Use memory mapping for very large files
		progress.Phase = "indexing"
		bm.loadProgress.Store(progress)

		fileBuf, err := NewFileBuffer(path)
		if err != nil {
			return fmt.Errorf("failed to create file buffer: %w", err)
		}

		// Release old file buffer if exists
		if bm.fileBuf != nil {
			bm.fileBuf.Release()
		}

		bm.fileBuf = fileBuf

		// Load initial lines into circular buffer for quick access
		if bm.config.EnableAsync {
			go bm.loadInitialLines(ctx, progressCallback)
		} else {
			return bm.loadInitialLines(ctx, progressCallback)
		}
	} else {
		// Use regular file reading with circular buffer
		return bm.loadWithCircularBuffer(ctx, path, progressCallback)
	}

	return nil
}

// loadInitialLines loads the first set of lines into the circular buffer
func (bm *BufferManager) loadInitialLines(ctx context.Context, progressCallback ProgressCallback) error {
	if bm.fileBuf == nil {
		return fmt.Errorf("no file buffer available")
	}

	progress := bm.loadProgress.Load().(*LoadProgress)
	progress.Phase = "loading"
	bm.loadProgress.Store(progress)

	// Load first N lines into circular buffer for quick access
	loadCount := bm.config.CircularBufferLines
	loaded := 0

	ticker := time.NewTicker(bm.config.ProgressUpdateInterval)
	defer ticker.Stop()

	for lineNum := 1; lineNum <= loadCount && atomic.LoadInt32(&bm.closed) == 0; lineNum++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Update progress
			progress.LoadedLines = int64(loaded)
			progress.LastUpdate = time.Now()
			bm.loadProgress.Store(progress)

			if progressCallback != nil {
				progressCallback(progress.LoadedLines, int64(loadCount), progress.Phase)
			}
		default:
		}

		line, err := bm.fileBuf.GetLine(lineNum)
		if err != nil {
			break // End of file or error
		}

		bm.circularBuf.AddLine(line)
		loaded++

		// Update memory usage estimate
		atomic.AddInt64(&bm.memoryUsage, int64(len(line.Content)+32)) // 32 bytes overhead per line
	}

	// Mark as complete
	progress.Phase = "complete"
	progress.LoadedLines = int64(loaded)
	progress.LastUpdate = time.Now()
	bm.loadProgress.Store(progress)

	if progressCallback != nil {
		progressCallback(progress.LoadedLines, int64(loadCount), progress.Phase)
	}

	return nil
}

// loadWithCircularBuffer loads a file using only the circular buffer (for smaller files)
func (bm *BufferManager) loadWithCircularBuffer(ctx context.Context, path string, progressCallback ProgressCallback) error {
	// Use existing FileReader for consistency
	reader := NewFileReader(
		WithStreamingThreshold(int(bm.config.MemoryMapThresholdBytes/1024/1024)),
		WithBufferSize(64), // 64KB buffer
	)
	defer reader.Close()

	lineChan, err := reader.ReadFile(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	progress := bm.loadProgress.Load().(*LoadProgress)
	progress.Phase = "loading"
	bm.loadProgress.Store(progress)

	ticker := time.NewTicker(bm.config.ProgressUpdateInterval)
	defer ticker.Stop()

	loaded := int64(0)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			// Update progress
			progress.LoadedLines = loaded
			progress.LastUpdate = time.Now()
			bm.loadProgress.Store(progress)

			if progressCallback != nil {
				progressCallback(progress.LoadedLines, progress.TotalBytes, progress.Phase)
			}

		case line, ok := <-lineChan:
			if !ok {
				// Channel closed, loading complete
				progress.Phase = "complete"
				progress.LoadedLines = loaded
				progress.LastUpdate = time.Now()
				bm.loadProgress.Store(progress)

				if progressCallback != nil {
					progressCallback(progress.LoadedLines, progress.TotalBytes, progress.Phase)
				}
				return nil
			}

			// Convert to LineBuffer format
			lineBuffer := LineBuffer{
				Number:    int32(line.Number),
				Offset:    line.Offset,
				Length:    uint16(len(line.Content)),
				Content:   line.Content,
				Timestamp: time.Now().UnixNano(),
			}

			bm.circularBuf.AddLine(lineBuffer)
			loaded++

			// Update memory usage estimate
			atomic.AddInt64(&bm.memoryUsage, int64(len(line.Content)+32))

			// Check memory limits
			if bm.getMemoryUsageMB() > int64(bm.config.MaxMemoryUsageMB) {
				// Could implement memory pressure handling here
				runtime.GC() // Force garbage collection
			}
		}
	}
}

// GetLine retrieves a line by number using the most efficient method available
func (bm *BufferManager) GetLine(lineNumber int) (LineBuffer, error) {
	if atomic.LoadInt32(&bm.closed) != 0 {
		return LineBuffer{}, fmt.Errorf("buffer manager is closed")
	}

	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// Try circular buffer first (for recent lines)
	if line, found := bm.circularBuf.GetLine(lineNumber); found {
		return line, nil
	}

	// Try file buffer if available
	if bm.fileBuf != nil {
		return bm.fileBuf.GetLine(lineNumber)
	}

	return LineBuffer{}, fmt.Errorf("line %d not available", lineNumber)
}

// LoadContext loads lines around a match for context display
func (bm *BufferManager) LoadContext(ctx context.Context, centerLine, windowSize int) ([]LineBuffer, error) {
	if atomic.LoadInt32(&bm.closed) != 0 {
		return nil, fmt.Errorf("buffer manager is closed")
	}

	if windowSize <= 0 {
		windowSize = bm.config.ContextWindowSize
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Check cache first
	cacheKey := int32(centerLine*1000 + windowSize) // Simple cache key
	if cached, exists := bm.contextCache[cacheKey]; exists {
		return cached, nil
	}

	// Calculate range
	startLine := centerLine - windowSize/2
	if startLine < 1 {
		startLine = 1
	}
	endLine := centerLine + windowSize/2

	var result []LineBuffer

	// Load context lines
	for lineNum := startLine; lineNum <= endLine; lineNum++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line, err := bm.GetLine(lineNum)
		if err != nil {
			// Line not available, continue with next
			continue
		}

		result = append(result, line)
	}

	// Cache the result
	if len(bm.contextCache) < 100 { // Limit cache size
		bm.contextCache[cacheKey] = result
	}

	return result, nil
}

// GetProgress returns current loading progress
func (bm *BufferManager) GetProgress() LoadProgress {
	if progress := bm.loadProgress.Load(); progress != nil {
		return *progress.(*LoadProgress)
	}
	return LoadProgress{Phase: "unknown"}
}

// GetMemoryUsage returns current memory usage estimate in bytes
func (bm *BufferManager) GetMemoryUsage() int64 {
	return atomic.LoadInt64(&bm.memoryUsage)
}

// getMemoryUsageMB returns memory usage in megabytes
func (bm *BufferManager) getMemoryUsageMB() int64 {
	return bm.GetMemoryUsage() / 1024 / 1024
}

// GetStats returns buffer statistics
func (bm *BufferManager) GetStats() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	size, capacity, totalIn, totalOut := bm.circularBuf.GetStats()

	stats := map[string]interface{}{
		"circular_buffer_size":     size,
		"circular_buffer_capacity": capacity,
		"total_lines_added":        totalIn,
		"total_lines_evicted":      totalOut,
		"memory_usage_mb":          bm.getMemoryUsageMB(),
		"context_cache_entries":    len(bm.contextCache),
		"has_file_buffer":          bm.fileBuf != nil,
	}

	if progress := bm.loadProgress.Load(); progress != nil {
		p := progress.(*LoadProgress)
		stats["load_progress_phase"] = p.Phase
		stats["load_progress_lines"] = p.LoadedLines
		stats["load_progress_bytes"] = p.LoadedBytes
	}

	return stats
}

// Close releases all resources and shuts down the buffer manager
func (bm *BufferManager) Close() error {
	if !atomic.CompareAndSwapInt32(&bm.closed, 0, 1) {
		return nil // Already closed
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	var errs []error

	// Release file buffer
	if bm.fileBuf != nil {
		if err := bm.fileBuf.Release(); err != nil {
			errs = append(errs, err)
		}
		bm.fileBuf = nil
	}

	// Clear circular buffer
	bm.circularBuf.Clear()

	// Clear context cache
	bm.contextCache = nil

	// Return first error if any
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// IsStreaming returns whether the buffer manager is using streaming mode
func (bm *BufferManager) IsStreaming() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.fileBuf != nil
}

// NewBufferManagerFromConfig creates a BufferManager from configuration
func NewBufferManagerFromConfig(circularBufferLines int, memoryMapThresholdGB float64, contextWindowSize int) *BufferManager {
	config := DefaultBufferConfig

	if circularBufferLines > 0 {
		config.CircularBufferLines = circularBufferLines
	}

	if memoryMapThresholdGB > 0 {
		config.MemoryMapThresholdBytes = int64(memoryMapThresholdGB * 1024 * 1024 * 1024)
	}

	if contextWindowSize > 0 {
		config.ContextWindowSize = contextWindowSize
	}

	return NewBufferManager(config)
}
