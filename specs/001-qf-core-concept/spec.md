# Feature Specification: Interactive Log Filter Composer

**Feature Branch**: `001-qf-core-concept`
**Created**: 2025-09-13
**Status**: Draft
**Input**: User description: "Core Concept

A visual, interactive filter composer that transforms log filtering from a command-line puzzle into a visual, iterative experience. Think of it as a \"regex playground\" meets \"log viewer\" with Vim muscle memory.

User Workflow Philosophy

The tool should feel like you're \"sculpting\" your view of the data - you add include patterns to carve out what you want to see, and exclude patterns to chisel away the noise. The live preview makes this a visual, immediate process rather than a guess-and-check cycle.

Modal Behavior Design

The Two-Mode System

- Normal Mode (for include/exclude panes): Your hands stay on the home row, shortcuts are immediate. Press 'm' and the pane expands. Press 'j/k' to navigate entries. This is your \"command center\" mode.
- Insert Mode: Activated with 'i', this is where you craft your regex patterns. The transition should feel natural to Vim users
    - Escape always brings you back to safety (Normal mode).

Filter Composition Logic

Include Filters: Work as an OR operation - if ANY include pattern matches, the line is a candidate
Exclude Filters: Work as a veto system - if ANY exclude pattern matches, the line is rejected
Empty States:

- No includes = show everything (except excludes)
- No excludes = show only includes
- Neither = show entire file

Advanced Functionality Ideas

Pattern Templates/Presets

- Quick patterns like \"errors only\", \"timestamps\", \"IPs\", accessible via shortcuts
- Save frequently used filter combinations
- Pattern history (like shell history) accessible with arrow keys

Smart Pattern Suggestions

- Detect common patterns in the file (timestamps, log levels, IPs)
- Offer these as quick-add options
- Learn from user's pattern history

Multi-File Support (Right pane with tabs)

- Open multiple files in tabs
- Apply same filter set across files
- Compare filtered results side-by-side

Filter Chaining/Pipelines

- Save filter sets as \"lenses\"
- Apply multiple lenses in sequence
- Export filter sets for reuse

Context Controls

- Show N lines before/after matches (like grep -A/-B)
- Highlight matches within the results
- Different highlight colors for different include patterns

Performance Optimizations

- Lazy evaluation - only process minimum amount of data from large files
- Incremental filtering as you type (with debouncing)
- Cache compiled regexes
- Background pre-filtering for large files
- Must support piping in to get content into the tool, e.g. `tail -n 100 | qf`

Interactive Features

Live Feedback

- Show match count in real-time
- Indicate which patterns are matching most/least
- Syntax highlighting for regex patterns

Pattern Testing Mode

- Test a pattern before adding it
    - Opens a new overlay on top of the screen
    - Can type the pattern
    - Once satisfied, can pick whether to add it to the `include` or `exclude` pane
- Show what would be matched/excluded

Collaboration Features

- Export filter sets as shareable configs
- Import filter sets from teammates
- Generate equivalent rg (ripgrep) commands

Navigation Enhancements

Smart Jumping

- Jump to specific log levels
- Jump between pattern matches
    - When inside the `include` pane, `n` scrolls to the next match in the pager/viewer, `N` backward, for the currently selected include pattern. The same is true for the `exclude` pattern.

Output Options

Export Capabilities

- Filtered results to new file
- Copy as formatted text
- Copy with line numbers preserved

Quality of Life Features

Help System

- Context-sensitive help
- Regex cheat sheet
- Common patterns library

Session Management

- Auto-save current filter set
- Restore last session
- Named sessions for different debugging scenarios
- Allow visualizing the include/exclude patterns for all sessions before applying it to the current context

Edge Cases to Consider

Large Files

- Streaming mode for files larger than memory
- Sampling mode for initial preview
- Progressive loading with visual feedback

Binary/Encoded Content

- Detect and handle gracefully
- Hex view mode option
- Base64 decode option

Live File Following

- Tail -f mode with filters
- Pause/resume following
- Buffer management for high-volume streams

Complex Patterns

- Validate regex as you type

Integration Points

Shell Integration

- Pipe support (stdin/stdout)
- Integration with existing Unix tools"

## Execution Flow (main)

