# qf Constitution Update Checklist

When amending the qf constitution (`/memory/constitution.md`), ensure all dependent documents are updated to maintain consistency with TUI development principles.

## Templates to Update

### When adding/modifying ANY article

- [ ] `/templates/plan-template.md` - Update Constitution Check section
- [ ] `/templates/spec-template.md` - Update if requirements/scope affected
- [ ] `/templates/tasks-template.md` - Update if new task types needed
- [ ] `/.claude/commands/plan.md` - Update if planning process changes
- [ ] `/.claude/commands/tasks.md` - Update if task generation affected
- [ ] `/CLAUDE.md` - Update runtime development guidelines

### qf-Specific Article Updates

#### Article I (Filter-First Architecture)

- [ ] Ensure templates emphasize filter specification creation
- [ ] Update examples to show serializable filter configs
- [ ] Add filter export/import requirements to templates
- [ ] Include filter composition validation

#### Article II (Modal Interface Discipline)

- [ ] Update TUI component templates with modal behavior requirements
- [ ] Add Vim keybinding consistency checks
- [ ] Include mode transition validation
- [ ] Update UI state management requirements

#### Article III (Component Modularity)

- [ ] Emphasize pane independence in component designs
- [ ] Update message passing interface requirements
- [ ] Add component isolation testing requirements
- [ ] Include state management clarity checks

#### Article IV (Real-Time Feedback Integrity)

- [ ] Add immediate update requirements to UI templates
- [ ] Include performance monitoring for large files
- [ ] Update error handling and display requirements
- [ ] Add responsive UI validation steps

#### Article V (Text-Stream Protocol)

- [ ] Update CLI compatibility requirements
- [ ] Add stdin/stdout pipeline support checks
- [ ] Include configuration export/import validation
- [ ] Update automation integration requirements

#### Article VI (Accessibility and Observability)

- [ ] Add keyboard-only navigation requirements
- [ ] Include visual state indicator checks
- [ ] Update help system coverage requirements
- [ ] Add status feedback validation

#### Article VII (Performance and Scalability)

- [ ] Include lazy evaluation implementation checks
- [ ] Add streaming support requirements
- [ ] Update memory limit configuration validation
- [ ] Include background processing requirements

## TUI-Specific Validation Steps

1. **Before committing constitution changes:**
   - [ ] All templates reference TUI-specific requirements
   - [ ] Terminal compatibility examples updated
   - [ ] Modal behavior consistently enforced
   - [ ] Performance requirements clearly stated

2. **After updating templates:**
   - [ ] Run through sample TUI component implementation
   - [ ] Verify filter specification format compliance
   - [ ] Test keybinding consistency across templates
   - [ ] Validate component modularity requirements

3. **TUI Quality Gates:**
   - [ ] Modal interface discipline verified
   - [ ] Vim keybinding consistency validated
   - [ ] Component independence tested
   - [ ] Performance characteristics documented
   - [ ] Terminal compatibility confirmed

4. **Version tracking:**
   - [ ] Update constitution version number
   - [ ] Note version in template footers
   - [ ] Add amendment to constitution history
   - [ ] Update TUI framework compatibility notes

## qf-Specific Common Misses

Watch for these TUI-specific often-forgotten updates:

- Keybinding documentation consistency
- Modal behavior enforcement across components
- Filter specification format updates
- Terminal compatibility requirements
- Component state management patterns
- Performance characteristics for large files
- Help system coverage for new features

## TUI Development Considerations

### When updating component-related articles

- [ ] Update component testing requirements
- [ ] Include UI state validation
- [ ] Add modal behavior consistency checks
- [ ] Update keybinding documentation

### When updating filter-related articles

- [ ] Update filter specification schema
- [ ] Include regex validation requirements
- [ ] Add filter composition logic
- [ ] Update export/import format docs

### When updating performance articles

- [ ] Update file size handling requirements
- [ ] Include memory usage guidelines
- [ ] Add streaming processing requirements
- [ ] Update responsiveness benchmarks

## Template Sync Status

Last sync check: 2025-09-13

- Constitution version: 1.0.0
- Templates aligned: ✅ (initial qf constitution created)
- TUI requirements: ✅ (aligned with qf-specific needs)

---

*This checklist ensures the qf constitution's TUI-specific principles are consistently applied across all project documentation and development workflows.*
