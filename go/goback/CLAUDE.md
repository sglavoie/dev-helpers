# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run

```bash
make build        # compile the binary
make install      # build and copy to ~/.local/bin/goback
make clean        # remove the compiled binary
make uninstall    # remove from ~/.local/bin
```

There are no tests in this project yet.

## Architecture

goback is a CLI backup tool that wraps `rsync` for incremental daily, weekly, and monthly backups. It targets macOS (uses `diskutil eject` for volume unmounting) and uses Cobra for CLI structure, Viper for configuration, SQLite for backup history, and Bubbletea for interactive prompts and paged output.

### Command layer (`cmd/`)

All commands are Cobra subcommands registered under `RootCmd`. The `PersistentPreRun` hook on the root command calls `config.MustInitConfig` before every subcommand, but skips config validation for `config` subcommands to avoid a chicken-and-egg problem.

The main commands are `run daily|weekly|monthly` (execute backups), `preview daily|weekly|monthly` (print the rsync command without running it), `config edit|print|reset`, `clean db|logs`, `usage last|view|reset`, and `eject`. Preview supports `--scan` (daily only) to show large new items that would be flagged before backup.

### Backup flow

The backup pipeline flows through three stages. First, `pkg/buildcmd` constructs the rsync command: `BuildDaily/Weekly/Monthly` factory functions read source/destination from Viper config, validate paths, and assemble the command string from per-type boolean flags and exclude patterns. Daily backups copy from the configured source to `<dest>/daily/`. Weekly and monthly copy from `<dest>/daily/` to `<dest>/weekly/` or `<dest>/monthly/`, treating the daily snapshot as input. For daily backups, `pkg/run` runs a pre-backup scan (`pkg/scan`) that detects large new items in the source not yet in the destination, using concurrent goroutines bounded by `runtime.NumCPU()`. If flagged, the user can proceed, exclude permanently (appends to config and saves), or abort. Second, `pkg/run` orchestrates execution by calling the builder, showing a confirmation prompt (if `confirmExec` is true), and running the command via `bash -c`. Third, the result is recorded in SQLite and the volume is optionally ejected.

### Backup type system (`pkg/models`)

`BackupTypes` is an interface implemented by three empty structs (`Daily`, `Weekly`, `Monthly`) plus `NoBackupType`. These act as a Go-idiomatic enum with `String()` methods and flow through the entire stack to parameterize config lookups, DB queries, and path construction.

### Configuration (`pkg/config`)

Config lives at `~/.goback.json` and is read via Viper. Top-level keys include `confirmExec`, `ejectOnExit`, `showProgress`, `editor`, `source`, `destination`, and a nested `rsync` object with per-type settings (`rsync.daily.*`, `rsync.weekly.*`). Monthly has no dedicated defaults and must be added manually by the user. Defaults are set in `defaults.go`.

Daily-specific detection keys (`rsync.daily.*`):
- `detectLargeItems` (bool, default `true`): enable pre-backup scan for large new items
- `largeItemThresholdGB` (float, default `1.0`): size threshold in GB above which a new item is flagged
- `largeItemScanDepth` (int, default `1`): how many directory levels deep to scan (1 = top-level only)

### Database (`pkg/db`)

SQLite database at `~/.goback.db` with a single `backups` table storing id, created_at, backup_type, execution_time, and command. The `WithDb` callback pattern opens, passes, and closes the connection. `WithQuery` closes the connection before rows are consumed, so rows must be read within a `WithRows` callback.

### Interactive UI

Bubbletea is used for yes/no confirmation prompts (`pkg/inputs`), a full-screen scrollable pager (`pkg/printer`), and config display. The `go-pretty` library renders tabular backup history.