```
1. Parse user description from Input
   � If empty: ERROR "No feature description provided"
2. Extract key concepts from description
   � Identify: actors, actions, data, constraints
3. For each unclear aspect:
   � Mark with [NEEDS CLARIFICATION: specific question]
4. Fill User Scenarios & Testing section
   � If no clear user flow: ERROR "Cannot determine user scenarios"
5. Generate Functional Requirements
   � Each requirement must be testable
   � Mark ambiguous requirements
6. Identify Key Entities (if data involved)
7. Run Review Checklist
   � If any [NEEDS CLARIFICATION]: WARN "Spec has uncertainties"
   � If implementation details found: ERROR "Remove tech details"
8. Return: SUCCESS (spec ready for planning)
```

---

## � Quick Guidelines

-  Focus on WHAT users need and WHY
- L Avoid HOW to implement (no tech stack, APIs, code structure)
- =e Written for business stakeholders, not developers

### Section Requirements

- **Mandatory sections**: Must be completed for every feature
- **Optional sections**: Include only when relevant to the feature
- When a section doesn't apply, remove it entirely (don't leave as "N/A")

### For AI Generation

When creating this spec from a user prompt:

1. **Mark all ambiguities**: Use [NEEDS CLARIFICATION: specific question] for any assumption you'd need to make
2. **Don't guess**: If the prompt doesn't specify something (e.g., "login system" without auth method), mark it
3. **Think like a tester**: Every vague requirement should fail the "testable and unambiguous" checklist item
4. **Common underspecified areas**:
   - User types and permissions
   - Data retention/deletion policies
   - Performance targets and scale
   - Error handling behaviors
   - Integration requirements
   - Security/compliance needs

---

## User Scenarios & Testing *(mandatory)*

### Primary User Story

A developer or system administrator needs to analyze log files to troubleshoot issues or monitor system behavior. Instead of crafting complex grep commands or using multiple tools, they open the interactive log filter composer which allows them to visually build filters by adding include patterns (to show relevant lines) and exclude patterns (to hide noise). They can see live results as they build filters, making the debugging process more intuitive and efficient.

### Acceptance Scenarios

1. **Given** a log file is loaded, **When** user adds an include pattern for "ERROR", **Then** only lines containing "ERROR" are displayed in real-time
2. **Given** filtered results showing error lines, **When** user adds an exclude pattern for "connection timeout", **Then** error lines containing "connection timeout" are hidden from view
3. **Given** user is in Normal mode, **When** user presses 'i' on a pattern, **Then** system enters Insert mode for editing that regex pattern
4. **Given** user has created multiple filters, **When** user saves the filter set as "production-debugging", **Then** the filter combination can be reloaded in future sessions
5. **Given** user has applied filters, **When** user presses 'n' while focused on an include pattern, **Then** system jumps to next match of that specific pattern
6. **Given** a large log file, **When** user starts typing a pattern, **Then** results update incrementally with debouncing to maintain performance
7. **Given** application is run for the first time, **When** user starts the application, **Then** system creates ~/.config/qf.json with default configuration values
8. **Given** user wants to change debounce delay, **When** user runs config edit command, **Then** system opens configuration editor for interactive modification
9. **Given** configuration file contains invalid values, **When** application starts, **Then** system logs warnings and uses default values for invalid settings
10. **Given** user updates streaming threshold to 50MB, **When** user opens a 75MB log file, **Then** system automatically switches to streaming mode
11. **Given** user modifies configuration externally, **When** user runs config reload command, **Then** system applies new settings without restart

### Edge Cases

- What happens when a regex pattern is invalid? System should validate and show syntax errors
- How does system handle binary or non-text files? Should detect and offer hex/base64 decode options
- What happens when log file is larger than available memory? Should use streaming mode with sampling
- How does system behave when multiple include patterns match the same line? Line should be shown once with appropriate highlighting
- What happens when user tries to follow a live log file that's being written to rapidly? Should provide pause/resume and buffer management
- What happens when configuration file is corrupted or contains malformed JSON? System should create backup and recreate with defaults
- What happens when configuration directory (~/.config) doesn't exist or lacks write permissions? Should handle gracefully and use in-memory defaults
- What happens when user tries to set invalid values (negative numbers, out-of-range values)? Should validate and reject with helpful error messages
- What happens during configuration migration when new version adds settings? Should preserve existing settings and add new defaults
- What happens when multiple application instances try to modify configuration simultaneously? Should handle file locking or conflicts gracefully

## Configuration Management *(mandatory)*

### Configuration File Location & Structure

