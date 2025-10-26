# GoTime Raycast Extension - Setup Guide

## Quick Start

### 1. Verify gotime is installed

The extension expects `gt` at: `~/.local/bin/gt`

Check if it exists:

```bash
ls -la ~/.local/bin/gt
```

If not installed, install it:

```bash
cd ../../go/gotime
make install
```

Test it works:

```bash
~/.local/bin/gt list --active --json
```

### 2. Run the Extension

```bash
cd typescript/gotime-raycast
npm run dev
```

This will open Raycast with the extension loaded.

### 3. Try the Commands

Open Raycast (⌘ Space) and search for:

- **Active Timers** - View and stop running timers
- **Weekly Report** - See your week's time tracking
- **Start Timer** - Start a new timer

## All Three Commands Work

✅ **Active Timers**

- Shows all running timers
- Live duration updates (every second!)
- Stop individual (`⌘ S`) or all timers (`⌘ ⇧ S`)
- Displays tags and keywords

✅ **Weekly Report**

- Total time for the week
- Breakdown by keyword
- Daily totals
- Detailed view per keyword

✅ **Start Timer**

- Simple form interface
- Keyword + optional tags
- Validation included

## Customizing the GT Path

If your `gt` binary is NOT at `~/.local/bin/gt`, you need to update the path:

1. Find your gt location:

   ```bash
   which gt
   ```

2. Update the path in these files:
   - `src/active-timers.tsx` - Line ~58 and ~105, ~143
   - `src/weekly-report.tsx` - Line ~72
   - `src/start-timer.tsx` - Line ~56

3. Search and replace:

   ```bash
   # Find all occurrences
   grep -r "~/.local/bin/gt" src/

   # Replace with your path (example):
   sed -i '' 's|~/.local/bin/gt|/usr/local/bin/gt|g' src/*.tsx
   ```

4. Rebuild:

   ```bash
   npm run build
   ```

## Testing Each Feature

### Test Active Timers

1. Start a timer from terminal:

   ```bash
   gt start testing raycast extension
   ```

2. Open Raycast → "Active Timers"
3. You should see:
   - Timer with keyword "testing"
   - Live updating duration
   - Tags "raycast" and "extension"
4. Try stopping it with `⌘ S`

### Test Weekly Report

1. Make sure you have data this week:

   ```bash
   gt report
   ```

2. Open Raycast → "Weekly Report"
3. You should see:
   - Total time summary
   - Keywords section
   - Daily totals section
4. Select a keyword to see daily breakdown

### Test Start Timer

1. Open Raycast → "Start Timer"
2. Fill in:
   - Keyword: `raycast-test`
   - Tags: `testing, ui`
3. Press `⌘ ⏎`
4. Verify it started:

   ```bash
   gt list --active
   ```

## Development Tips

### Enable Developer Mode

1. Open Raycast preferences (`⌘ ,`)
2. Go to "Advanced" tab
3. Enable "Developer Mode"
4. Press `⌘ ⌥ I` to open console (useful for debugging)

### Hot Reload

When running `npm run dev`, changes to source files trigger automatic reloads in Raycast.

### Check Logs

Use `console.log()` in your code and view output in the Developer Console (`⌘ ⌥ I`).

## Common Issues

### Extension won't load

- Make sure you're running `npm run dev`
- Check Developer Console for errors
- Verify package.json syntax is valid

### Can't find gt command

- Verify path: `ls ~/.local/bin/gt`
- Update hardcoded paths in source files if needed
- Make sure gt is executable: `chmod +x ~/.local/bin/gt`

### Timers don't update

- They update every second via setInterval
- Try refreshing with `⌘ R`
- Check Developer Console for errors

## Next Steps

1. **Add to Raycast permanently**: Import the extension in Raycast preferences
2. **Set hotkeys**: Assign global keyboard shortcuts for quick access
3. **Customize**: Modify the source code to add features you want

Enjoy your visual time tracking interface! 🎉
