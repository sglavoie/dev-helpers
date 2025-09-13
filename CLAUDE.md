# Claude Code Assistant Context - qf (Interactive Log Filter Composer)

**Project**: Interactive Log Filter Composer (qf)
**Language**: Go 1.25+
**Architecture**: Terminal UI application using Bubble Tea framework
**Last Updated**: 2025-09-13

## Project Overview

qf is a terminal-based interactive log filter composer that transforms log filtering from command-line complexity into a visual, iterative experience. It uses Vim-style modal interface with real-time filtering and session management.

## Core Technologies

### Primary Framework

- **Bubble Tea v1.3.9**: TUI framework for component-based interface
- **Lipgloss v1.1.0**: Styling and layout system
- **Bubbles v0.21.0**: Additional UI components (viewport, input, etc.)

### Supporting Libraries

- **Go standard library**: Primary dependency for core functionality and regex features
- **clipboard v0.1.4**: Cross-platform clipboard support

### Architecture Principles

- **Component-based**: Each UI pane is an independent tea.Model
- **Message passing**: Components communicate via Bubble Tea messages
- **Modal interface**: Strict Normal/Insert mode separation (Vim-style)
- **Streaming support**: Handle large files without memory issues

## Project Structure

```
qf/
├── cmd/qf/                  # Main application entry point
├── internal/
│   ├── ui/                  # TUI components (Bubble Tea models)
│   │   ├── app.go           # Main application model
│   │   ├── filter_pane.go   # Include/Exclude pattern panes
│   │   ├── viewer.go        # Content display component
│   │   ├── tabs.go          # File tab management
│   │   ├── overlay.go       # Pattern testing overlay
│   │   └── statusbar.go     # Status and help display
│   ├── core/                # Business logic
│   │   ├── filter.go        # Filtering engine
│   │   ├── pattern.go       # Pattern compilation and caching
│   │   ├── matcher.go       # Regex matching logic
│   │   └── highlighter.go   # Match highlighting
│   ├── config/              # Configuration management
│   │   ├── config.go        # Config loading/saving
│   │   ├── schema.go        # Config structure definitions
│   │   └── migration.go     # Config version migration
│   ├── session/             # Session persistence
│   │   ├── session.go       # Session management
│   │   ├── history.go       # Pattern history tracking
│   │   └── persistence.go   # File I/O for sessions
│   ├── file/                # File operations
│   │   ├── reader.go        # File reading with streaming
│   │   ├── buffer.go        # Circular buffer for large files
│   │   └── watcher.go       # File change detection
│   └── export/              # Export functionality
│       ├── exporter.go      # Export interface
│       ├── text.go          # Plain text export
│       └── ripgrep.go       # Generate ripgrep commands
├── tests/                   # Test suites
│   ├── contract/            # Interface contract tests
│   ├── integration/         # Component integration tests
│   └── unit/                # Unit tests
└── specs/                   # Design documentation
    └── 001-qf-core-concept/  # Current feature specification
```

## Key Data Structures

### Core Entities

```go
type Pattern struct {
    ID          string           // UUID for identification
    Expression  string           // Raw regex pattern
    Type        PatternType      // Include or Exclude
    MatchCount  int              // Usage statistics
    Color       string           // Highlighting color
    Created     time.Time        // Metadata
    IsValid     bool             // Compilation status
}

type FilterSet struct {
    Include     []Pattern        // OR logic patterns
    Exclude     []Pattern        // Veto logic patterns
    Name        string           // Session identifier
}

type Session struct {
    Name        string           // Session name
    FilterSet   FilterSet        // Active filters
    OpenFiles   []FileTab        // File tabs
    UIState     UIState          // Interface state
}
```

### Configuration

```go
type Config struct {
    Version     string
    Performance PerformanceConfig  // Streaming thresholds, cache sizes
    UI          UIConfig          // Colors, layout, keybindings
    DataMgmt    DataConfig        // Session limits, retention
    FileHandling FileConfig       // Encoding, export options
}
```

## Development Guidelines

### Constitutional Requirements (NON-NEGOTIABLE)

1. **Modal Interface Discipline**: Strict Vim-style Normal/Insert modes
2. **Filter-First Architecture**: Filter specifications are source of truth
3. **Component Modularity**: Independent, testable components
4. **Real-Time Feedback**: Immediate UI updates for user actions
5. **Text-Stream Protocol**: CLI compatibility and automation support
6. **Performance Requirements**: Handle 100MB+ files responsively

