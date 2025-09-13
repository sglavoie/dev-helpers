# Quickstart Guide - Interactive Log Filter Composer (qf)

**Date**: 2025-09-13
**Purpose**: User interaction flows and validation scenarios for implementation testing

## Installation & First Run

### Quick Installation

```bash
# Install from source (development)
git clone https://github.com/user/qf.git
cd qf
go build -o qf ./cmd/qf
./qf --help

# First run - creates default configuration
./qf sample.log
```

### Expected First Run Behavior

1. Creates `~/.config/qf.json` with default settings
2. Opens sample.log in content viewer
3. Shows empty include/exclude panes
4. Displays help hint in status bar: "Press 'h' for help, 'i' to add patterns"

## Core User Workflows

### Workflow 1: Basic Log Filtering

**Scenario**: Developer wants to find error messages in application.log

**Steps**:

1. **Launch**: `qf application.log`
   - **Expected**: File loads, shows all lines, cursor in content viewer
   - **Validation**: Status bar shows file info (lines, size)

2. **Add Include Pattern**: Press `Tab` to focus include pane, then `i`
   - **Expected**: Enters Insert mode, cursor in include pane
   - **Validation**: Mode indicator shows "INSERT", pane highlighted

3. **Enter Pattern**: Type `ERROR`
   - **Expected**: Real-time validation, no syntax errors
   - **Validation**: Pattern shows green (valid) indicator

4. **Apply Filter**: Press `Escape` to exit Insert mode
   - **Expected**: Filter applies immediately, only ERROR lines visible
   - **Validation**: Status bar shows match count, content updates

5. **Add Exclude Pattern**: Press `Tab` twice to focus exclude pane, press `i`
   - **Expected**: Focus moves to exclude pane, Insert mode active

6. **Enter Exclusion**: Type `connection timeout`
   - **Expected**: Pattern validates, filter updates on Escape
   - **Validation**: ERROR lines with "connection timeout" hidden

**Success Criteria**:

- ✅ Modal interface works correctly (Normal/Insert modes)
- ✅ Real-time filtering with live preview
- ✅ Include/exclude logic functions properly
- ✅ Status updates reflect current state

### Workflow 2: Multi-File Analysis

**Scenario**: System administrator comparing logs from multiple servers

**Steps**:

1. **Open Multiple Files**:

   ```bash
   qf server1.log server2.log server3.log
   ```

   - **Expected**: Tab bar appears, first file active
   - **Validation**: 3 tabs visible, active tab highlighted

2. **Create Filter Set**: Add include pattern `CRITICAL`
   - **Expected**: Filter applies to current tab only
   - **Validation**: Only current tab content filtered

3. **Switch Tabs**: Press `2` to switch to second tab
   - **Expected**: Tab 2 becomes active, same filter applied
   - **Validation**: Filter set shared across all tabs

4. **Save Session**: Press `Ctrl+S`, type session name "critical-analysis"
   - **Expected**: Session saves successfully
   - **Validation**: Status confirms save, session stored to disk

5. **Close and Restore**: Exit qf, then `qf --session critical-analysis`
   - **Expected**: All tabs restore with filter set intact
   - **Validation**: Identical state to before exit

**Success Criteria**:

- ✅ Multi-tab interface functions correctly
- ✅ Filter sets apply consistently across tabs
- ✅ Session persistence works reliably
- ✅ Tab navigation responds to keyboard shortcuts

### Workflow 3: Large File Handling

**Scenario**: Developer analyzing 200MB production log file

**Steps**:

1. **Open Large File**: `qf production.log` (200MB file)
   - **Expected**: Streaming mode activates automatically
   - **Validation**: Status shows "Streaming mode", progress indicator

2. **Initial Display**: File content appears within 2 seconds
   - **Expected**: First 1000 lines displayed, more available on scroll
   - **Validation**: Smooth scrolling, responsive interface

3. **Apply Filter**: Add include pattern `Exception`
   - **Expected**: Filtering processes in background
   - **Validation**: Progress bar, non-blocking UI updates

