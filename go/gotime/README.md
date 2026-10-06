# GoTime (gt) - Personal Time Tracking CLI

A fast, simple time tracking CLI tool designed to replace commercial solutions like Clockify for personal use. Built with Go for speed and reliability.

## Key Features

- ⏱️ **Multiple Concurrent Timers**: Run unlimited stopwatches simultaneously
- 🏷️ **Keywords & Tags**: Organize activities with primary keywords and secondary tags  
- 📊 **Advanced Filtering**: Duration filters, date ranges, tag filtering, and more
- 🔄 **Continue & Stash**: Resume previous work or temporarily pause running timers
- ↩️ **Undo Operations**: Reverse destructive operations like deletes and bulk edits
- 🎯 **Smart ID System**: Short IDs (1-10) for quick reference + permanent UUIDs
- 💾 **JSON Storage**: Simple, portable configuration file
- 🏷️ **Tag Management**: List, rename, and remove tags across all entries

## Installation

### Using Make (Recommended)

```bash
make build          # Build the binary
make install        # Build and install to ~/.local/bin
make clean          # Clean build artifacts
```

### Manual Build

```bash
go build -o gt
```

## Quick Start

```bash
# Start tracking
gt start coding golang cli

# Check active timers
gt list --active

# Stop tracking
gt stop coding

# Generate reports
gt report --today
gt report --week --keyword coding

# Continue previous work
gt continue coding
gt continue --last

# Stash running timers temporarily
gt stash

# Undo mistakes
gt undo
```

## Commands

### Time Tracking

- `gt start <keyword> [tags...]` - Start new timer
- `gt stop [keyword | --id N | --all]` - Stop timer(s)
- `gt continue [keyword | --last | --id N]` - Resume previous timer
- `gt stash [show|apply|clear]` - Temporarily pause/manage running timers

### Entry Management

- `gt list [filters...]` - List entries with advanced filtering
- `gt add` - Add retroactive entries (interactive form)
- `gt set [keyword | --id N] [field value]` - Update entry fields (supports bulk editing)
- `gt delete [keyword | --id N]` - Delete entries

### Advanced Features

- `gt report [time-range] [filters...]` - Generate detailed reports
- `gt undo [--list]` - Undo destructive operations or list undo history
- `gt tags [list|rename|remove]` - Manage tags across all entries
- `gt pop` - Resume stashed entries

## Editing from scripts and Raycast

Use `gt list --json` to obtain permanent entry UUIDs. `set`, `stop`, `continue`,
and `delete` accept a UUID wherever they accept a short ID. Short IDs are
reassigned when entries are added, removed, or moved in time; use UUIDs when
acting on a previously displayed entry.

`update` changes several fields together, preserves the UUID, and records one
undo operation. It validates the complete edit before saving:

```bash
gt update UUID --keyword review --tags work,project
gt update UUID --start '2026-10-01T09:00:00-06:00' --end '2026-10-01T10:00:00-06:00'
gt update UUID --tags ''                 # clear tags
gt update UUID --end active              # resume this entry in place
gt update UUID --duration 1800           # seconds; state stays running/stopped
```

Replace `UUID` with the entry's `id` from JSON. Timestamps require an explicit
time zone (RFC3339); an end time completes the entry, while `--end active`
clears the end time and keeps it running. Metadata-only edits leave its timing
and running state unchanged. Duration edits move the end of a completed entry
or the start of an active timer, and cannot be combined with timestamp flags.
Future timestamps, reversed ranges, and conflicting active keywords are refused.
Restore stashed entries before editing them. Use `gt undo` to restore an update.

The [Raycast extension](../../typescript/gotime-raycast/README.md) uses this
command; reinstall the CLI with `make install` when updating the extension.

Saves write a temporary file, sync it, and replace the data file atomically.
Existing file permissions and configuration symlinks are preserved. A short
file lock protects the final comparison and replacement; if another process
changed the data since it was loaded, the stale save fails with a retry message
instead of overwriting that work. Reload the Raycast list or reopen the CLI editor
before retrying. The adjacent `.lock` file is normal and remains on disk; its
presence alone does not mean a process holds the lock.

