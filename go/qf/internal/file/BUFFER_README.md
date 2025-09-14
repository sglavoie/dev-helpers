# Buffer Management System for qf

The buffer management system provides efficient file streaming and content access for the qf Interactive Log Filter Composer. It implements circular buffers, memory-mapped files, and context-aware loading to handle large files while maintaining performance requirements.

## Architecture Overview

The buffer system consists of three main components:

1. **CircularBuffer**: Thread-safe ring buffer for recent lines
2. **FileBuffer**: Memory-mapped access for very large files (>1GB)
3. **BufferManager**: Coordinates between buffers and provides unified API

## Key Features

### Memory Efficiency
- **Circular Buffer**: Keeps only the most recent N lines (default: 10,000)
- **Memory Mapping**: Direct OS-level mapping for files >1GB
- **Context Caching**: Intelligent caching of frequently accessed context regions
- **Memory Limits**: Configurable memory usage limits with automatic cleanup

### Performance Optimization
- **Sub-millisecond line access**: ~0.5ms for 1,000 line retrievals
- **Fast file opening**: <50ms for typical files
- **Streaming mode**: Immediate display while loading in background
- **Progress tracking**: Real-time loading progress with callbacks

### Large File Support
- **Automatic threshold detection**: Files >1GB use memory mapping
- **Streaming mode**: Handle files of any size without memory overflow
- **Context loading**: Efficient retrieval of lines around matches
- **Background processing**: Non-blocking file operations

## Usage Examples

### Basic Circular Buffer

```go
// Create buffer with 5,000 line capacity
cb := NewCircularBuffer(5000)

// Add lines (automatically evicts oldest when full)
for i := 1; i <= 10000; i++ {
    line := LineBuffer{
        Number:  int32(i),
        Content: fmt.Sprintf("Line %d content", i),
        Offset:  int64(i * 50),
    }
    cb.AddLine(line)
}

// Retrieve specific line
line, found := cb.GetLine(8000)
if found {
    fmt.Printf("Line %d: %s\n", line.Number, line.Content)
}

// Get buffer statistics
size, capacity, totalIn, totalOut := cb.GetStats()
```

### Memory-Mapped File Buffer

```go
// Open file with memory mapping
fb, err := NewFileBuffer("/path/to/large/file.log")
if err != nil {
    return err
}
defer fb.Release()

// Access lines by number (1-based)
line, err := fb.GetLine(1000000) // Works even for very large files
if err != nil {
    return err
}

// Get range of lines
lines, err := fb.GetRange(1000000, 1000100) // 100 lines
if err != nil {
    return err
}
```

### Complete Buffer Manager

```go
// Configure for your use case
config := DefaultBufferConfig
config.CircularBufferLines = 15000        // Keep 15K recent lines
config.MemoryMapThresholdBytes = 500 << 20 // 500MB threshold
config.ContextWindowSize = 50              // 50 lines context
config.MaxMemoryUsageMB = 150             // 150MB memory limit

// Create buffer manager
bm := NewBufferManager(config)
defer bm.Close()

// Progress tracking callback
progressCallback := func(loaded, total int64, phase string) {
    percentage := float64(loaded) / float64(total) * 100
    fmt.Printf("Loading: %.1f%% (%s)\n", percentage, phase)
}

// Open and load file
ctx := context.Background()
err := bm.OpenFile(ctx, "/path/to/logfile.log", progressCallback)
if err != nil {
    return err
}

// Access lines efficiently
line, err := bm.GetLine(500000)
if err != nil {
    return err
}

// Load context around matches
contextLines, err := bm.LoadContext(ctx, 500000, 20) // 20 lines around line 500000
if err != nil {
    return err
}

// Monitor memory usage
memUsage := bm.GetMemoryUsage()
fmt.Printf("Memory usage: %d MB\n", memUsage/1024/1024)
```

## Configuration Options

### BufferConfig Structure

```go
type BufferConfig struct {
    // CircularBufferLines: Maximum lines in circular buffer (default: 10,000)
    CircularBufferLines int

    // MemoryMapThresholdBytes: File size threshold for memory mapping (default: 1GB)
    MemoryMapThresholdBytes int64

    // ContextWindowSize: Lines to load around matches (default: 50)
    ContextWindowSize int

    // MaxMemoryUsageMB: Memory usage limit in MB (default: 100MB)
    MaxMemoryUsageMB int

    // ProgressUpdateInterval: How often to call progress callbacks (default: 250ms)
    ProgressUpdateInterval time.Duration

    // EnableAsync: Enable background loading (default: true)
    EnableAsync bool
}
```

### Recommended Configurations

#### Small Files (Interactive Response)
```go
config := BufferConfig{
    CircularBufferLines:     5000,
    MemoryMapThresholdBytes: 500 * 1024 * 1024, // 500MB
    ContextWindowSize:       25,
    MaxMemoryUsageMB:        50,
    ProgressUpdateInterval:  100 * time.Millisecond,
    EnableAsync:             true,
}
```