The system uses a JSON configuration file located at `~/.config/qf.json` to store user preferences and system settings. This file is automatically created with default values on first run if it doesn't exist.

### Configuration Schema

The configuration file contains the following sections:

**Performance Settings:**

- `streaming_threshold`: File size threshold (in MB) for switching to streaming mode (default: 100MB)
- `debounce_delay`: Delay in milliseconds for incremental filtering (default: 150ms)
- `regex_cache_size`: Maximum number of compiled regex patterns to cache (default: 100)
- `sample_size`: Number of lines to show for large file preview (default: 1000)
- `buffer_size`: Buffer size in MB for live file following (default: 10MB)

**User Interface Settings:**

- `default_context_lines`: Default number of lines to show before/after matches (default: 3)
- `color_scheme`: Color scheme name for pattern highlighting (default: "default")
- `auto_save_interval`: Automatic session save interval in seconds (default: 30)
- `pane_sizes`: Default pane size ratios for UI layout

**Data Management Settings:**

- `max_concurrent_files`: Maximum number of files that can be opened simultaneously (default: 5)
- `session_history_count`: Number of previous sessions to retain (default: 100)
- `session_retention_days`: Number of days to keep session history (default: 180)
- `pattern_history_size`: Maximum number of patterns to keep in history (default: 50)
- `template_storage_path`: Directory for storing custom pattern templates

**File Handling Settings:**

- `default_encoding`: Default file encoding (default: "UTF-8")
- `max_line_length`: Maximum characters per line before truncation (default: 5000)
- `export_default_format`: Default format for exporting results (default: "text")
- `export_default_directory`: Default directory for exports (default: "~/Downloads")

### Configuration Management Behaviors

**Initialization:** System creates `~/.config/qf.json` with default values if the file doesn't exist when the application starts.

**Validation:** All configuration values are validated against expected types and ranges. Invalid values are logged as warnings and replaced with defaults.

**CLI Override:** Command-line flags can override configuration file values for individual sessions without modifying the saved configuration.

**Hot Reload:** Configuration changes can be applied without restarting the application through a dedicated reload command.

**Migration:** Configuration schema changes are handled automatically, preserving user settings while adding new default values.

**Error Handling:** If the configuration file is corrupted or unreadable, the system falls back to defaults and optionally creates a backup of the invalid file.

### Example Configuration File Structure

```json
{
  "version": "1.0",
  "performance": {
    "streaming_threshold_mb": 100,
    "debounce_delay_ms": 150,
    "regex_cache_size": 100,
    "sample_size": 1000,
    "buffer_size_mb": 10
  },
  "ui": {
    "default_context_lines": 3,
    "color_scheme": "default",
    "auto_save_interval_sec": 30,
    "pane_sizes": {
      "filter_pane": 0.3,
      "content_pane": 0.7
    }
  },
  "data_management": {
    "max_concurrent_files": 5,
    "session_history_count": 100,
    "session_retention_days": 180,
    "pattern_history_size": 50,
    "template_storage_path": "~/.config/qf/templates"
  },
  "file_handling": {
    "default_encoding": "UTF-8",
    "max_line_length": 5000,
    "export_default_format": "text",
    "export_default_directory": "~/Downloads"
  }
}
```

**Configuration Value Ranges and Validation:**