## Advanced Filtering

### Time Ranges

- Default: Today's entries
- `--today` / `--yesterday` - Single day
- `--week` / `--month` / `--year` - Calendar periods  
- `--days N` - Last N days
- `--between YYYY-MM-DD,YYYY-MM-DD` - Custom range
- `--from YYYY-MM-DD --to YYYY-MM-DD` - Flexible date range

### Duration Filters

- `--min-duration 1h` - Entries >= 1 hour (supports `1h30m`, `90m`, `5400` seconds)
- `--max-duration 4h` - Entries <= 4 hours  
- Combined: `--min-duration 30m --max-duration 2h` - Duration range

### Content Filters

- `--keyword <keyword>` - Filter by keyword
- `--tags <tag1,tag2>` - Include entries with specified tags
- `--invert-tags` - Exclude entries with specified tags
- `--active` / `--no-active` - Filter by active status

## Usage Examples

### Basic Workflow

```bash
# Start multiple activities
gt start coding golang cli
gt start meeting team planning

# Check status and filter by duration
gt list --active
gt list --min-duration 1h  # Show entries >= 1 hour

# Temporarily pause all running timers
gt stash

# Resume work later
gt pop
```

### Advanced Filtering

```bash
# Duration-based filtering
gt list --min-duration 30m --max-duration 2h  # 30min - 2 hour entries
gt report --week --min-duration 1h            # Weekly report, entries >= 1h

# Date range filtering  
gt list --from 2025-08-01 --to 2025-08-07     # Specific date range
gt report --month --keyword coding            # Monthly coding report

# Tag filtering
gt list --tags team,urgent                    # Team OR urgent entries
gt report --invert-tags personal --week       # Exclude personal entries
```

### Advanced Operations

```bash
# Tag management
gt tags list --count                          # List tags with usage counts  
gt tags rename old-name new-name              # Rename tag across all entries
gt tags remove deprecated-tag                 # Remove tag from all entries

# Undo operations
gt undo                                       # Undo last destructive operation
gt undo --list                               # Show available undo operations

# Bulk editing
gt set --keyword coding duration 7200         # Set all "coding" entries to 2 hours
gt set --keyword coding endtime "2025-08-08 17:00:00"  # Set end time for all "coding" entries
```

## Configuration

Configuration is stored in `~/.gotime.json` by default. Use `--config <path>` to specify a custom location.

Example configuration structure:

```json
{
  "entries": [
    {
      "id": "01234567-89ab-7def-0123-456789abcdef",
      "short_id": 1,
      "keyword": "coding",
      "tags": ["golang", "cli"],
      "start_time": "2025-08-08T10:00:00-04:00",
      "end_time": "2025-08-08T12:30:00-04:00", 
      "duration": 9000,
      "active": false,
      "stashed": false
    }
  ],
  "next_short_id": 2,
  "last_entry_keyword": "coding",
  "stashes": [],
  "undo_history": []
}
```

## Global Flags

- `--config <path>` - Custom config file location
- `--verbose, -v` - Verbose output
- `--help, -h` - Show help

## Design Goals

1. **Speed**: < 100ms startup time for common commands
2. **Simplicity**: Intuitive commands requiring minimal flags
3. **Flexibility**: Support complex filtering scenarios
4. **Reliability**: No data loss, consistent state management
5. **Portability**: Single binary, JSON configuration

## Development

### Running Tests

```bash
make test           # Run all tests
make test-cmd       # Run cmd package tests only
make test-tui       # Run TUI package tests only
go test ./... -v    # Verbose test output
```

### Building and Running

```bash
make build          # Build the binary
make install        # Install to ~/.local/bin
make dev            # Build with race detection
make fmt            # Format code
make lint           # Lint code (requires golangci-lint)
```
