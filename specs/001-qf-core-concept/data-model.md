# Data Model - Interactive Log Filter Composer (qf)

**Date**: 2025-09-13
**Status**: Complete
**Dependencies**: research.md findings

## Entity Definitions

### Core Filtering Entities

#### Pattern

Represents a single regex pattern with metadata for filtering operations.

```go
type PatternType int

const (
    PatternInclude PatternType = iota
    PatternExclude
)

type Pattern struct {
    ID          string           `json:"id"`          // UUID for pattern identification
    Expression  string           `json:"expression"`  // Raw regex pattern
    Type        PatternType      `json:"type"`        // Include or Exclude
    MatchCount  int              `json:"match_count"` // Statistics: matches found
    Color       string           `json:"color"`       // Hex color for highlighting
    Created     time.Time        `json:"created"`     // Creation timestamp
    LastUsed    time.Time        `json:"last_used"`   // Last usage for history sorting
    IsValid     bool             `json:"is_valid"`    // Validation status
    Error       string           `json:"error,omitempty"` // Validation error message
}
```

**Validation Rules**:

- `Expression`: Non-empty, valid regex syntax
- `Type`: Must be PatternInclude or PatternExclude
- `Color`: Valid hex color code or empty for default
- `ID`: Generated UUID, immutable after creation

**State Transitions**:

- Created → Valid (on successful regex compilation)
- Created → Invalid (on compilation error)
- Valid ↔ Invalid (on expression changes)

#### FilterSet

Collection of patterns that work together as a filtering unit.

```go
type FilterSet struct {
    Include      []Pattern    `json:"include"`       // Include patterns (OR logic)
    Exclude      []Pattern    `json:"exclude"`       // Exclude patterns (veto logic)
    Name         string       `json:"name"`          // Human-readable name
    LastModified time.Time    `json:"last_modified"` // Modification timestamp
    Description  string       `json:"description,omitempty"` // Optional description
}
```

**Validation Rules**:

- At least one pattern (include or exclude) must be present
- Pattern IDs must be unique within the set
- Name must be non-empty for saved filter sets

**Filter Logic**:

1. Line matches if ANY include pattern matches (OR logic)
2. Line is excluded if ANY exclude pattern matches (veto logic)
3. Empty include list = show all lines (subject to excludes)
4. Empty exclude list = no exclusions applied

### File Management Entities

#### FileTab

Represents an open log file with its viewing state.

```go
type ViewState struct {
    ScrollOffset   int     `json:"scroll_offset"`   // Current scroll position
    ContextLines   int     `json:"context_lines"`   // Lines before/after matches
    ShowLineNumbers bool   `json:"show_line_numbers"` // Display line numbers
    WrapLines      bool    `json:"wrap_lines"`      // Text wrapping enabled
}

type FileTab struct {
    Path         string      `json:"path"`          // Absolute file path
    DisplayName  string      `json:"display_name"`  // Tab display name
    Content      *FileBuffer `json:"-"`             // File content (not serialized)
    ViewState    ViewState   `json:"view_state"`    // UI state
    IsModified   bool        `json:"is_modified"`   // Has unsaved changes
    Size         int64       `json:"size"`          // File size in bytes
    LastRead     time.Time   `json:"last_read"`     // Last read timestamp
    IsStreaming  bool        `json:"is_streaming"`  // Using streaming mode
    IsFollowing  bool        `json:"is_following"`  // Following file changes (tail -f)
}
```

**Validation Rules**:

- `Path`: Must exist and be readable
- `DisplayName`: Auto-generated from filename if empty
- `ContextLines`: 0-50 (configurable limit)
- Size limits enforced via configuration

#### FileBuffer

Manages file content with streaming support for large files.

```go
type Line struct {
    Number    int      `json:"number"`    // Original line number
    Text      string   `json:"text"`      // Line content
    Matches   []Match  `json:"matches"`   // Pattern matches in this line
    IsVisible bool     `json:"is_visible"` // Passes current filter
}

type Match struct {
    PatternID string `json:"pattern_id"` // Which pattern matched
    Start     int    `json:"start"`      // Match start position
    End       int    `json:"end"`        // Match end position
    Color     string `json:"color"`      // Highlight color
}

type FileBuffer struct {
    Lines       []Line    `json:"lines"`        // Loaded lines
    TotalLines  int       `json:"total_lines"`  // Total lines in file
    IsComplete  bool      `json:"is_complete"`  // All lines loaded
    StreamPos   int64     `json:"stream_pos"`   // Current stream position
    BufferSize  int       `json:"buffer_size"`  // Max lines in memory
}
```

**Validation Rules**:

- Lines array size ≤ configured buffer limit
- Line numbers must be sequential and positive
- Match positions must be within line text bounds

### Session Management Entities

#### Session

Complete workspace state including filters, files, and UI configuration.