- `streaming_threshold_mb`: 1-10000 (integer)
- `debounce_delay_ms`: 50-1000 (integer)
- `regex_cache_size`: 10-1000 (integer)
- `sample_size`: 100-10000 (integer)
- `buffer_size_mb`: 1-100 (integer)
- `default_context_lines`: 0-50 (integer)
- `color_scheme`: predefined list of valid scheme names
- `auto_save_interval_sec`: 10-600 (integer)
- `max_concurrent_files`: 1-20 (integer)
- `session_history_count`: 10-1000 (integer)
- `session_retention_days`: 1-3650 (integer)
- `pattern_history_size`: 10-500 (integer)
- `max_line_length`: 1000-50000 (integer)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide visual interface with separate panes for include and exclude filter patterns
- **FR-002**: System MUST support Vim-like modal interaction with Normal mode and Insert mode
- **FR-003**: System MUST apply include filters using OR logic (any pattern match shows the line)
- **FR-004**: System MUST apply exclude filters as veto system (any pattern match hides the line)
- **FR-005**: System MUST show live preview of filtered results as user builds patterns
- **FR-006**: System MUST support regex pattern validation with real-time syntax checking
- **FR-007**: System MUST provide pattern testing mode with overlay interface
- **FR-008**: System MUST support keyboard navigation with j/k for entries and 'm' for pane expansion
- **FR-009**: System MUST allow users to save and restore named filter sets (sessions)
- **FR-010**: System MUST support opening multiple files in tabs with shared filter application
- **FR-011**: System MUST provide pattern templates and presets for common use cases
- **FR-012**: System MUST support pattern history accessible via keyboard navigation
- **FR-013**: System MUST detect and suggest common patterns from file content
- **FR-014**: System MUST support context display (N lines before/after matches)
- **FR-015**: System MUST provide different highlight colors for different include patterns
- **FR-016**: System MUST support smart jumping between pattern matches with n/N navigation
- **FR-017**: System MUST export filtered results to file or clipboard with formatting options
- **FR-018**: System MUST support piped input from other command-line tools
- **FR-019**: System MUST handle large files with streaming mode and lazy evaluation
- **FR-020**: System MUST provide live file following mode (tail -f equivalent) with filtering
- **FR-021**: System MUST offer help system with context-sensitive assistance and regex reference
- **FR-022**: System MUST export equivalent ripgrep commands for filter sets
- **FR-023**: System MUST auto-save current session and restore on restart
- **FR-024**: System MUST handle files larger than a configurable amount using streaming mode (configured in ~/.config/qf.json, default: 100MB)
- **FR-025**: System MUST support up to a configurable maximum concurrent files (configured in ~/.config/qf.json, default: 5)
- **FR-026**: System MUST retain session history for a configurable count and period (configured in ~/.config/qf.json, defaults: 100 sessions, 180 days)
- **FR-027**: System MUST load configuration from ~/.config/qf.json on startup with automatic fallback to defaults if file is missing
- **FR-028**: System MUST create default configuration file with all settings and their default values if configuration file doesn't exist
- **FR-029**: System MUST validate all configuration values against expected types and ranges, logging warnings for invalid values
- **FR-030**: System MUST provide command-line flags to override configuration values for individual sessions
- **FR-031**: System MUST support configuration hot-reload through dedicated command without requiring application restart
- **FR-032**: System MUST provide command or interface to edit configuration settings interactively
- **FR-033**: System MUST handle configuration schema migration between application versions while preserving user settings
- **FR-034**: System MUST log configuration errors and continue operation with default values instead of crashing
- **FR-035**: System MUST provide command to reset configuration to factory defaults with user confirmation

### Key Entities *(include if feature involves data)*

- **Filter Set**: Collection of include and exclude patterns that can be saved, named, and reloaded as a session
- **Pattern**: Individual regex expression with metadata including type (include/exclude), match statistics, and validation status
- **Session**: Named workspace containing filter set, open files, and UI state that persists across application restarts
- **File Tab**: Representation of an open log file with its own view state but sharing active filter set
- **Match Result**: Individual line from log file with highlighting information and pattern match associations
- **Template**: Predefined pattern with name and description for common filtering scenarios
- **Context View**: Display configuration for showing N lines before/after matches with specific patterns
- **Configuration**: User preferences and system settings stored in JSON format at ~/.config/qf.json, including performance thresholds, UI preferences, data management options, and file handling settings with automatic validation and migration support

---

## Review & Acceptance Checklist

*GATE: Automated checks run during main() execution*

### Content Quality

- [ ] No implementation details (languages, frameworks, APIs)
- [ ] Focused on user value and business needs
- [ ] Written for non-technical stakeholders
- [ ] All mandatory sections completed

### Requirement Completeness

- [ ] No [NEEDS CLARIFICATION] markers remain
- [ ] Requirements are testable and unambiguous
- [ ] Success criteria are measurable
- [ ] Scope is clearly bounded
- [ ] Dependencies and assumptions identified

### Configuration Quality

- [ ] Configuration options are user-friendly and well-documented
- [ ] Default values are reasonable for typical use cases
- [ ] Configuration schema is extensible for future requirements
- [ ] Configuration migration path is clearly defined
- [ ] All configurable values have appropriate validation ranges
- [ ] Configuration errors are handled gracefully without application crashes

---

## Execution Status

*Updated by main() during processing*

- [x] User description parsed
- [x] Key concepts extracted
- [x] Ambiguities marked
- [x] User scenarios defined
- [x] Requirements generated
- [x] Entities identified
- [ ] Review checklist passed

---
