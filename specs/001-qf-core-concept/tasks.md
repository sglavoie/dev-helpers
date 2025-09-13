# Tasks: Interactive Log Filter Composer (qf)

**Input**: Design documents from `/specs/001-qf-core-concept/`
**Prerequisites**: research.md, data-model.md, contracts/, quickstart.md

## Execution Flow (main)

```
1. Load research.md, data-model.md, contracts/, quickstart.md
   → Extract: Go 1.25+, Bubble Tea v1.3.9, Lipgloss v1.1.0, Bubbles v0.21.0
   → Extract entities: Pattern, FilterSet, FileTab, Session, Config
   → Extract contracts: FileReader, FilterEngine, UI Messages
   → Extract user stories: Basic filtering, multi-file analysis, large file handling
2. Generate tasks by category:
   → Setup: Go project init, Bubble Tea dependencies, linting setup
   → Tests: 3 contract tests, 4 integration tests from quickstart scenarios
   → Core: 5 models, 3 services, Bubble Tea components
   → Integration: TUI layout, session management, config hot-reload
   → Polish: unit tests, performance optimization, documentation
3. Apply task rules:
   → Different files = mark [P] for parallel execution
   → Same file = sequential (no [P] marking)
   → Tests before implementation (TDD approach)
4. Number tasks sequentially (T001, T002...)
5. Generate dependency graph and parallel execution guidance
```

## Format: `[ID] [P?] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Path Conventions

- **Go project structure**: `cmd/qf/`, `internal/`, `tests/`
- Paths shown assume single project at repository root

## Phase 3.1: Setup

- [ ] T001 Create Go project structure with cmd/qf/ and internal/ directories
- [ ] T002 Initialize Go module and install Bubble Tea dependencies (v1.3.9, Lipgloss v1.1.0, Bubbles v0.21.0)
- [ ] T003 Configure linting tools (golangci-lint) and formatting (gofmt)

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.3

**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**

- [ ] T004 [P] Contract test for FileReader interface in tests/contract/file_operations_test.go
    - **Test Requirements:** ReadFile returns channel of Line structs, handles files >100MB with streaming mode, context cancellation stops reading, returns error for non-existent files, supports UTF-8 and handles encoding errors
    - **Must assert:** Channel closes on completion, line numbers sequential

- [ ] T005 [P] Contract test for FilterEngine interface in tests/contract/filtering_engine_test.go
    - **Test Requirements:** Include patterns use OR logic, exclude patterns use veto logic, empty includes = show all (minus excludes), pattern compilation caching works, invalid regex returns validation error
    - **Must assert:** Performance <150ms for 10K lines

- [ ] T006 [P] Contract test for UI Message handling in tests/contract/ui_messages_test.go
    - **Test Requirements:** FilterUpdateMsg propagates filter changes, KeyMsg handles Normal/Insert mode transitions, ErrorMsg displays user-friendly errors, FileOpenMsg loads content correctly
    - **Must assert:** Message passing between components

- [ ] T007 [P] Integration test for basic log filtering workflow in tests/integration/basic_filtering_test.go
    - **Scenario:** From quickstart.md - basic filtering with include/exclude patterns

- [ ] T008 [P] Integration test for multi-file analysis in tests/integration/multi_file_test.go
    - **Scenario:** From quickstart.md - tab management and shared filter sets

- [ ] T009 [P] Integration test for large file handling in tests/integration/large_file_test.go
    - **Scenario:** From quickstart.md - streaming mode activation and performance

- [ ] T010 [P] Integration test for configuration management in tests/integration/config_test.go
    - **Scenario:** From quickstart.md - config editing and hot-reload

## Phase 3.3: Core Implementation (ONLY after tests are failing)

- [ ] T011 [P] Pattern model in internal/core/pattern.go
    - Implement Pattern struct with UUID generation
    - Use standard `regexp` package (not regexp2)
    - Include validation method returning bool + error
    - Color field uses hex format validation

- [ ] T012 [P] FilterSet model in internal/core/filter_set.go
    - Include/Exclude pattern collections
    - Validation for unique pattern IDs
    - Name and description fields

- [ ] T013 [P] FileTab model in internal/file/file_tab.go
    - File metadata and view state
    - Display name and path handling
    - Modified flag tracking

- [ ] T014 [P] Session model in internal/session/session.go
    - Complete workspace state
    - FilterSet and file tab management
    - UI state persistence

- [ ] T015 [P] Config model in internal/config/config.go
    - Configuration struct with validation tags
    - Default value initialization
    - JSON marshaling support

