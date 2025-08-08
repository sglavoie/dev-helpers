# GoTime (gt) - Personal Time Tracking CLI

A fast and simple time tracking CLI tool designed to replace commercial solutions like Clockify. Built with Go, featuring multiple concurrent stopwatches, keyword-based organization, tag filtering, and powerful reporting capabilities.

## Features

- ⏱️ **Multiple Concurrent Timers**: Run unlimited stopwatches simultaneously
- 🏷️ **Keywords & Tags**: Organize activities with primary keywords and secondary tags
- 📊 **Rich Reporting**: Weekly, monthly, yearly reports with advanced filtering
- 🔄 **Continue Tracking**: Resume previous activities easily
- 📈 **Live Dashboard**: Real-time monitoring of active timers
- 🎯 **Smart ID System**: Short IDs (1-10) for quick reference + permanent UUIDs
- 💾 **JSON Storage**: Simple, portable configuration file

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
gt report --week
gt report --keyword coding

# Continue previous work
gt continue coding
gt continue --last

# Live dashboard
gt watch
```

## Commands

### Time Tracking

- `gt start <keyword> [tags...]` - Start new timer
- `gt stop [keyword | --id N | --all]` - Stop timer(s)
- `gt continue [keyword | --last | --id N]` - Resume previous timer

### Entry Management

- `gt list [filters...]` - List entries with filtering
- `gt add` - Add retroactive entries (interactive form)
- `gt set [keyword | --id N] [field value]` - Update entry fields
- `gt delete [keyword | --id N]` - Delete entries

### Reporting

- `gt report [time-range] [filters...]` - Generate reports
- `gt watch` - Live dashboard

## Filtering Options

### Time Ranges

- Default: Current week (Sunday-Saturday)
- `--today` / `--yesterday` - Single day
- `--days N` - Last N days
- `--month` / `--year` - Calendar periods
- `--between YYYY-MM-DD,YYYY-MM-DD` - Custom range

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

# Check status
gt list --active

# Stop specific timer
gt stop --id 1

# Weekly report
gt report
```

### Advanced Filtering

```bash
# Coding activities last 30 days
gt report --days 30 --keyword coding

# All team-related entries this month
gt report --month --tags team

# Everything except personal activities
gt report --tags personal --invert-tags
```

### Entry Management

```bash
# Add retroactive entries
gt add

# Interactive time setting
gt set coding

# Direct field updates
gt set --id 5 duration 3600
gt set coding keyword development

# Continue previous work
gt continue --last
gt continue coding
```

## Configuration

Configuration is stored in `~/.gotime.json` by default. Use `--config <path>` to specify a custom location.

Example configuration structure:

```json
{
  "entries": [
    {
      "id": "uuid-v7-here",
      "short_id": 1,
      "keyword": "coding",
      "tags": ["golang", "cli"],
      "start_time": "2025-08-08T10:00:00Z",
      "end_time": "2025-08-08T12:30:00Z",
      "duration": 9000,
      "active": false
    }
  ],
  "next_short_id": 2,
  "last_entry_keyword": "coding"
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

### Built with

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [go-pretty](https://github.com/jedib0t/go-pretty) - Table formatting
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI components
- [UUID](https://github.com/google/uuid) - UUIDv7 generation

### Recent Updates

- Added `gt add` command for retroactive entry creation
- Enhanced field editor with end_time field for completed entries
- **Fixed critical timezone issues**: Time parsing now correctly uses local timezone instead of UTC
    - Field editor now parses start_time and end_time as local time
    - Date range parsing (--between flag) now uses local dates
    - Duration calculations are now accurate across all timezones
- Fixed duration calculation issues with retrospective entries
- Added comprehensive test coverage for duration handling and timezone behavior
- Improved default values for retroactive entries (1-hour completed entries)
