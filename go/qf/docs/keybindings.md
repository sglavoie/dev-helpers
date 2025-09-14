# qf Keybinding Reference

**qf** uses a Vim-style modal interface with three distinct modes: Normal, Insert, and Command. This guide provides a comprehensive reference for all keyboard shortcuts and navigation patterns.

## Table of Contents

- [Mode Overview](#mode-overview)
- [Global Shortcuts](#global-shortcuts)
- [Normal Mode](#normal-mode)
- [Insert Mode](#insert-mode)
- [Command Mode](#command-mode)
- [Component-Specific Shortcuts](#component-specific-shortcuts)
- [Multi-Key Sequences](#multi-key-sequences)
- [Quick Reference Tables](#quick-reference-tables)
- [Customization](#customization)

## Mode Overview

qf operates in three distinct modes, following Vim conventions:

### Normal Mode (Default)
- **Purpose**: Navigation, file operations, and mode transitions
- **Indicator**: Status bar shows `NORMAL`
- **Entry**: Press `Esc` from any other mode
- **Characteristics**: Key presses trigger actions, no text input

### Insert Mode
- **Purpose**: Text editing in filter panes
- **Indicator**: Status bar shows `INSERT`
- **Entry**: Press `i` when focused on a filter pane
- **Characteristics**: Key presses input text, limited navigation

### Command Mode
- **Purpose**: Advanced operations and configuration
- **Indicator**: Status bar shows `COMMAND`
- **Entry**: Press `:` from Normal mode
- **Characteristics**: Command-line interface for complex operations

## Global Shortcuts

These shortcuts work in any mode and across all components:

| Key | Action | Description |
|-----|--------|-------------|
| `Esc` | Exit to Normal | Always returns to Normal mode |
| `Ctrl+C` | Quit | Immediately quit application |
| `Ctrl+S` | Save Session | Save current filter session |
| `?` | Show Help | Display keyboard shortcuts overlay |

## Normal Mode

Normal mode is the default mode for navigation and operations.

### Navigation

| Key | Action | Description |
|-----|--------|-------------|
| `j` | Move Down | Move cursor/selection down one line |
| `k` | Move Up | Move cursor/selection up one line |
| `h` | Move Left | Move cursor/selection left (context-dependent) |
| `l` | Move Right | Move cursor/selection right (context-dependent) |
| `Ctrl+D` | Page Down | Move down half a screen |
| `Ctrl+U` | Page Up | Move up half a screen |
| `gg` | Go to Top | Jump to first line/item |
| `G` | Go to Bottom | Jump to last line/item |

### Pane Management

| Key | Action | Description |
|-----|--------|-------------|
| `Tab` | Cycle Focus | Move focus to next component |
| `Shift+Tab` | Reverse Focus | Move focus to previous component |
| `m` | Toggle Pane | Expand/collapse current pane |

### Mode Transitions

| Key | Action | Description |
|-----|--------|-------------|
| `i` | Enter Insert | Switch to Insert mode (filter panes only) |
| `:` | Enter Command | Switch to Command mode |
| `Esc` | Stay Normal | Confirm Normal mode (no-op) |

### File Operations

| Key | Action | Description |
|-----|--------|-------------|
| `o` | Open File | Open file dialog |
| `w` | Close Tab | Close current file tab |
| `1`-`9` | Switch Tab | Jump to tab by number |

### Search Operations (Viewer only)

| Key | Action | Description |
|-----|--------|-------------|
| `n` | Next Match | Jump to next search result |
| `N` | Previous Match | Jump to previous search result |
| `/` | Start Search | Begin text search in viewer |

### Pattern Management (Filter Panes only)

| Key | Action | Description |
|-----|--------|-------------|
| `a` | Add Pattern | Create new filter pattern |
| `d` | Delete Pattern | Remove selected pattern |
| `y` | Copy Pattern | Copy pattern to clipboard |
| `p` | Paste Pattern | Paste pattern from clipboard |

### Application Control

| Key | Action | Description |
|-----|--------|-------------|
| `q` | Quit | Exit application (with confirmation) |
| `h` | Show Help | Display help overlay |
| `?` | Show Shortcuts | Display this keybinding reference |

## Insert Mode

Insert mode is active when editing filter patterns.

### Text Editing

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | Apply Pattern | Apply current pattern and return to Normal |
| `Backspace` | Delete Char | Delete character before cursor |
| `Delete` | Delete Forward | Delete character at cursor |
| `Ctrl+W` | Delete Word | Delete word before cursor |
| `Ctrl+U` | Clear Line | Clear entire input line |

### Cursor Movement

| Key | Action | Description |
|-----|--------|-------------|
| `Ctrl+A` | Line Start | Move cursor to beginning of line |
| `Ctrl+E` | Line End | Move cursor to end of line |
| `Left` | Char Left | Move cursor one character left |
| `Right` | Char Right | Move cursor one character right |
| `Ctrl+B` | Char Left | Alternative to Left arrow |
| `Ctrl+F` | Char Right | Alternative to Right arrow |

### History Navigation

| Key | Action | Description |
|-----|--------|-------------|
| `Up` | Previous Pattern | Navigate to previous pattern in history |
| `Down` | Next Pattern | Navigate to next pattern in history |
| `Ctrl+P` | Previous Pattern | Alternative to Up arrow |
| `Ctrl+N` | Next Pattern | Alternative to Down arrow |

### Mode Exit

| Key | Action | Description |
|-----|--------|-------------|
| `Esc` | Exit Insert | Return to Normal mode without applying |
| `Enter` | Apply & Exit | Apply pattern and return to Normal mode |
| `Ctrl+C` | Cancel | Cancel edit and return to Normal mode |

## Command Mode

Command mode provides advanced operations through a command-line interface.

### File Operations

| Command | Description |
|---------|-------------|
| `:o [file]` | Open file |
| `:w` | Save session |
| `:q` | Quit application |
| `:q!` | Force quit without saving |
| `:wq` | Save and quit |

### Configuration

| Command | Description |
|---------|-------------|
| `:set [option]` | Set configuration option |
| `:config` | Show current configuration |
| `:reload` | Reload configuration file |

### Session Management

| Command | Description |
|---------|-------------|
| `:save [name]` | Save current session with name |
| `:load [name]` | Load saved session |
| `:sessions` | List available sessions |

### Export Operations

| Command | Description |
|---------|-------------|
| `:export [file]` | Export filtered results |
| `:ripgrep` | Generate ripgrep command |

## Component-Specific Shortcuts

Different components have specialized shortcuts when focused:

### Viewer Component

| Key | Action | Context |
|-----|--------|---------|
| `j/k` | Scroll | Vertical navigation |
| `h/l` | Horizontal scroll | When content is wide |
| `/` | Search | Start text search |
| `n/N` | Search nav | Navigate search results |
| `Space` | Page down | Alternative to Ctrl+D |

### Include Filter Pane

| Key | Action | Context |
|-----|--------|---------|
| `i` | Edit mode | Enter Insert mode for editing |
| `a` | Add pattern | Create new include pattern |
| `d` | Delete pattern | Remove selected pattern |
| `j/k` | Navigate | Move between patterns |

### Exclude Filter Pane

| Key | Action | Context |
|-----|--------|---------|
| `i` | Edit mode | Enter Insert mode for editing |
| `a` | Add pattern | Create new exclude pattern |
| `d` | Delete pattern | Remove selected pattern |
| `j/k` | Navigate | Move between patterns |

### Tab Bar

| Key | Action | Context |
|-----|--------|---------|
| `h/l` | Navigate | Move between tabs |
| `w` | Close tab | Close current tab |
| `1-9` | Jump to tab | Direct tab selection |

## Multi-Key Sequences

Some operations require multiple keystrokes:

### Navigation Sequences

| Sequence | Action | Description |
|----------|--------|-------------|
| `gg` | Go to top | Jump to beginning |
| `G` | Go to bottom | Jump to end (single key) |

### Advanced Operations

| Sequence | Action | Description |
|----------|--------|-------------|
| `dd` | Delete line | Delete current pattern/line |
| `yy` | Yank line | Copy current pattern/line |
| `cc` | Change line | Delete and enter Insert mode |

**Note**: Multi-key sequences have a timeout of 1 second by default.

## Quick Reference Tables

### Mode-Specific Quick Reference

#### Normal Mode Essentials
```
Navigation:  j/k (up/down)  h/l (left/right)  gg/G (top/bottom)
Paging:      Ctrl+D/U (page down/up)
Focus:       Tab (next pane)
File:        o (open)  w (close tab)  1-9 (switch tab)
Patterns:    a (add)  d (delete)  y (copy)
Mode:        i (insert)  : (command)
App:         q (quit)  ? (help)
```

#### Insert Mode Essentials
```
Edit:        Enter (apply)  Backspace (delete)  Ctrl+W (delete word)
Navigate:    Ctrl+A/E (line start/end)  Left/Right (char)
History:     Up/Down (previous/next pattern)
Exit:        Esc (cancel)  Enter (apply)
```

#### Command Mode Essentials
```
File:        :o (open)  :w (save)  :q (quit)  :wq (save+quit)
Session:     :save/:load (session management)
Export:      :export (save results)  :ripgrep (generate command)
Config:      :set (option)  :config (show)
```

### Component Focus Cycle

```
Viewer → Include Filter → Exclude Filter → Tabs → Status Bar → (back to Viewer)
```

Use `Tab` to cycle forward, `Shift+Tab` to cycle backward.

## Customization

### Configuration File

Keybindings can be customized through the configuration file:

```json
{
  "keyboard": {
    "enableVimBindings": true,
    "caseSensitive": false,
    "sequenceTimeoutMs": 1000,
    "customBindings": {
      "ctrl+r": "reload_config",
      "F5": "refresh_view"
    }
  }
}
```

### Custom Bindings Format

```json
"customBindings": {
  "key_combination": "action_name"
}
```

**Supported Key Formats**:
- Single keys: `a`, `j`, `space`, `enter`
- Ctrl combinations: `ctrl+c`, `ctrl+shift+r`
- Function keys: `f1`, `f2`, etc.
- Special keys: `tab`, `esc`, `backspace`, `delete`

### Disabling Vim Mode

To use simplified keybindings:

```json
{
  "keyboard": {
    "enableVimBindings": false
  }
}
```

This enables a more conventional interface with:
- Arrow keys for navigation
- Enter for actions
- Tab for focus cycling
- No mode switching

## Tips and Best Practices

### Efficient Workflow

1. **Start in Normal mode**: Always begin navigation here
2. **Use Tab liberally**: Quick way to move between components
3. **Master gg/G**: Fastest way to jump to top/bottom
4. **Learn number keys**: Direct tab switching (1-9)
5. **Use pattern shortcuts**: `a/d/y` for pattern management

### Common Patterns

**Quick filter creation**:
1. `Tab` to Include Filter pane
2. `a` to add pattern
3. Type pattern, `Enter` to apply
4. `Esc` to return to Normal mode

**File browsing**:
1. `o` to open file
2. `1-9` to switch between tabs
3. `w` to close unwanted tabs

**Search workflow**:
1. Focus on Viewer with `Tab`
2. `/` to start search
3. `n/N` to navigate results

### Troubleshooting

**Stuck in a mode?**
- Press `Esc` multiple times to ensure Normal mode

**Keybinding not working?**
- Check if you're in the correct mode
- Verify component focus (Tab to cycle)
- Check for multi-key sequences (wait for timeout)

**Need help quickly?**
- Press `?` for interactive help overlay
- Press `h` in Normal mode for context help

---

*This reference covers qf version 1.0+. For the latest updates, see the project documentation.*