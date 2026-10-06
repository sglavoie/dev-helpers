# ccl - Claude Code Limits

A small CLI that tells you how much of your weekly Claude Code usage limit you
"should" have used by now. Compare it with the real figure to see whether
you're ahead of or behind pace.

The expected share grows linearly through each work day and stays flat on days
off, so a week with six work days gives each of them 1/6 of the budget.

## Installation

```bash
go install .        # installs to $GOPATH/bin (e.g. ~/.go/bin)
```

Make sure `$GOPATH/bin` is on your `PATH`.

## Usage

```bash
ccl                 # Expected usage: 37.8%
ccl --short         # 37.8 (compact, for status lines)
ccl status          # full view of the current cycle
ccl config          # show the configuration and where it lives
```

`ccl status` output:

```
Cycle:                      Sun Oct 4 12:59 AM -> Sun Oct 11 12:59 AM
Now:                        Tue Oct 6 7:23 AM
Expected:                   37.8%
Work days:                  2/6 completed
Current work segment ends:  Wed Oct 7 12:59 AM
Remaining:                  4d 17h 35m
```

On a day off, `Next work segment ends` shows the end of the next work segment,
or `none remaining this cycle` when all work segments have finished. These are
segment end times, not the time to begin working.

## Configuration

Settings are stored in `~/.config/claude-usage/config.json`. The first run
creates the file with these defaults:

```json
{
  "work_days": ["mon", "tue", "wed", "thu", "fri", "sat"],
  "reset_time": "Thu 2:59 PM"
}
```

Change them with `ccl set`:

```bash
ccl set days mon,tue,wed,thu,fri    # comma-separated, any of mon..sun
ccl set reset "Sun 12:59 AM"        # weekday + 12-hour time, local time zone
```

Set the reset time to the weekly reset shown on your Claude usage page.

## How it works

- A **cycle** is the 7 days from the most recent reset time to the next one.
- The cycle is split into seven 24-hour **segments**, each starting at the
  reset clock time.
- Each segment counts for the weekday it **ends** on. With a `Thu 2:59 PM`
  reset, the segment from Thursday 2:59 PM to Friday 2:59 PM counts as Friday.
- During a work-day segment, the expected share grows from
  `done × 100 / N` to `(done + 1) × 100 / N`, where `N` is the number of work
  days and `done` is the number of work-day segments already finished. During
  a day off it stays at `done × 100 / N`.

## Status line integration

`--short` prints just the number with no newline, so a status line script can
use it without parsing anything. Check that `ccl` is installed first:

```bash
if command -v ccl >/dev/null 2>&1; then
    ccl_out=$(ccl --short 2>/dev/null)
fi
```

## Development

```bash
go test ./...
go vet ./...
```