- [ ] T016 [P] FileReader implementation in internal/file/reader.go
    - Implement streaming for files >100MB (configurable)
    - Use buffered reader with 64KB chunks
    - Return `chan Line` for async processing
    - Support context cancellation via `select` statement

- [ ] T017 [P] FilterEngine implementation in internal/core/filter.go
    - Implement include OR logic: `if any(include.Match(line)) { show }`
    - Implement exclude veto: `if any(exclude.Match(line)) { hide }`
    - Cache compiled patterns using sync.Map
    - Debounce filter updates (150ms default)

- [ ] T018 [P] PatternManager with LRU cache in internal/core/pattern_manager.go
    - Use container/list for LRU implementation
    - Max 100 patterns (configurable)
    - Thread-safe with sync.RWMutex
    - Track hit/miss statistics
- [ ] T019 Filter pane Bubble Tea component in internal/ui/filter_pane.go
- [ ] T020 Content viewer Bubble Tea component in internal/ui/viewer.go
- [ ] T021 Tab manager Bubble Tea component in internal/ui/tabs.go
- [ ] T022 Status bar Bubble Tea component in internal/ui/statusbar.go
- [ ] T023 Main application Bubble Tea model in internal/ui/app.go
- [ ] T024 CLI command parsing and main function in cmd/qf/main.go

## Phase 3.3.5: Modal Interface

- [ ] T024a Modal state manager in internal/ui/mode.go
    - Implement Normal/Insert mode state machine
    - Handle Escape key for mode exit
    - Track which pane has focus

- [ ] T024b Keyboard shortcut handler in internal/ui/keyboard.go
    - Map keys to commands based on current mode
    - Implement vim-style navigation (j/k, gg/G, n/N)
    - Handle Tab for pane switching
    - **Tab navigation:** Use left/right arrow keys to switch between file tabs when multiple files are open
    - Number keys (1-9) for direct tab access

- [ ] T024c Pattern testing overlay in internal/ui/overlay.go
    - Floating UI component for pattern testing
    - Shows live matches before adding pattern
    - Choose include/exclude on confirmation

## Phase 3.4: Integration (Enhanced with Implementation Details)

- [ ] T025 File streaming and buffer management in internal/file/buffer.go
    - Circular buffer for last 10K lines
    - Load-on-demand for context lines
    - Memory-mapped files for >1GB files
    - Progress indicator for initial load

- [ ] T026 Session persistence and auto-save in internal/session/persistence.go
    - JSON format at `~/.config/qf/sessions/`
    - Atomic writes with temp file + rename
    - Auto-save every 30 seconds (configurable)
    - Keep 3 backup versions

- [ ] T027 Configuration hot-reload and validation in internal/config/manager.go
    - File watcher using fsnotify or polling
    - Validate before applying changes
    - Send ConfigUpdateMsg to all components
    - Log rejected invalid values

- [ ] T028 Pattern highlighting and match display in internal/core/highlighter.go
    - ANSI escape sequences for terminal colors
    - Non-overlapping highlights (priority: exclude > include)
    - Support 256-color and true-color terminals
    - Fallback to basic 8 colors

- [ ] T029 Export functionality (text, ripgrep commands) in internal/export/exporter.go
    - Plain text with optional line numbers
    - Generate ripgrep command: `rg '(pattern1|pattern2)' --invert-match 'exclude'`
    - Clipboard support via clipboard package
    - File export with timestamp suffix

- [ ] T030 Keyboard shortcuts and modal interface handling in internal/ui/input.go
    - Bubble Tea KeyMsg handling
    - Mode-aware command dispatch
    - Help overlay with context-sensitive shortcuts
    - Customizable keybindings via config

## Phase 3.5: Polish (Enhanced with Concrete Targets)

- [ ] T031 [P] Unit tests for pattern validation in tests/unit/pattern_validation_test.go
    - **Target:** 90% coverage

- [ ] T032 [P] Unit tests for filtering logic in tests/unit/filter_logic_test.go
    - **Target:** 95% coverage

- [ ] T033 [P] Unit tests for session management in tests/unit/session_test.go
    - **Target:** 85% coverage

- [ ] T034 [P] Performance benchmarks for large file processing in tests/performance/streaming_test.go
    - **Target:** 100MB file loads in <3 seconds
    - Streaming maintains <100MB memory usage
    - Smooth scrolling at 60fps

- [ ] T035 [P] Performance benchmarks for regex caching in tests/performance/cache_test.go
    - **Target:** >80% cache hit rate
    - Pattern compilation <20ms
    - 100K lines/sec for simple patterns

- [ ] T036 Vim-style keybinding documentation in docs/keybindings.md

- [ ] T037 Code optimization
    - **Specific targets:**
        - Extract duplicate filter logic to shared function
        - Consolidate file I/O error handling
        - Refactor Bubble Tea Update methods >100 lines
        - Profile and optimize hot paths