4. **Navigate Matches**: Press `n` to jump to next match
   - **Expected**: Cursor jumps to next Exception line
   - **Validation**: Context lines visible, match highlighted

5. **Memory Usage**: Check memory consumption
   - **Expected**: Memory stays within configured limits (~100MB)
   - **Validation**: No memory growth over time

**Success Criteria**:

- ✅ Streaming mode activates for large files
- ✅ UI remains responsive during processing
- ✅ Memory usage stays within bounds
- ✅ Navigation works with partial loading

### Workflow 4: Configuration Management

**Scenario**: User customizing interface and performance settings

**Steps**:

1. **Open Config**: `qf --config edit`
   - **Expected**: Configuration editor opens (or file opens in $EDITOR)
   - **Validation**: Current configuration displayed with comments

2. **Modify Settings**: Change debounce delay to 100ms, increase cache size
   - **Expected**: Settings validate on save
   - **Validation**: Invalid values rejected with helpful errors

3. **Apply Changes**: Save configuration file
   - **Expected**: Hot-reload applies changes without restart
   - **Validation**: New settings take effect immediately

4. **Test Changes**: Open file and verify faster response time
   - **Expected**: Filter updates reflect new debounce delay
   - **Validation**: Timing matches configured value

**Success Criteria**:

- ✅ Configuration editing workflow works
- ✅ Validation prevents invalid configurations
- ✅ Hot-reload applies changes without restart
- ✅ Settings affect application behavior correctly

## Keyboard Navigation Reference

### Normal Mode Shortcuts

```
Movement:
  j/k         - Navigate pattern lists
  Ctrl+d/u    - Scroll content viewer half-page
  gg/G        - Jump to beginning/end of content
  n/N         - Next/previous match for focused pattern

Pane Control:
  Tab         - Cycle between panes (include → exclude → viewer)
  m           - Expand/collapse focused pane
  h           - Show/hide help overlay

Pattern Management:
  i           - Enter Insert mode for current pane
  a           - Add new pattern to current pane
  d           - Delete focused pattern
  y           - Copy pattern to clipboard

File Operations:
  o           - Open new file
  w           - Close current tab
  1-9         - Switch to tab by number
  Ctrl+s      - Save current session

Application:
  :           - Command mode (save, load, quit)
  q           - Quit application
  ?           - Show keyboard shortcuts
```

### Insert Mode Shortcuts

```
Text Editing:
  Escape      - Return to Normal mode
  Ctrl+a/e    - Beginning/end of line
  Ctrl+w      - Delete word backward
  Tab         - Pattern completion (future feature)

Navigation:
  Up/Down     - Pattern history
  Enter       - Apply pattern and return to Normal mode
```

## Integration Test Scenarios

### Scenario 1: End-to-End Filtering Workflow

```go
func TestCompleteFilteringWorkflow(t *testing.T) {
    // Setup test file with known content
    testFile := createTestFile([]string{
        "INFO: Starting application",
        "ERROR: Database connection failed",
        "ERROR: Connection timeout occurred",
        "WARN: Retrying connection",
        "INFO: Connection established",
    })

    // Launch application with test file
    app := newTestApp(testFile)

    // Add include pattern
    app.SendKey(tea.KeyMsg{Type: tea.KeyTab})  // Focus include pane
    app.SendKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})  // Insert mode
    app.SendKeySequence("ERROR")
    app.SendKey(tea.KeyMsg{Type: tea.KeyEscape})  // Apply filter

    // Verify filtering result
    visibleLines := app.GetVisibleLines()
    assert.Len(t, visibleLines, 2, "Should show 2 ERROR lines")
    assert.Contains(t, visibleLines[0].Text, "Database connection failed")
    assert.Contains(t, visibleLines[1].Text, "Connection timeout occurred")

    // Add exclude pattern
    app.SendKey(tea.KeyMsg{Type: tea.KeyTab})  // Focus exclude pane
    app.SendKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})  // Insert mode
    app.SendKeySequence("timeout")
    app.SendKey(tea.KeyMsg{Type: tea.KeyEscape})  // Apply filter

    // Verify exclusion applied
    visibleLines = app.GetVisibleLines()
    assert.Len(t, visibleLines, 1, "Should show 1 ERROR line after exclusion")
    assert.Contains(t, visibleLines[0].Text, "Database connection failed")
}
```

