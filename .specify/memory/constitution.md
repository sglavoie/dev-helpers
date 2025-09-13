# qf (Query File) Constitution

## Core Principles

### I. Filter-First Architecture

Every filtering operation in qf MUST begin as a declarative filter specification. Filter specifications are the source of truth—the UI serves the specification, not the reverse. All filter operations must be:

- **Serializable**: Filter configurations can be saved, loaded, and shared as text/JSON
- **Versionable**: Filter specs can be tracked in version control
- **Composable**: Include and exclude patterns work together predictably
- **Auditable**: Filter logic is explicit and inspectable

### II. Modal Interface Discipline (NON-NEGOTIABLE)

qf MUST maintain strict modal behavior following Vim conventions:

- **Normal Mode**: Navigation, commands, and pane management (default state)
- **Insert Mode**: Text editing within filter panes only
- **Mode Transitions**: 'i' enters insert, Escape returns to normal
- **Visual Feedback**: Current mode and focus must be unambiguously displayed
- **Vim Keybindings**: j/k navigation, Ctrl+d/u scrolling, gg/G jumping enforced throughout

### III. Component Modularity

Each functional area MUST be an independent, testable component:

- **Pane Independence**: Include, exclude, results, and help panes are self-contained
- **Clear Interfaces**: Components communicate through well-defined message passing
- **Testable Isolation**: Each component can be tested without dependencies
- **State Management**: Component state is predictable and debuggable

### IV. Real-Time Feedback Integrity

Filter updates MUST provide immediate, accurate feedback:

- **Instant Updates**: Filter changes apply immediately when validation occurs
- **Performance Transparency**: Show filtering progress for large files
- **Error Visibility**: All regex errors, file access issues displayed prominently
- **Graceful Degradation**: Maintain responsiveness with large datasets

### V. Text-Stream Protocol

qf MUST support text-based operation for automation and integration:

- **CLI Compatibility**: Filter specifications exportable as command-line equivalent
- **Stream Processing**: Support stdin/stdout for pipeline integration
- **File I/O**: Read from files, write filtered results to files
- **Configuration Export**: Save/load filter sets as portable text formats

### VI. Accessibility and Observability

The interface MUST be fully accessible and transparent:

- **Keyboard-Only Navigation**: No mouse dependency for any functionality
- **Visual State Indicators**: Current focus, mode, and operations clearly displayed
- **Help System**: Context-sensitive help accessible via 'h' key
- **Status Feedback**: File processing status, match counts, error states visible

### VII. Performance and Scalability

qf MUST handle files of varying sizes efficiently:

- **Lazy Evaluation**: Process only visible portions of large files initially
- **Streaming Support**: Handle real-time log streams without memory accumulation
- **Configurable Limits**: User-adjustable memory and processing limits
- **Background Processing**: Large operations don't block UI responsiveness

## TUI Implementation Standards

### Terminal Compatibility

- **Cross-Platform**: Support Unix, Linux, macOS, Windows terminals
- **Responsive Layout**: Adapt gracefully to different terminal sizes (minimum 80x24)
- **Color Scheme**: Accessible colors with fallback for limited palettes
- **Character Set**: Use only portable character sets for borders and indicators

### User Experience Requirements

- **Immediate Feedback**: All user actions produce visible response within 100ms
- **Predictable Navigation**: Tab cycles panes, standard Vim keys for movement
- **Error Recovery**: Clear error messages with suggested corrections
- **Session Persistence**: Remember last used filters and pane layout

### Technology Stack Constraints

- **Go + Bubbletea**: Primary framework for TUI development
- **Standard Library**: Prefer standard library over external dependencies
- **Regex Engine**: Use Go's standard regexp package for consistency
- **File Handling**: Support for large files through streaming/chunking

## Development Workflow

### Test-Driven Development (NON-NEGOTIABLE)

All qf development MUST follow strict TDD:

1. **Component Tests First**: Write tests for UI components before implementation
2. **Filter Logic Tests**: Comprehensive regex and filtering logic tests
3. **Integration Tests**: Full TUI workflows with mock file systems
4. **User Acceptance**: Manual testing with real log files and edge cases

### Quality Gates

Before any feature is considered complete:

- [ ] All component tests pass
- [ ] Filter specifications are exportable/importable
- [ ] Vim keybindings work consistently across all modes
- [ ] Performance acceptable with 100MB+ files
- [ ] Help system covers all functionality
- [ ] Cross-platform terminal compatibility verified

### Documentation Requirements

- **Filter Specification Format**: Document filter config schema
- **Keybinding Reference**: Complete key mapping documentation
- **Architecture Overview**: Component interaction diagrams
- **Performance Characteristics**: Documented limits and optimizations

## Governance

This constitution supersedes all other development practices for qf. All code reviews, feature implementations, and architectural decisions must verify compliance with these principles.

**Complexity Justification**: Any deviation from these principles requires explicit documentation of rationale and approval from project maintainers.

**Amendment Process**: Constitution changes require documented rationale, backwards compatibility assessment, and validation that changes align with SDD methodology.

**Version**: 1.0.0 | **Ratified**: 2025-09-13 | **Last Amended**: 2025-09-13