### Code Patterns

#### Bubble Tea Component Structure

```go
type ComponentModel struct {
    // State fields
    focused bool
    items   []Item
}

func (m ComponentModel) Init() tea.Cmd { return nil }

func (m ComponentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKeypress(msg)
    case CustomMsg:
        return m.handleCustomMessage(msg)
    }
    return m, nil
}

func (m ComponentModel) View() string {
    // Render component UI
}
```

#### Message Passing Pattern

```go
// Define custom messages for component communication
type FilterUpdateMsg struct {
    FilterSet FilterSet
}

// Emit messages from components
return m, tea.Cmd(func() tea.Msg {
    return FilterUpdateMsg{FilterSet: m.filterSet}
})
```

#### Configuration Hot-Reload

```go
type ConfigWatcher struct {
    path     string
    callback func(Config)
}

func (w *ConfigWatcher) Watch(ctx context.Context) {
    // File watching implementation
    // Call callback on config changes
}
```

### Testing Approach

#### TDD Workflow (REQUIRED)

1. Write failing contract test
2. Write failing integration test
3. Write failing unit test
4. Implement minimal code to pass
5. Refactor while keeping tests green

#### Test Structure

```go
func TestComponentContract(t *testing.T) {
    // Test component meets interface requirements
    var _ tea.Model = (*ComponentModel)(nil)

    // Test message handling
    model := NewComponent()
    updatedModel, cmd := model.Update(testMsg)

    assert.NotNil(t, cmd)
    assert.Equal(t, expectedState, updatedModel.state)
}
```

## Performance Considerations

### Memory Management

- LRU cache for compiled regex patterns (configurable size)
- Streaming mode for files > threshold (default 100MB)
- Circular buffer for file content (configurable line limit)
- Session cleanup based on retention policies

### Responsiveness Targets

- Keystroke response: <50ms
- Filter updates: <150ms (configurable debounce)
- File loading: <1s for small files, streaming for large
- Tab switching: <100ms

### Concurrency

- File I/O in separate goroutines
- Non-blocking UI updates
- Context-based cancellation for long operations
- Worker pool for parallel pattern matching

## Common Implementation Patterns

### Error Handling

```go
// Always provide context in errors
return fmt.Errorf("failed to compile pattern %q: %w", pattern, err)

// Use ErrorMsg for user-facing errors
return m, tea.Cmd(func() tea.Msg {
    return ErrorMsg{
        Message: "Invalid regex pattern",
        Context: "pattern validation",
        Recoverable: true,
    }
})
```

### Configuration Validation

```go
func (c *Config) Validate() error {
    if c.Performance.DebounceDelayMs < 50 || c.Performance.DebounceDelayMs > 1000 {
        return fmt.Errorf("debounce_delay_ms must be between 50 and 1000, got %d",
            c.Performance.DebounceDelayMs)
    }
    // Additional validation...
}
```

### File Operations

```go
func (r *FileReader) ReadWithContext(ctx context.Context, path string) (<-chan Line, error) {
    lineChan := make(chan Line, 100)

    go func() {
        defer close(lineChan)
        // Reading implementation with context checking
        select {
        case lineChan <- line:
        case <-ctx.Done():
            return
        }
    }()

    return lineChan, nil
}
```

## Development Status

**Current Phase**: Implementation Planning
**Branch**: `001-qf-core-concept`
**Next Steps**:

1. Create task breakdown (/tasks command)
2. Implement core data structures
3. Build Bubble Tea components
4. Add configuration system
5. Implement filtering engine

## Troubleshooting Tips

### Common Issues

- **Slow filtering**: Check regex complexity, consider pattern optimization
- **Memory usage**: Verify streaming threshold, check buffer limits
- **UI lag**: Profile message handling, optimize rendering
- **Config errors**: Validate JSON schema, check file permissions

### Debug Commands

```bash
# Enable debug logging
qf --debug application.log

# Profile performance
go tool pprof qf profile.pprof

# Test configuration
qf --config validate

# Check session files
ls -la ~/.config/qf/sessions/
```

---
*Claude Code Context - Last updated: 2025-09-13*
