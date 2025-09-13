# Phase 0: Research Findings - Interactive Log Filter Composer (qf)

**Date**: 2025-09-13
**Status**: Complete
**Next Phase**: Design & Contracts

## Research Overview

This document consolidates research findings for technical decisions needed to implement the Interactive Log Filter Composer. All NEEDS CLARIFICATION items from the technical context have been resolved through analysis of requirements, performance constraints, and constitutional principles.

## Research Topics & Findings

### 1. Bubble Tea TUI Architecture

**Research Question**: How to structure components for modal interface with real-time filtering?

**Decision**: Component-based architecture with centralized message passing
**Rationale**:

- Bubble Tea's `tea.Model` interface provides clean separation between UI components
- Message passing enables loose coupling and testable interactions
- Viewport components handle scrolling and large content efficiently
- Modal state management through application-level state machine

**Alternatives Considered**:

- Direct terminal manipulation: Too complex, poor maintainability
- Immediate mode GUI: Doesn't fit terminal constraints
- Event-driven architecture: Conflicts with Bubble Tea patterns

**Implementation Approach**:

- Main app model coordinates pane focus and mode transitions
- Each pane (Include, Exclude, Viewer) as independent `tea.Model`
- Custom messages for filter updates, file operations, configuration changes
- Viewport component for scrollable content with context lines

### 2. Go Regex Performance Optimization

**Research Question**: How to handle regex compilation and matching performance for real-time filtering?

**Decision**: LRU cache for compiled patterns with configurable limits
**Rationale**:

- Regex compilation is expensive (5-20ms per pattern)
- Users typically reuse patterns frequently
- LRU eviction handles memory bounds gracefully
- Cache hit rates >80% expected in typical usage

**Alternatives Considered**:

- No caching: Poor performance with pattern reuse
- Unlimited cache: Memory growth risk with many patterns
- Simple map cache: No memory bounds

**Implementation Approach**:

- `github.com/hashicorp/golang-lru` for LRU implementation
- Cache compiled `*regexp.Regexp` objects by pattern string
- Configurable cache size (default 100 patterns)
- Cache statistics for observability (hit rate, evictions)

### 3. Configuration Management System

**Research Question**: How to handle JSON configuration with validation, hot-reload, and migration?

**Decision**: JSON with atomic writes and schema validation
**Rationale**:

- JSON is human-readable and widely supported
- Go's `json` package provides excellent validation
- Atomic writes prevent corruption during updates
- Schema evolution through version field

**Alternatives Considered**:

- YAML: Additional parsing dependency
- TOML: Less familiar format
- Binary formats: Not user-editable

**Implementation Approach**:

- Struct tags for JSON validation (`validate` package)
- Atomic file writes using temp files + rename
- Configuration hot-reload via file watching
- Version-based migration with backwards compatibility
- Default value injection for missing fields

### 4. Large File Streaming Patterns

**Research Question**: How to handle files larger than available memory while maintaining UI responsiveness?

**Decision**: Channel-based streaming with configurable buffering
**Rationale**:

- Go channels provide backpressure naturally
- Streaming enables constant memory usage regardless of file size
- UI remains responsive during background processing
- Cancellation support for large operations

**Alternatives Considered**:

- Memory mapping: Platform-specific, complexity with large files
- Buffer pools: Complex memory management
- Full file loading: Memory constraints with large files

**Implementation Approach**:

- Producer goroutine streams lines via channel
- Consumer processes lines with filtering logic
- Configurable buffer size (default 1000 lines)
- Context-based cancellation for operations
- Progress reporting for large file operations

### 5. Terminal Compatibility Matrix

**Research Question**: How to ensure consistent behavior across different terminal implementations?

**Decision**: Lipgloss styling with terminal capability detection
**Rationale**:

- Lipgloss handles terminal capability detection automatically
- Graceful degradation for limited color terminals
- Consistent cross-platform behavior
- Accessibility support with high-contrast fallbacks

**Alternatives Considered**:

- Direct ANSI codes: Fragile across terminals
- No styling: Poor user experience
- Platform-specific detection: Maintenance burden

**Implementation Approach**:

- Lipgloss `Color` type with fallback handling
- Terminal size detection and responsive layout
- Color scheme configuration with presets
- Minimum terminal size requirements (80x24)
- Unicode support detection for borders

### 6. Session Persistence Strategy

**Research Question**: How to safely persist user sessions with corruption recovery?

**Decision**: JSON session files with atomic writes and backup recovery
**Rationale**:

- JSON enables version control and debugging
- Atomic writes prevent partial corruption
- Backup rotation provides recovery options
- Human-readable format for troubleshooting

**Alternatives Considered**:

- SQLite: Overkill for single-user data
- Binary formats: Debugging difficulty
- No persistence: Poor user experience

**Implementation Approach**:

- Session files in `~/.config/qf/sessions/`
- JSON format with schema versioning
- Atomic writes with backup rotation (3 backups)
- Corruption detection and automatic recovery
- Session history cleanup based on age/count limits

## Performance Characteristics

Based on research findings, expected performance characteristics:

### Memory Usage

- Base application: ~10MB
- Per-file buffer: Configurable (default ~10MB for streaming)
- Regex cache: ~100KB (100 patterns × ~1KB each)
- UI rendering: ~1MB (terminal buffers and components)

### CPU Performance

- Pattern compilation: 5-20ms (cached after first use)
- Line filtering: ~100K lines/second (simple patterns)
- File streaming: I/O bounded (typically >1M lines/second)
- UI updates: <16ms (60fps target)

### Responsiveness Targets

- Keystroke response: <50ms
- Filter updates: <150ms (configurable debounce)
- File loading: <1s for files <10MB, streaming for larger
- Configuration reload: <100ms

## Risk Assessment

### Technical Risks

1. **Terminal Compatibility**: Mitigated by Lipgloss capability detection
2. **Memory Growth**: Mitigated by LRU caches and configurable limits
3. **File Handle Limits**: Mitigated by concurrent file limits (default 5)
4. **Configuration Corruption**: Mitigated by atomic writes and backups

### User Experience Risks

1. **Complex Regex Patterns**: Mitigated by pattern validation and help system
2. **Large File Performance**: Mitigated by streaming mode and progress indicators
3. **Session Data Loss**: Mitigated by auto-save and backup recovery
4. **Keyboard Navigation**: Mitigated by standard Vim bindings and help overlay

## Dependencies Validated

All primary dependencies have been researched and validated:

- **Bubble Tea v1.3.9**: Mature TUI framework, active development
- **Lipgloss v1.1.0**: Styling library, well-maintained
- **Bubbles v0.21.0**: Additional components, official support
- **clipboard v0.1.4**: Cross-platform clipboard support

## Next Steps

All research questions have been resolved. Ready to proceed to Phase 1: Design & Contracts.

Key outputs for Phase 1:

1. **Data Model**: Entity definitions based on research findings
2. **Contracts**: Component interfaces using Bubble Tea patterns
3. **Quickstart**: User interaction flows and test scenarios
4. **CLAUDE.md**: Agent context file for development assistance

---
*Research complete: 2025-09-13*
