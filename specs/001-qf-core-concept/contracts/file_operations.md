# File Operations Contracts

**Date**: 2025-09-13
**Purpose**: Define interfaces for file reading, streaming, and monitoring operations

## File Reader Interface

```go
// FileReader handles file content loading with streaming support
type FileReader interface {
    // Open prepares file for reading, returns metadata
    Open(path string) (*FileInfo, error)

    // ReadLines reads lines from file with optional streaming
    ReadLines(ctx context.Context, startLine int, maxLines int) (<-chan Line, error)

    // ReadAll loads entire file (up to streaming threshold)
    ReadAll(ctx context.Context) ([]Line, error)

    // Close releases file resources
    Close() error

    // GetInfo returns current file metadata
    GetInfo() FileInfo
}

type FileInfo struct {
    Path         string
    Size         int64
    ModTime      time.Time
    LineCount    int64    // Estimated or actual
    Encoding     string
    IsReadable   bool
}
```

**Implementation Contract**:

- MUST detect file encoding automatically
- MUST handle files larger than memory via streaming
- MUST respect context cancellation
- MUST emit lines in order
- MUST close channels on completion or error

## File Streamer Interface

```go
// FileStreamer provides real-time file content updates
type FileStreamer interface {
    // StartStream begins streaming from specified position
    StartStream(ctx context.Context, path string, fromPos int64) (<-chan StreamEvent, error)

    // Follow enables tail-f style following of file updates
    Follow(ctx context.Context, path string) (<-chan StreamEvent, error)

    // Stop terminates streaming
    Stop() error

    // GetPosition returns current stream position
    GetPosition() int64
}

type StreamEvent struct {
    Type    StreamEventType
    Line    *Line
    Error   error
    EOF     bool
}

type StreamEventType int

const (
    StreamNewLine StreamEventType = iota
    StreamError
    StreamEOF
    StreamTruncated
)
```

**Implementation Contract**:

- MUST handle file rotation/truncation gracefully
- MUST provide backpressure when consumer is slow
- MUST respect system file handle limits
- MUST emit events in chronological order
- MUST handle permission changes during streaming

## File Watcher Interface

```go
// FileWatcher monitors file system changes
type FileWatcher interface {
    // Watch starts monitoring file for changes
    Watch(ctx context.Context, path string) (<-chan WatchEvent, error)

    // WatchMultiple monitors multiple files
    WatchMultiple(ctx context.Context, paths []string) (<-chan WatchEvent, error)

    // Stop terminates watching
    Stop() error
}

type WatchEvent struct {
    Path      string
    EventType WatchEventType
    Error     error
}

type WatchEventType int

const (
    WatchModified WatchEventType = iota
    WatchDeleted
    WatchCreated
    WatchMoved
    WatchPermissionChanged
)
```

**Implementation Contract**:

- MUST work across different operating systems
- MUST handle watched file deletion/recreation
- MUST debounce rapid file changes
- MUST provide file path in all events
- MUST handle permission denied gracefully

## File Buffer Interface

```go
// FileBuffer manages in-memory file content with size limits
type FileBuffer interface {
    // Append adds lines to buffer, evicting old lines if needed
    Append(lines []Line)

    // GetLines returns lines in specified range
    GetLines(startLine, endLine int) []Line

    // GetVisible returns only visible lines (pass current filter)
    GetVisible() []Line

    // GetContext returns lines with context around matches
    GetContext(lineNumber int, contextLines int) []Line

    // Clear removes all content
    Clear()

    // Stats returns buffer statistics
    Stats() BufferStats
}

type BufferStats struct {
    TotalLines    int
    VisibleLines  int
    MemoryUsage   int64
    OldestLine    int
    NewestLine    int
    BufferFull    bool
}
```

**Implementation Contract**:

- MUST maintain line number consistency
- MUST implement LRU eviction when buffer full
- MUST preserve context lines around visible content
- MUST be thread-safe for concurrent access
- MUST provide accurate memory usage reporting

## Export Interface

