# GoTime - Time Tracker for Raycast

A Raycast extension for managing your time tracking with the [gotime](../../go/gotime) CLI tool.

## Features

- **Active Timers**: View all running timers with live duration updates
    - Stop individual timers or all timers at once
    - See tags and keywords
    - Real-time duration counter (updates every second)

- **Weekly Report**: View your weekly time tracking summary
    - See total time tracked
    - Breakdown by keyword with daily details
    - Daily totals for each day of the week

- **Start Timer**: Quick form to start a new timer
    - Enter keyword and optional tags
    - Input validation
    - Instant feedback

## Prerequisites

- [Raycast](https://www.raycast.com/) installed
- [gotime (gt)](../../go/gotime) CLI installed at `~/.local/bin/gt`

### Installing gotime

```bash
cd ../../go/gotime
make install
```

This will install the `gt` binary to `~/.local/bin/gt`.

**Important**: The extension expects `gt` to be at `~/.local/bin/gt`. If you installed it elsewhere, you'll need to update the paths in the source code (search for `~/.local/bin/gt` and replace with your path).

## Installation

### From Source (Development)

1. Navigate to this directory:

   ```bash
   cd typescript/gotime-raycast
   ```

2. Install dependencies (already done):

   ```bash
   npm install
   ```

3. Run in development mode:

   ```bash
   npm run dev
   ```

## Usage

### Active Timers

- **Open**: Search for "Active Timers" in Raycast
- **Stop a timer**: Select a timer and press `⌘ S`
- **Stop all timers**: Press `⌘ ⇧ S`
- **Refresh**: Press `⌘ R`

The duration updates automatically every second!

### Weekly Report

- **Open**: Search for "Weekly Report" in Raycast
- **View details**: Select a keyword to see daily breakdown in the detail view
- **Copy keyword**: Select a keyword and press `⌘ C`
- **Refresh**: Press `⌘ R`

### Start Timer

- **Open**: Search for "Start Timer" in Raycast
- **Fill form**:
    - Keyword: Required (e.g., "coding", "meeting")
    - Tags: Optional, comma-separated (e.g., "golang, cli")
- **Submit**: Press `⌘ ⏎`

## Keyboard Shortcuts

### Active Timers

- `⌘ S` - Stop selected timer
- `⌘ ⇧ S` - Stop all timers
- `⌘ R` - Refresh list

### Weekly Report

- `⌘ C` - Copy keyword name
- `⌘ R` - Refresh report

### Start Timer

- `⌘ ⏎` - Submit form

## Development

### Project Structure

```
gotime-raycast/
├── src/
│   ├── active-timers.tsx    # Active timers list view
│   ├── weekly-report.tsx    # Weekly report view
│   └── start-timer.tsx      # Start timer form
├── package.json            # Extension manifest
├── tsconfig.json          # TypeScript config
└── README.md              # This file
```

### Scripts

- `npm run dev` - Start development server
- `npm run build` - Build extension
- `npm run lint` - Lint code
- `npm run fix-lint` - Fix linting issues

## Customization

If your `gt` binary is installed in a different location, you'll need to update the hardcoded path in all three command files:

1. Open each file: `src/active-timers.tsx`, `src/weekly-report.tsx`, `src/start-timer.tsx`
2. Find: `~/.local/bin/gt`
3. Replace with your gt path (e.g., `/usr/local/bin/gt`)

## Troubleshooting

### "gt: command not found" or extension crashes

Make sure the `gt` binary is at `~/.local/bin/gt`:

```bash
ls -la ~/.local/bin/gt
```

If it's elsewhere, update the paths in the source files as described in Customization above.

### Timers not updating

The Active Timers view updates durations every second. If they're not updating:

1. Check if the timer is actually active: `gt list --active`
2. Try refreshing with `⌘ R`

## What's Next

You can now use Raycast as a beautiful visual interface for your gotime tracker! Some ideas:

1. **Set global hotkeys** - In Raycast preferences, assign keyboard shortcuts to these commands for instant access
2. **Create workflows** - Combine with other Raycast extensions for productivity workflows
3. **Customize** - Fork and modify to add your own features

## License

MIT