- [ ] T038 Execute quickstart manual testing scenarios from quickstart.md

## Dependencies

- Tests (T004-T010) before implementation (T011-T024)
- T011 (Pattern) blocks T012 (FilterSet), T017 (FilterEngine)
- T013 (FileTab) blocks T016 (FileReader), T025 (Buffer)
- T014 (Session) blocks T026 (Persistence)
- T015 (Config) blocks T027 (ConfigManager)
- T016-T018 block T019-T023 (UI components need core services)
- T019-T023 block T024 (main function needs complete UI)
- T024 blocks T024a-T024c (modal interface components)
- Implementation (T011-T024c) before integration (T025-T030)
- Integration before polish (T031-T038)

## Parallel Example

```
# Launch T004-T006 together (contract tests):
Task: "Contract test for FileReader interface in tests/contract/file_operations_test.go"
Task: "Contract test for FilterEngine interface in tests/contract/filtering_engine_test.go"
Task: "Contract test for UI Message handling in tests/contract/ui_messages_test.go"

# Launch T007-T010 together (integration tests):
Task: "Integration test for basic log filtering workflow in tests/integration/basic_filtering_test.go"
Task: "Integration test for multi-file analysis in tests/integration/multi_file_test.go"
Task: "Integration test for large file handling in tests/integration/large_file_test.go"
Task: "Integration test for configuration management in tests/integration/config_test.go"

# Launch T011-T015 together (core models):
Task: "Pattern model in internal/core/pattern.go"
Task: "FilterSet model in internal/core/filter_set.go"
Task: "FileTab model in internal/file/file_tab.go"
Task: "Session model in internal/session/session.go"
Task: "Config model in internal/config/config.go"
```

## Notes

- [P] tasks = different files, no dependencies
- Verify tests fail before implementing (TDD approach)
- Commit after each task completion
- Avoid: vague tasks, same file conflicts
- Follow Go project structure conventions
- Use Bubble Tea message passing patterns throughout

## Task Generation Rules Applied

1. **From Contracts**:
   - file_operations.md → T004 (FileReader contract test) → T016 (FileReader impl)
   - filtering_engine.md → T005 (FilterEngine contract test) → T017 (FilterEngine impl)
   - ui_messages.md → T006 (UI Messages contract test) → T019-T024 (UI components)

2. **From Data Model**:
   - Pattern entity → T011 (Pattern model)
   - FilterSet entity → T012 (FilterSet model)
   - FileTab entity → T013 (FileTab model)
   - Session entity → T014 (Session model)
   - Config entity → T015 (Config model)

3. **From Quickstart Scenarios**:
   - Basic log filtering → T007 (integration test)
   - Multi-file analysis → T008 (integration test)
   - Large file handling → T009 (integration test)
   - Configuration management → T010 (integration test)

4. **Ordering Applied**:
   - Setup (T001-T003) → Tests (T004-T010) → Models (T011-T015) → Services (T016-T018) → UI Components (T019-T023) → Main (T024) → Integration (T025-T030) → Polish (T031-T038)

## Validation Checklist

*Checked before task execution*

- [x] All contracts have corresponding tests (T004-T006)
- [x] All entities have model tasks (T011-T015)
- [x] All tests come before implementation (T004-T010 before T011-T024)
- [x] Parallel tasks are truly independent (different files, marked [P])
- [x] Each task specifies exact file path
- [x] No task modifies same file as another [P] task
- [x] Quickstart scenarios covered by integration tests
- [x] Performance requirements addressed in T034-T035
- [x] TDD approach enforced (tests must fail before implementation)

## Technical Specifications

### Bubble Tea Message Types

```go
type FilterUpdateMsg struct{ FilterSet FilterSet }
type FileOpenMsg struct{ Path string; Content []Line }
type ErrorMsg struct{ Error error; Recoverable bool }
type ModeChangeMsg struct{ Mode string }
type ConfigUpdateMsg struct{ Config Config }
```

### Component Communication

- App.go orchestrates all components
- Components return commands that emit messages
- Use tea.Batch for multiple simultaneous commands
- Components don't directly reference each other

### Error Handling Strategy

- User errors: Display in status bar with ErrorMsg
- System errors: Log to stderr, fallback gracefully
- Validation errors: Show inline with red highlighting
- File errors: Offer retry or skip options

### Success Criteria

- All tests pass before implementation
- Performance targets met
- No race conditions (test with -race)
- Memory usage within configured limits
- Keyboard navigation fully functional
- Configuration hot-reload works

---
*Tasks enhanced: 2025-09-14*