```go
// Exporter handles saving filtered content to various formats
type Exporter interface {
    // ExportText saves filtered lines as plain text
    ExportText(ctx context.Context, lines []Line, path string, options TextOptions) error

    // ExportRipgrep generates equivalent ripgrep command
    ExportRipgrep(filterSet FilterSet, filePaths []string) (string, error)

    // ExportJSON saves as structured JSON
    ExportJSON(ctx context.Context, session Session, path string) error

    // CopyToClipboard copies content to system clipboard
    CopyToClipboard(content string) error
}

type TextOptions struct {
    IncludeLineNumbers bool
    IncludeTimestamps  bool
    IncludeContext     int
    Format             TextFormat
}

type TextFormat int

const (
    FormatPlain TextFormat = iota
    FormatMarkdown
    FormatHTML
)
```

**Implementation Contract**:

- MUST handle write permission errors gracefully
- MUST generate syntactically correct ripgrep commands
- MUST preserve UTF-8 encoding
- MUST validate output paths before writing
- MUST support atomic writes to prevent corruption

## Configuration File Interface

```go
// ConfigManager handles configuration persistence and validation
type ConfigManager interface {
    // Load reads configuration from file
    Load(path string) (*Config, error)

    // Save writes configuration to file atomically
    Save(config *Config, path string) error

    // Validate checks configuration correctness
    Validate(config *Config) error

    // Migrate upgrades configuration schema
    Migrate(config *Config, fromVersion string) (*Config, error)

    // Watch monitors configuration file for changes
    Watch(ctx context.Context, path string) (<-chan *Config, error)
}
```

**Implementation Contract**:

- MUST use atomic writes to prevent corruption
- MUST validate all configuration values
- MUST provide meaningful error messages
- MUST handle missing configuration directories
- MUST preserve user comments in JSON files where possible

## Session Persistence Interface

```go
// SessionManager handles session save/load operations
type SessionManager interface {
    // Save persists session with backup rotation
    Save(session *Session, name string) error

    // Load retrieves session by name
    Load(name string) (*Session, error)

    // List returns all available session names
    List() ([]string, error)

    // Delete removes session and its backups
    Delete(name string) error

    // Cleanup removes expired sessions based on retention policy
    Cleanup(config DataConfig) error

    // AutoSave enables automatic session persistence
    AutoSave(ctx context.Context, session *Session, interval time.Duration) error
}
```

**Implementation Contract**:

- MUST maintain backup rotation (3 most recent)
- MUST detect and recover from corrupted sessions
- MUST handle concurrent access to session files
- MUST respect retention policies
- MUST provide atomic saves to prevent partial corruption

## Testing Contracts

### File Reader Tests

```go
func TestFileReaderContract(t *testing.T) {
    testCases := []struct {
        name        string
        fileSize    int64
        encoding    string
        expectError bool
    }{
        {"Small UTF-8 file", 1024, "utf-8", false},
        {"Large UTF-8 file", 100*1024*1024, "utf-8", false},
        {"Binary file", 1024, "binary", true},
        {"Non-existent file", 0, "", true},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Streaming Tests

```go
func TestStreamingContract(t *testing.T) {
    // Test stream cancellation
    // Test backpressure handling
    // Test file rotation during streaming
    // Test permission changes during streaming
}
```

### Buffer Management Tests

```go
func TestBufferContract(t *testing.T) {
    // Test buffer size limits
    // Test LRU eviction
    // Test concurrent access
    // Test memory usage tracking
}
```

## Error Handling Standards

All file operations MUST handle these error conditions:

1. **File Not Found**: Return clear error message with file path
2. **Permission Denied**: Provide actionable error message
3. **Disk Full**: Detect and report during write operations
4. **File Locked**: Handle locked files gracefully
5. **Network Drives**: Handle network timeouts and disconnections
6. **File Too Large**: Automatically switch to streaming mode
7. **Invalid Encoding**: Detect and report encoding issues
8. **Corrupted Data**: Validate file format and report corruption

## Performance Requirements

All file operations MUST meet these performance targets:

- **Small files (<10MB)**: Load within 1 second
- **Large files (>100MB)**: Begin streaming within 2 seconds
- **File watching**: Detect changes within 1 second
- **Memory usage**: Respect configured buffer limits
- **Concurrent operations**: Support 5 concurrent file operations
- **Stream throughput**: Process >100K lines/second
- **Export operations**: Complete within 2x read time

---
*File Operations Contracts complete: 2025-09-13*
