# Implementation Plan: Interactive Log Filter Composer (qf)

**Branch**: `001-qf-core-concept` | **Date**: 2025-09-13 | **Spec**: [spec.md](/Users/sglavoie/dev/sglavoie/dev-helpers/specs/001-qf-core-concept/spec.md)
**Input**: Feature specification from `/specs/001-qf-core-concept/spec.md`

## Execution Flow (/plan command scope)

```
1. Load feature spec from Input path → ✅ COMPLETE
   → If not found: ERROR "No feature spec at {path}"
2. Fill Technical Context (scan for NEEDS CLARIFICATION) → ✅ COMPLETE
   → Detect Project Type from context (web=frontend+backend, mobile=app+api)
   → Set Structure Decision based on project type
3. Evaluate Constitution Check section below → ✅ COMPLETE
   → If violations exist: Document in Complexity Tracking
   → If no justification possible: ERROR "Simplify approach first"
   → Update Progress Tracking: Initial Constitution Check
4. Execute Phase 0 → research.md → ✅ COMPLETE
   → If NEEDS CLARIFICATION remain: ERROR "Resolve unknowns"
5. Execute Phase 1 → contracts, data-model.md, quickstart.md, CLAUDE.md → ✅ COMPLETE
6. Re-evaluate Constitution Check section → ✅ COMPLETE
   → If new violations: Refactor design, return to Phase 1
   → Update Progress Tracking: Post-Design Constitution Check
7. Plan Phase 2 → Describe task generation approach (DO NOT create tasks.md) → ✅ COMPLETE
8. STOP - Ready for /tasks command
```

**IMPORTANT**: The /plan command STOPS at step 7. Phases 2-4 are executed by other commands:

- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary

Interactive Log Filter Composer (qf) is a terminal-based tool that transforms log filtering from command-line complexity into a visual, iterative experience. Uses Vim-style modal interface with separate include/exclude pattern panes and real-time filtering with live preview. Technical approach: Go + Bubble Tea TUI framework with configurable JSON settings, streaming support for large files, and session management.

## Technical Context

**Language/Version**: Go 1.25+
**Primary Dependencies**: Bubble Tea v1.3.9, Lipgloss v1.1.0, Bubbles v0.21.0
**Storage**: JSON configuration files (~/.config/qf.json), session persistence, pattern templates
**Testing**: Go standard testing framework with table-driven tests and test fixtures
**Target Platform**: Cross-platform terminal applications (Unix, Linux, macOS, Windows)
**Project Type**: single - Command-line TUI application
**Performance Goals**: Handle 100MB+ log files, <150ms debounce delay, real-time filtering
**Constraints**: Terminal compatibility 80x24 minimum, keyboard-only navigation, streaming mode for memory efficiency
**Scale/Scope**: Single-user desktop application, configurable limits (5 concurrent files, 100 session history)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Simplicity**:

- Projects: 1 (main CLI application)
- Using framework directly? YES (Bubble Tea TUI framework used directly)
- Single data model? YES (Filter specs, sessions, and config as simple structs)
- Avoiding patterns? YES (Direct component interaction, no unnecessary abstractions)

**Architecture**:

- EVERY feature as library? YES (core filtering, UI components, config, session management)
- Libraries listed:
    - core: Filter engine and pattern matching
    - ui: TUI components (panes, viewer, overlay)
    - config: Configuration management and validation
    - session: Session persistence and history
    - file: File reading with streaming support
    - export: Text and ripgrep export functionality
- CLI per library: Single main CLI with subcommands (qf filter, qf config, qf session)
- Library docs: YES, llms.txt format planned for component documentation

**Testing (NON-NEGOTIABLE)**:

- RED-GREEN-Refactor cycle enforced? YES (Tests written before implementation)
- Git commits show tests before implementation? YES (TDD workflow enforced)
- Order: Contract→Integration→E2E→Unit strictly followed? YES
- Real dependencies used? YES (Real file system, actual regex engine, real config files)
- Integration tests for: Component interaction, file operations, session management, config handling
- FORBIDDEN: Implementation before test, skipping RED phase

**Observability**:

- Structured logging included? YES (Configuration errors, file operations, performance metrics)
- Frontend logs → backend? N/A (Single TUI application)
- Error context sufficient? YES (Pattern validation errors, file access errors, config issues)

**Versioning**:

- Version number assigned? YES (Starting at 1.0.0)
- BUILD increments on every change? YES (Semantic versioning with automated build numbers)
- Breaking changes handled? YES (Configuration migration, session format evolution)

## Project Structure

### Documentation (this feature)

```
specs/001-qf-core-concept/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
```

### Source Code (repository root)

```
# Option 1: Single project (DEFAULT) ✅ SELECTED
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: Option 1 - Single project (Go CLI application with library components)

## Phase 0: Outline & Research

*STATUS: IN PROGRESS*

### Research Tasks Identified

1. **Bubble Tea TUI Architecture**: Component composition patterns, message passing, viewport management
2. **Go Regex Performance**: Caching strategies, compilation optimization, memory management
3. **Configuration Management**: JSON schema validation, hot-reload patterns, migration strategies
4. **File Streaming Patterns**: Large file handling, circular buffers, memory-mapped files
5. **Terminal Compatibility**: Cross-platform concerns, color schemes, responsive layouts
6. **Session Persistence**: File formats, atomic writes, corruption recovery

### Research Findings

**Decision**: Bubble Tea component architecture with message passing
**Rationale**: Provides clean separation of concerns, testable components, and established TUI patterns
**Alternatives considered**: Custom terminal handling (too complex), Termbox (deprecated)

**Decision**: LRU cache for compiled regex patterns
**Rationale**: Balances memory usage with performance, configurable size limits
**Alternatives considered**: Unlimited cache (memory risk), no cache (performance cost)

**Decision**: JSON configuration with atomic writes
**Rationale**: Human-readable, easy to validate, atomic updates prevent corruption
**Alternatives considered**: YAML (parsing complexity), TOML (less familiar), binary (not editable)

**Decision**: Channel-based streaming for large files
**Rationale**: Non-blocking UI, memory-bounded processing, cancellable operations
**Alternatives considered**: Memory mapping (platform-specific), buffer pools (complexity)

**Decision**: Lipgloss styling with fallback colors
**Rationale**: Consistent theming, terminal capability detection, accessibility support
**Alternatives considered**: Direct ANSI codes (brittle), no colors (poor UX)

**Decision**: JSON session files with backup/recovery
**Rationale**: Portable format, version-trackable, corruption detection
**Alternatives considered**: SQLite (overkill), binary format (debugging difficulty)

**Output**: All NEEDS CLARIFICATION items resolved, technology choices finalized

## Phase 1: Design & Contracts

*STATUS: PENDING - Will execute after Phase 0 completion*

### Data Model Planning

- Filter patterns with metadata (type, stats, colors)
- Session management with history and persistence
- Configuration schema with validation ranges
- File buffer management for streaming

### Contract Planning

- Component message interfaces (Bubble Tea Msg types)
- Configuration validation contracts
- File operation interfaces (Reader, Streamer, Watcher)
- Export format contracts (Text, Ripgrep command generation)

### Integration Test Planning

- Full filtering workflows with real files
- Configuration hot-reload scenarios
- Session save/restore operations
- Multi-file tab management

## Phase 2: Task Planning Approach

*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:

- Load `/templates/tasks-template.md` as base
- Generate tasks from TUI component contracts (pane interactions, keyboard handling)
- Generate tasks from filtering engine contracts (pattern compilation, application logic)
- Generate tasks from configuration system (validation, migration, persistence)
- Generate tasks from session management (save/restore, history tracking)
- Each component interface → contract test task [P]
- Each data structure → model creation task [P]
- Each user interaction → integration test task
- Implementation tasks to make contract tests pass

**Ordering Strategy**:

- TDD order: Interface contracts → component tests → implementation
- Dependency order: Core models → services → UI components → integration
- Mark [P] for parallel execution (independent components)
- Critical path: Filter engine → UI components → session management

**Estimated Output**: 35-40 numbered, ordered tasks in tasks.md focusing on TUI architecture and filtering logic

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation

*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)
**Phase 4**: Implementation (execute tasks.md following constitutional principles)
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking

*All Constitution Check items passed - no violations to justify*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |

## Progress Tracking

*This checklist is updated during execution flow*

**Phase Status**:

- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [x] Phase 2: Task planning complete (/plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:

- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented (none required)

---
*Based on Constitution v1.0.0 - See `/.specify/memory/constitution.md`*