#### Large Files (Memory Efficient)
```go
config := BufferConfig{
    CircularBufferLines:     20000,
    MemoryMapThresholdBytes: 100 * 1024 * 1024, // 100MB
    ContextWindowSize:       100,
    MaxMemoryUsageMB:        200,
    ProgressUpdateInterval:  500 * time.Millisecond,
    EnableAsync:             true,
}
```

## Performance Characteristics

### Benchmarks (Typical Hardware)

| Operation | Performance | Notes |
|-----------|-------------|-------|
| Circular Buffer Add | ~140ns/line | 10K lines in ~1.4ms |
| Circular Buffer Get | ~520ns/line | 1K retrievals in ~0.5ms |
| File Buffer Open | <50ms | Including memory mapping setup |
| Line Access | <10μs/line | Memory-mapped or circular buffer |
| Context Loading | <5ms | 100 lines with caching |
| Memory Usage | <100MB | For files up to several GB |

### Memory Usage Patterns

- **Small files (<100MB)**: Loaded entirely into circular buffer
- **Medium files (100MB-1GB)**: Initial chunk in circular buffer, rest streamed
- **Large files (>1GB)**: Memory-mapped with circular buffer for recent lines
- **Context regions**: Cached for frequently accessed areas

## Integration with qf Components

### With FileReader
```go
// The buffer manager integrates seamlessly with the existing FileReader
reader := NewFileReader(WithStreamingThreshold(100))
bm := NewBufferManager(DefaultBufferConfig)

// BufferManager can use FileReader for consistency
bm.OpenFile(ctx, path, progressCallback)
```

### With FilterEngine
```go
// Efficient filtering with buffer integration
line, err := bm.GetLine(lineNumber)
if err == nil && matchesFilter(line.Content) {
    // Load context for this match
    context, _ := bm.LoadContext(ctx, int(line.Number), 50)
    displayMatch(line, context)
}
```

### Progress Integration
```go
// Real-time progress updates for UI
progressCallback := func(loaded, total int64, phase string) {
    // Update UI progress bar
    updateProgressBar(float64(loaded)/float64(total), phase)

    // Update status message
    updateStatus(fmt.Sprintf("Loading: %s (%d lines)", phase, loaded))
}
```

## Error Handling

### Common Error Scenarios

1. **File Access Errors**
   ```go
   err := bm.OpenFile(ctx, path, callback)
   if os.IsNotExist(err) {
       // File not found
   } else if os.IsPermission(err) {
       // Permission denied
   }
   ```

2. **Memory Mapping Failures**
   ```go
   fb, err := NewFileBuffer(path)
   if err != nil {
       // Falls back to regular file reading
       // Handle gracefully
   }
   ```

3. **Context Cancellation**
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()

   err := bm.OpenFile(ctx, path, callback)
   if err == context.DeadlineExceeded {
       // Loading took too long
   }
   ```

## Memory Management

### Automatic Cleanup
- Reference counting for file buffers
- Automatic garbage collection when memory limits exceeded
- Context cache pruning for memory efficiency
- Proper cleanup on Close()

### Memory Monitoring
```go
// Get current memory usage
memUsage := bm.GetMemoryUsage()

// Get detailed statistics
stats := bm.GetStats()
fmt.Printf("Circular buffer size: %v\n", stats["circular_buffer_size"])
fmt.Printf("Memory usage: %v MB\n", stats["memory_usage_mb"])
fmt.Printf("Context cache entries: %v\n", stats["context_cache_entries"])
```

## Thread Safety

All buffer components are thread-safe:

- **CircularBuffer**: Uses RWMutex for concurrent access
- **FileBuffer**: Thread-safe memory mapping with reference counting
- **BufferManager**: Coordinates thread-safe access across components

## Testing

Run the comprehensive test suite:

```bash
# All buffer tests
go test ./internal/file -v -run "TestCircularBuffer|TestFileBuffer|TestBufferManager"

# Performance benchmarks
go test ./internal/file -v -run "TestPerformanceRequirements"

# Examples and usage
go test ./internal/file -v -run "Example"
```

## Troubleshooting

### High Memory Usage
- Check `MaxMemoryUsageMB` configuration
- Reduce `CircularBufferLines` for memory-constrained environments
- Increase `MemoryMapThresholdBytes` to use memory mapping earlier

### Slow Performance
- Ensure `EnableAsync` is true for background loading
- Reduce `ProgressUpdateInterval` for less frequent callbacks
- Check file system performance for memory-mapped files

### Context Loading Issues
- Verify `ContextWindowSize` is appropriate for your use case
- Check that line numbers are valid (1-based indexing)
- Monitor context cache size in statistics

---

*This buffer system is designed to meet the performance requirements outlined in the qf large file handling specifications while maintaining memory efficiency and responsive user interaction.*