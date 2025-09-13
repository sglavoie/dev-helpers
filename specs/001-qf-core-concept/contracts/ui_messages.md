# UI Message Contracts - Bubble Tea Components

**Date**: 2025-09-13
**Purpose**: Define message types for component communication in Bubble Tea architecture

## Message Type Definitions

### Filter Management Messages

```go
// Pattern-related messages
type PatternAddMsg struct {
    Pattern     string
    Type        PatternType
    Position    int // Insert position in list
}

type PatternEditMsg struct {
    PatternID   string
    Expression  string
}

type PatternDeleteMsg struct {
    PatternID   string
}

type PatternValidatedMsg struct {
    PatternID   string
    IsValid     bool
    Error       string
}

// Filter application messages
type FilterApplyMsg struct {
    FilterSet   FilterSet
    FileIndex   int // Which file tab to apply to (-1 for all)
}

type FilterResultMsg struct {
    FileIndex   int
    MatchCount  int
    FilterTime  time.Duration
    Error       error
}
```

### File Operation Messages

```go
// File loading messages
type FileOpenMsg struct {
    Path        string
    StreamMode  bool
}

type FileLoadedMsg struct {
    Tab         FileTab
    Error       error
}

type FileCloseMsg struct {
    TabIndex    int
}

// File content messages
type FileContentMsg struct {
    TabIndex    int
    Lines       []Line
    IsComplete  bool
    Error       error
}

type FileWatchMsg struct {
    TabIndex    int
    NewLines    []Line
}
```

### Navigation Messages

```go
// Pane focus messages
type FocusMsg struct {
    Pane        string // "include", "exclude", "viewer", "help"
}

type ModeChangeMsg struct {
    Mode        string // "normal", "insert"
    Context     string // Which component triggered the change
}

// Tab management messages
type TabSwitchMsg struct {
    TabIndex    int
}

type TabCloseMsg struct {
    TabIndex    int
}
```

### Session Messages

```go
// Session management
type SessionSaveMsg struct {
    Name        string
    AutoSave    bool
}

type SessionLoadMsg struct {
    Name        string
}

type SessionSavedMsg struct {
    Name        string
    Error       error
}

type SessionLoadedMsg struct {
    Session     Session
    Error       error
}
```

### Configuration Messages

```go
// Configuration updates
type ConfigUpdateMsg struct {
    Config      Config
    HotReload   bool // Whether to apply immediately
}

type ConfigReloadMsg struct {
    Path        string // Config file path
}

type ConfigLoadedMsg struct {
    Config      Config
    Error       error
}
```

### Error and Status Messages

```go
// Error reporting
type ErrorMsg struct {
    Message     string
    Context     string // Component or operation context
    Recoverable bool
}

// Status updates
type StatusMsg struct {
    Message     string
    Type        StatusType // Info, Warning, Error
    Duration    time.Duration // How long to display
}

type StatusType int

const (
    StatusInfo StatusType = iota
    StatusWarning
    StatusError
)
```

## Component Interface Contracts

### Filter Pane Component

```go
// Filter pane must implement tea.Model
type FilterPaneModel interface {
    tea.Model

    // State queries
    GetPatterns() []Pattern
    GetFocusedIndex() int
    IsInEditMode() bool

    // State updates
    AddPattern(pattern Pattern) tea.Cmd
    EditPattern(id string, expression string) tea.Cmd
    DeletePattern(id string) tea.Cmd
    SetFocus(index int) tea.Cmd

    // Mode transitions
    EnterEditMode() tea.Cmd
    ExitEditMode() tea.Cmd
}
```

**Message Handling Contract**:

- MUST respond to `PatternAddMsg`, `PatternEditMsg`, `PatternDeleteMsg`
- MUST emit `PatternValidatedMsg` after pattern changes
- MUST emit `FilterApplyMsg` when patterns change in normal mode
- MUST handle keyboard navigation (j/k, Enter, Escape)

### Content Viewer Component