```go
type Session struct {
    Name         string      `json:"name"`          // Session identifier
    FilterSet    FilterSet   `json:"filter_set"`    // Active filter configuration
    OpenFiles    []FileTab   `json:"open_files"`    // Open file tabs
    ActiveTab    int         `json:"active_tab"`    // Currently selected tab
    UIState      UIState     `json:"ui_state"`      // Interface configuration
    Created      time.Time   `json:"created"`       // Creation time
    LastAccessed time.Time   `json:"last_accessed"` // Last access time
    AutoSave     bool        `json:"auto_save"`     // Auto-save enabled
}

type UIState struct {
    Mode           string  `json:"mode"`             // "normal" or "insert"
    FocusedPane    string  `json:"focused_pane"`     // Current pane focus
    PaneSizes      map[string]float64 `json:"pane_sizes"` // Pane size ratios
    ShowHelp       bool    `json:"show_help"`        // Help overlay visible
    ColorScheme    string  `json:"color_scheme"`     // Active color scheme
}
```

**Validation Rules**:

- `Name`: Non-empty, filesystem-safe characters
- `ActiveTab`: Valid index within OpenFiles range
- `Mode`: Must be "normal" or "insert"
- `FocusedPane`: Must be valid pane identifier
- Pane size ratios must sum to 1.0

### Configuration Entities

#### Config

Application-wide configuration with validation and migration support.

```go
type Config struct {
    Version      string           `json:"version"`       // Schema version
    Performance  PerformanceConfig `json:"performance"`   // Performance settings
    UI           UIConfig         `json:"ui"`            // Interface settings
    DataMgmt     DataConfig       `json:"data_management"` // Data handling
    FileHandling FileConfig       `json:"file_handling"` // File operations
}

type PerformanceConfig struct {
    StreamingThresholdMB int `json:"streaming_threshold_mb" validate:"min=1,max=10000"`
    DebounceDelayMs      int `json:"debounce_delay_ms" validate:"min=50,max=1000"`
    RegexCacheSize       int `json:"regex_cache_size" validate:"min=10,max=1000"`
    SampleSize           int `json:"sample_size" validate:"min=100,max=10000"`
    BufferSizeMB         int `json:"buffer_size_mb" validate:"min=1,max=100"`
}

type UIConfig struct {
    DefaultContextLines  int               `json:"default_context_lines" validate:"min=0,max=50"`
    ColorScheme         string             `json:"color_scheme"`
    AutoSaveIntervalSec int               `json:"auto_save_interval_sec" validate:"min=10,max=600"`
    PaneSizes           map[string]float64 `json:"pane_sizes"`
}

type DataConfig struct {
    MaxConcurrentFiles   int `json:"max_concurrent_files" validate:"min=1,max=20"`
    SessionHistoryCount  int `json:"session_history_count" validate:"min=10,max=1000"`
    SessionRetentionDays int `json:"session_retention_days" validate:"min=1,max=3650"`
    PatternHistorySize   int `json:"pattern_history_size" validate:"min=10,max=500"`
    TemplateStoragePath  string `json:"template_storage_path"`
}

type FileConfig struct {
    DefaultEncoding       string `json:"default_encoding"`
    MaxLineLength        int    `json:"max_line_length" validate:"min=1000,max=50000"`
    ExportDefaultFormat  string `json:"export_default_format"`
    ExportDefaultDir     string `json:"export_default_directory"`
}
```

**Validation Rules**:

- All numeric fields have min/max constraints via struct tags
- Paths must be valid and accessible
- Color scheme must exist in predefined list
- Pane size ratios must be positive and sum to 1.0

### Template & History Entities

#### PatternTemplate

Predefined patterns for common filtering scenarios.

```go
type PatternTemplate struct {
    Name        string      `json:"name"`        // Display name
    Pattern     string      `json:"pattern"`     // Regex pattern
    Type        PatternType `json:"type"`        // Include or Exclude
    Description string      `json:"description"` // Usage description
    Category    string      `json:"category"`    // Grouping category
    Builtin     bool        `json:"builtin"`     // System vs user template
}
```

#### PatternHistory

Recent pattern usage for quick access.

```go
type PatternHistory struct {
    Patterns []HistoryEntry `json:"patterns"`
    MaxSize  int           `json:"max_size"`
}

type HistoryEntry struct {
    Pattern   string    `json:"pattern"`
    Type      PatternType `json:"type"`
    UsageCount int      `json:"usage_count"`
    LastUsed  time.Time `json:"last_used"`
}
```

## Entity Relationships

```
Session (1) ──── (n) FileTab
    │                │
    └── FilterSet    └── FileBuffer
          │              │
          └── (n) Pattern └── (n) Line
                │              │
                └── Match ──────┘

Config ──── (global settings)

PatternTemplate ──── (predefined patterns)
PatternHistory  ──── (usage tracking)
```

## State Management Rules

### Pattern Management

- Patterns are immutable after creation (create new for changes)
- Pattern compilation happens lazily on first use
- Invalid patterns remain in UI with error indicators
- Pattern statistics updated on each filter application

### Session Persistence

- Auto-save every 30 seconds (configurable)
- Manual save on session switch or application exit
- Atomic writes to prevent corruption
- Backup rotation (3 most recent versions)

### File Buffer Management

- Streaming mode activated when file size > threshold
- Buffer maintains fixed size window of recent lines
- Context lines loaded on-demand around matches
- File watching for live updates when following

### Configuration Updates

- Hot-reload supported for most settings
- UI-affecting changes require component refresh
- Invalid configurations fall back to defaults
- Migration scripts handle version upgrades

---
*Data model complete: 2025-09-13*
