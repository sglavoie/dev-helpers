# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run

```bash
make build        # compile the binary
make install      # build and copy to ~/.local/bin/goback
make clean        # remove the compiled binary
make uninstall    # remove from ~/.local/bin
```

Tests live beside the code they cover and run with `go test ./...`.

## Architecture

goback is a CLI backup tool that wraps `rsync` for incremental daily, weekly, and monthly backups. It targets macOS (uses `diskutil eject` for volume unmounting) and uses Cobra for CLI structure, Viper for configuration, SQLite for backup history, and Bubbletea for interactive prompts and paged output.

### Command layer (`cmd/`)

All commands are Cobra subcommands registered under `RootCmd`. The `PersistentPreRun` hook on the root command calls `config.MustInitConfig` before every subcommand, but skips config validation for `config` subcommands to avoid a chicken-and-egg problem.

The main commands are `run daily|weekly|monthly` (execute backups), `preview daily|weekly|monthly` (print the rsync command without running it), `config edit|print|reset`, `clean db|logs`, `usage last|view|reset`, and `eject`. Preview supports `--scan` (daily only) to show large new items that would be flagged before backup.

### Backup flow

The backup pipeline flows through three stages. First, `pkg/buildcmd` constructs the rsync command: `BuildDaily/Weekly/Monthly` factory functions read source/destination from Viper config, validate paths, and assemble the command string from per-type boolean flags and exclude patterns. Daily backups copy from the configured source to `<dest>/daily/`. Weekly and monthly copy from `<dest>/daily/` to `<dest>/weekly/` or `<dest>/monthly/`, treating the daily snapshot as input. For daily backups, `pkg/run` runs a pre-backup scan (`pkg/scan`) that detects large new items in the source not yet in the destination, using concurrent goroutines bounded by `runtime.NumCPU()`. If flagged, the user can proceed, exclude permanently (appends to config and saves), or abort. Second, `pkg/run` orchestrates execution by calling the builder, checking that rsync exists, showing a confirmation prompt (if `confirmExec` is true), and running the command via `bash -c` under a signal context created once per `run` invocation. Third, the result is recorded in SQLite and the volume is optionally ejected.

`pkg/run` classifies the main rsync as skipped, declined, interrupted, failed, or succeeded, and the daily companions run only when rsync was actually attempted, whether it succeeded or failed. A build or path failure, a missing rsync, a declined confirmation, and an interruption all skip the companions. A companion never changes the exit status: `run daily` fails only when its own rsync failed, while every companion outcome is recorded in history and shown in the combined table each profile prints once its companions have finished (replacing the history summary that used to be printed there). `run all` calls the same daily entry point, so companions behave identically there, and `preview daily` lists the companions with the exact argument vector of a real run and of a dry run. A companion runs with its `dryRunArgs` when the backup is a dry run from either `--dry-run` or `rsync.daily.dryRun`.

### Backup type system (`pkg/models`)

`BackupTypes` is an interface implemented by three empty structs (`Daily`, `Weekly`, `Monthly`) plus `NoBackupType`. These act as a Go-idiomatic enum with `String()` methods and flow through the entire stack to parameterize config lookups, DB queries, and path construction.

### Configuration (`pkg/config`)

Config lives at `~/.goback.json` and is read via Viper. Top-level keys include `confirmExec`, `ejectOnExit`, `showProgress`, `editor`, `source`, `destination`, and a nested `rsync` object with per-type settings (`rsync.daily.*`, `rsync.weekly.*`). Monthly has no dedicated defaults and must be added manually by the user. Defaults are set in `defaults.go`.

Daily-specific detection keys (`rsync.daily.*`):
- `detectLargeItems` (bool, default `true`): enable pre-backup scan for large new items
- `largeItemThresholdGB` (float, default `1.0`): size threshold in GB above which a new item is flagged
- `largeItemScanDepth` (int, default `1`): how many directory levels deep to scan (1 = top-level only)

A profile may also declare `dailyCompanions`, a list of programs run alongside its daily backup. Each entry has `id` (letters, digits, dots, dashes and underscores, unique within the profile), an optional `name` for display, `command` (the program followed by each argument as a separate list item), and optional `dryRunArgs` appended when the run is a dry run. `companions.go` parses and validates them; a command written as one shell string is rejected because companions are executed without a shell.

The Apple Photos companion this repository ships alongside is configured as:

```json
"dailyCompanions": [
  {
    "id": "apple-photos",
    "name": "Apple Photos",
    "command": ["photos-backup", "daily"],
    "dryRunArgs": ["--dry-run"]
  }
]
```

`photos-backup` is the console script of `python/photos_backup`, resolved through `PATH`, and `pkg/run/installed_cli_test.go` runs a real executable of that name from a temporary `PATH` entry to prove the resolution and the argument vector across process boundaries.

### Database (`pkg/db`)

SQLite database at `~/.goback.db` with a single `backups` table storing id, created_at, backup_type, execution_time, command, profile, and exit_code. `backup_type` is `daily`, `weekly`, `monthly`, or `companion/<id>` for a companion program. Every write goes through `db.RecordBackup`, which logs a warning rather than failing the backup. The `WithDb` callback pattern opens, passes, and closes the connection. `WithQuery` closes the connection before rows are consumed, so rows must be read within a `WithRows` callback.

### Interactive UI

Bubbletea is used for yes/no confirmation prompts (`pkg/inputs`), a full-screen scrollable pager (`pkg/printer`), and config display. The `go-pretty` library renders tabular backup history.