### Scenario 2: Configuration Hot-Reload Test

```go
func TestConfigurationHotReload(t *testing.T) {
    app := newTestApp("sample.log")

    // Verify initial debounce delay
    originalDelay := app.GetConfig().Performance.DebounceDelayMs
    assert.Equal(t, 150, originalDelay, "Default debounce should be 150ms")

    // Modify configuration file
    newConfig := app.GetConfig()
    newConfig.Performance.DebounceDelayMs = 50
    saveConfig(newConfig, app.GetConfigPath())

    // Send reload command
    app.SendCommand(":config reload")

    // Verify configuration updated
    reloadedConfig := app.GetConfig()
    assert.Equal(t, 50, reloadedConfig.Performance.DebounceDelayMs)

    // Verify behavior change
    startTime := time.Now()
    app.SendKeySequence("test pattern")
    app.WaitForFilterUpdate()
    elapsed := time.Since(startTime)

    assert.Less(t, elapsed, 100*time.Millisecond, "Filter should apply faster with new config")
}
```

### Scenario 3: Session Persistence Test

```go
func TestSessionPersistence(t *testing.T) {
    // Create initial session
    app := newTestApp([]string{"file1.log", "file2.log"})
    app.AddIncludePattern("ERROR")
    app.AddExcludePattern("timeout")
    app.SwitchTab(1)  // Switch to second tab

    // Save session
    sessionName := "test-session"
    app.SaveSession(sessionName)

    // Verify session file exists
    sessionPath := getSessionPath(sessionName)
    assert.FileExists(t, sessionPath)

    // Close and reopen
    app.Close()
    newApp := newTestApp()
    newApp.LoadSession(sessionName)

    // Verify state restored
    assert.Equal(t, 2, newApp.GetTabCount())
    assert.Equal(t, 1, newApp.GetActiveTabIndex())

    filterSet := newApp.GetFilterSet()
    assert.Len(t, filterSet.Include, 1)
    assert.Len(t, filterSet.Exclude, 1)
    assert.Equal(t, "ERROR", filterSet.Include[0].Expression)
    assert.Equal(t, "timeout", filterSet.Exclude[0].Expression)
}
```

## Performance Validation

### Responsiveness Tests

- **Keystroke Response**: <50ms from key press to UI update
- **Filter Application**: <150ms for simple patterns on <10K lines
- **Tab Switching**: <100ms to switch between tabs
- **File Loading**: <1s for files under 10MB

### Memory Usage Tests

- **Base Application**: <15MB without loaded files
- **Small File (1MB)**: <25MB total memory usage
- **Large File (100MB)**: <100MB total memory usage (streaming)
- **Multiple Files**: <50MB for 5 concurrent files

### Throughput Tests

- **Pattern Compilation**: >50 patterns/second compilation rate
- **Line Processing**: >100K lines/second for simple patterns
- **File Reading**: >1M lines/second from fast storage

## Error Handling Validation

### Configuration Errors

```bash
# Test invalid configuration
echo '{"invalid": "json"' > ~/.config/qf.json
qf sample.log
# Expected: Warning message, fallback to defaults, backup created
```

### File Access Errors

```bash
# Test permission denied
chmod 000 restricted.log
qf restricted.log
# Expected: Clear error message, graceful degradation
```

### Pattern Validation

```bash
# Test invalid regex
qf sample.log
# Enter pattern: "("
# Expected: Real-time error indicator, helpful error message
```

## Success Criteria Summary

The implementation is considered successful when:

1. ✅ **All workflows execute without errors**
2. ✅ **Performance targets are met consistently**
3. ✅ **Error conditions are handled gracefully**
4. ✅ **Keyboard shortcuts work as documented**
5. ✅ **Configuration changes apply correctly**
6. ✅ **Session persistence is reliable**
7. ✅ **UI remains responsive under load**
8. ✅ **Memory usage stays within bounds**

---
*Quickstart Guide complete: 2025-09-13*