```go
type ViewerModel interface {
    tea.Model

    // Content management
    SetContent(lines []Line) tea.Cmd
    AppendContent(lines []Line) tea.Cmd
    ClearContent() tea.Cmd

    // View state
    GetViewState() ViewState
    SetViewState(state ViewState) tea.Cmd
    ScrollTo(lineNumber int) tea.Cmd

    // Search/navigation
    FindNext(patternID string) tea.Cmd
    FindPrevious(patternID string) tea.Cmd
    GetVisibleRange() (start, end int)
}
```

**Message Handling Contract**:

- MUST respond to `FileContentMsg`, `FilterResultMsg`
- MUST handle scroll events (Ctrl+d, Ctrl+u, gg, G)
- MUST emit navigation messages for match jumping (n, N)
- MUST render highlighted matches with appropriate colors

### Tab Manager Component

```go
type TabManagerModel interface {
    tea.Model

    // Tab management
    AddTab(tab FileTab) tea.Cmd
    CloseTab(index int) tea.Cmd
    SwitchTab(index int) tea.Cmd
    GetActiveTab() *FileTab
    GetTabCount() int

    // State queries
    IsVisible() bool // Only show when multiple tabs
    GetTabNames() []string
}
```

**Message Handling Contract**:

- MUST respond to `TabSwitchMsg`, `TabCloseMsg`, `FileOpenMsg`
- MUST emit `TabSwitchMsg` on tab selection
- MUST handle keyboard navigation (Tab, Shift+Tab, 1-9 for direct selection)
- MUST show/hide based on tab count (hidden when ≤1 tab)

### Application Model

```go
type AppModel interface {
    tea.Model

    // Global state
    GetCurrentSession() Session
    GetConfig() Config
    GetMode() string
    GetFocusedPane() string

    // Command coordination
    BroadcastMessage(msg tea.Msg) tea.Cmd
    SaveSession() tea.Cmd
    LoadSession(name string) tea.Cmd

    // Error handling
    ShowError(err error) tea.Cmd
    ShowStatus(message string, statusType StatusType) tea.Cmd
}
```

**Message Handling Contract**:

- MUST coordinate messages between all child components
- MUST handle global keyboard shortcuts (quit, help, save)
- MUST manage mode transitions (Normal/Insert)
- MUST persist session state on changes

## Testing Contracts

### Component Test Requirements

Each component MUST have tests that verify:

1. **Message Handling**:

   ```go
   func TestComponentHandlesRequiredMessages(t *testing.T) {
       // Test each required message type
       // Verify correct state changes
       // Verify emitted commands
   }
   ```

2. **State Consistency**:

   ```go
   func TestComponentStateConsistency(t *testing.T) {
       // Test state transitions are valid
       // Test invariants are maintained
       // Test error states are handled
   }
   ```

3. **Keyboard Handling**:

   ```go
   func TestComponentKeyboardNavigation(t *testing.T) {
       // Test all keyboard shortcuts
       // Test modal behavior (Normal/Insert)
       // Test focus management
   }
   ```

### Integration Test Requirements

Components MUST pass integration tests for:

1. **Message Flow**:

   ```go
   func TestMessageFlowBetweenComponents(t *testing.T) {
       // Test complete user workflows
       // Verify message propagation
       // Test error propagation
   }
   ```

2. **State Synchronization**:

   ```go
   func TestStateSynchronization(t *testing.T) {
       // Test filter changes update viewer
       // Test tab switches update content
       // Test config changes update UI
   }
   ```

## Error Handling Contract

All components MUST:

- Never panic on invalid input
- Emit `ErrorMsg` for user-facing errors
- Log technical errors with context
- Maintain UI responsiveness during errors
- Provide recovery mechanisms where possible

## Performance Contract

All components MUST:

- Respond to user input within 50ms
- Handle large datasets without blocking UI
- Implement proper cleanup for goroutines
- Use context for cancellable operations
- Respect memory limits from configuration

---
*UI Message Contracts complete: 2025-09-13*
