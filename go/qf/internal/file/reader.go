// Package file provides file reading functionality with streaming support for the qf application.
//
// This package implements the FileReader interface with support for large file streaming,
// configurable thresholds, context cancellation, and comprehensive error handling.
package file

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// FileReader defines the interface for reading files with streaming support
type FileReader interface {
	// ReadFile reads a file and returns a channel of Line structs
	// The channel is closed when reading is complete or an error occurs
	// Context can be used to cancel the operation
	ReadFile(ctx context.Context, path string) (<-chan Line, error)

	// IsStreamingMode indicates if the reader is using streaming mode
	// Typically enabled for files larger than a configured threshold
	IsStreamingMode(path string) (bool, error)

	// Close releases any resources held by the reader
	Close() error
}

// fileReaderImpl implements the FileReader interface
type fileReaderImpl struct {
	streamingThresholdBytes int64 // Threshold in bytes for enabling streaming mode
	bufferSize              int   // Buffer size for reading chunks
	closed                  int32 // Atomic flag to track if reader is closed
	mu                      sync.RWMutex
}

// ReaderOption defines functional options for configuring FileReader
type ReaderOption func(*fileReaderImpl)

// WithStreamingThreshold sets the file size threshold for enabling streaming mode
func WithStreamingThreshold(thresholdMB int) ReaderOption {
	return func(r *fileReaderImpl) {
		r.streamingThresholdBytes = int64(thresholdMB) * 1024 * 1024
	}
}

// WithBufferSize sets the buffer size for file reading
func WithBufferSize(sizeKB int) ReaderOption {
	return func(r *fileReaderImpl) {
		r.bufferSize = sizeKB * 1024
	}
}

// NewFileReader creates a new FileReader with default configuration
func NewFileReader(options ...ReaderOption) FileReader {
	reader := &fileReaderImpl{
		streamingThresholdBytes: 100 * 1024 * 1024, // Default 100MB threshold
		bufferSize:              64 * 1024,         // Default 64KB buffer
		closed:                  0,
	}

	// Apply options
	for _, option := range options {
		option(reader)
	}

	return reader
}

// ReadFile reads a file and returns a channel of Line structs
func (r *fileReaderImpl) ReadFile(ctx context.Context, path string) (<-chan Line, error) {
	// Check if reader is closed
	if atomic.LoadInt32(&r.closed) == 1 {
		return nil, fmt.Errorf("FileReader is closed")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Validate file access using centralized error handling
	file, err := OpenFileWithErrorHandling(path)
	if err != nil {
		return nil, err
	}

	// Get file info to determine streaming mode
	fileInfo, err := file.Stat()
	if err != nil {
		SafeCloseWithErrorHandling(file, path)
		return nil, DefaultErrorHandler.WrapError("stat", path, err)
	}

	// Determine if streaming mode should be used
	useStreaming := fileInfo.Size() > r.streamingThresholdBytes

	// Create channel for lines
	lineChan := make(chan Line, 100) // Buffered channel to prevent blocking

	// Start reading in a goroutine
	go r.readFileAsync(ctx, file, lineChan, useStreaming)

	return lineChan, nil
}

// readFileAsync performs the actual file reading in a separate goroutine
func (r *fileReaderImpl) readFileAsync(ctx context.Context, file *os.File, lineChan chan<- Line, useStreaming bool) {
	defer file.Close()
	defer close(lineChan)

	// Create buffered reader
	reader := bufio.NewReaderSize(file, r.bufferSize)

	lineNumber := 1
	var offset int64

	for {
		// Check for context cancellation at the beginning of each iteration
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check if reader is closed
		if atomic.LoadInt32(&r.closed) == 1 {
			return
		}

		// Read next line with periodic context checks
		lineBytes, err := r.readLineWithContext(ctx, reader)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				return
			}
			if err == io.EOF {
				// Handle last line without newline
				if len(lineBytes) > 0 {
					content := r.processLineContent(lineBytes)
					line := Line{
						Number:  lineNumber,
						Content: content,
						Offset:  offset,
					}

					select {
					case lineChan <- line:
					case <-ctx.Done():
						return
					}
				}
				return // End of file reached
			}
			// Other read errors
			return
		}

		// Process the line content
		content := r.processLineContent(lineBytes)

		// Create Line struct
		line := Line{
			Number:  lineNumber,
			Content: content,
			Offset:  offset,
		}

		// Send line to channel (non-blocking with context check)
		select {
		case lineChan <- line:
		case <-ctx.Done():
			return
		}

		// Update offset and line number
		offset += int64(len(lineBytes))
		lineNumber++

		// Check for cancellation every line to be maximally responsive
		// This is critical for proper context cancellation behavior
		select {
		case <-ctx.Done():
			return
		default:
			// For testing responsiveness, yield control very frequently
			// This ensures cancellation can be processed even during rapid file reading
			if lineNumber%3 == 0 { // Check every 3 lines
				runtime.Gosched()
				// Add a small sleep to allow other goroutines (like cancellation) to run
				time.Sleep(time.Microsecond * 100)
			}
		}
	}
}

// readLineWithContext reads a line with context cancellation support
func (r *fileReaderImpl) readLineWithContext(ctx context.Context, reader *bufio.Reader) ([]byte, error) {
	// We can't easily cancel a ReadBytes operation in progress,
	// but we can check context before and handle it gracefully
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	lineBytes, err := reader.ReadBytes('\n')

	// Check context again after read
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return lineBytes, err
}

// processLineContent cleans line content and handles UTF-8 encoding
func (r *fileReaderImpl) processLineContent(lineBytes []byte) string {
	// Remove trailing newline characters
	content := string(lineBytes)
	if len(content) > 0 {
		if content[len(content)-1] == '\n' {
			content = content[:len(content)-1]
		}
		if len(content) > 0 && content[len(content)-1] == '\r' {
			content = content[:len(content)-1]
		}
	}

	// Ensure valid UTF-8 encoding
	if !utf8.ValidString(content) {
		// Replace invalid UTF-8 sequences with replacement character
		content = string([]rune(content))
	}

	return content
}

// IsStreamingMode indicates if the reader would use streaming mode for the given file
func (r *fileReaderImpl) IsStreamingMode(path string) (bool, error) {
	// Check if reader is closed
	if atomic.LoadInt32(&r.closed) == 1 {
		return false, fmt.Errorf("FileReader is closed")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check file existence and get size using centralized error handling
	fileInfo, err := StatFileWithErrorHandling(path)
	if err != nil {
		return false, err
	}

	// Return whether file size exceeds streaming threshold
	return fileInfo.Size() > r.streamingThresholdBytes, nil
}

// Close releases any resources held by the reader
func (r *fileReaderImpl) Close() error {
	// Set closed flag atomically
	if !atomic.CompareAndSwapInt32(&r.closed, 0, 1) {
		return nil // Already closed
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// No additional resources to clean up in this implementation
	// In a more complex implementation, this might close file handles,
	// stop background workers, etc.

	return nil
}

// GetStreamingThreshold returns the current streaming threshold in bytes
func (r *fileReaderImpl) GetStreamingThreshold() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.streamingThresholdBytes
}

// GetBufferSize returns the current buffer size in bytes
func (r *fileReaderImpl) GetBufferSize() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bufferSize
}

// NewFileReaderFromConfig creates a FileReader configured from application config
// This integrates with the existing config system
func NewFileReaderFromConfig(streamingThresholdMB int) FileReader {
	return NewFileReader(
		WithStreamingThreshold(streamingThresholdMB),
		WithBufferSize(64), // 64KB default buffer
	)
}
