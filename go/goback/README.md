# goback

Revamped version of [rsync backup](../../python/rsync_backup/README.md), ported to Go.

## Quick start

From this directory, build and install the CLI (requires Go and rsync):

```bash
mkdir -p ~/.local/bin
go build -o ~/.local/bin/goback .
```

Make sure `~/.local/bin` is on your `PATH`. Save this complete example as
`~/.goback.json`, replacing the source and destination with your own paths.
The destination directory must already exist on the mounted backup drive.
The source's trailing slash copies its contents into `daily/`.

```json
{
  "confirmExec": true,
  "ejectOnExit": false,
  "showProgress": true,
  "editor": "",
  "profiles": {
    "default": {
      "source": "/Users/me/Documents/",
      "destination": "/Volumes/Backup/Documents",
      "rsync": {
        "daily": {
          "archive": true,
          "hardLinks": true,
          "delete": false,
          "ignoreErrors": false,
          "deleteExcluded": false,
          "excludedPatterns": [".DS_Store", "*.tmp"]
        },
        "weekly": {
          "archive": true,
          "hardLinks": true,
          "delete": false,
          "ignoreErrors": false,
          "deleteExcluded": false,
          "excludedPatterns": []
        },
        "monthly": {
          "archive": true,
          "hardLinks": true,
          "delete": false,
          "ignoreErrors": false,
          "deleteExcluded": false,
          "excludedPatterns": []
        }
      }
    }
  }
}
```

A single profile needs no hostname setting. This example leaves deletion off;
review the preview and dry run before your first backup:

```bash
goback config check
goback preview daily
goback run daily --dry-run
goback run daily
goback usage last --summary
goback run weekly
```

Weekly and monthly copy the existing `daily/` backup, so they also work when
the original source is offline. These are three maintained directories, not a
new dated snapshot for every invocation. Generated configurations use the same
daily, weekly, and monthly settings as this example, with deletion disabled.
Existing configuration files keep their current settings. The optional global
`mirror` and daily companions can be added later using the examples below.

## Usage

See available commands:

```bash
just
```

## Commands

| Command | What it does |
| --- | --- |
| `run daily\|weekly\|monthly\|all` | Run the profile's incremental snapshot backups. |
| `preview daily\|weekly\|monthly` | Print the rsync command a run would execute, without running it. |
| `mirror [--dry-run]` | Mirror one configured directory onto another, exactly. |
| `eject [--all\|--volume NAME\|--list]` | Unmount the active profile's volume, every configured volume, or one named volume, or list the mounted ones. |
| `usage last\|view\|reset` | Read and trim the backup history. |
| `profiles` | List configured profiles, their paths, and automatic selection. |
| `status [--daily] [--older-than 48h] [--check] [--json]` | Show configured backups and companions, including those with no recorded attempts. |
| `config check\|edit\|print\|reset` | Check or manage `~/.goback.json`. |
| `clean db\|logs\|backup` | Remove a history entry, old log files, or excluded backup content. |

`--config <path>` points any command at a different configuration file, which is
read as-is and never rewritten when it already exists.

## Configuration

Configuration lives at `~/.goback.json` and is read with Viper. Backups are
organized under `profiles`; each profile has its own `source`, `destination`,
`hostname`, `rsync` settings, and optional `dailyCompanions`. The profile is
selected by matching `hostname`, or explicitly with `--profile`. If exactly one
profile is configured, it is used even when its hostname does not match. `--all`
selects every profile. A command that needs profiles fails with a clear error
when none are configured.

`dailyCompanions` belongs inside the profile it accompanies. A top-level
`dailyCompanions` key is rejected with instructions to move it; configuration
editing and printing remain available. For example, this entry belongs under
`profiles.default` alongside its `source`, `destination`, and `rsync` settings:

```json
"dailyCompanions": [
  {
    "id": "apple-photos",
    "command": ["photos-backup", "daily"],
    "dryRunArgs": ["--dry-run"]
  }
]
```

The mirror is configured separately, as a single top-level block:

```json
"mirror": {
  "source": "/Volumes/SanDisk/Media",
  "destination": "/Volumes/Elements/Media"
}
```

Both are full paths and both are required: when either is missing, `goback
mirror` names the missing key and stops. Nothing falls back to a compiled
default; the paths shown above are only what a generated configuration starts
with. Because the mirror belongs to no profile, `goback mirror` needs no profile
at all and rejects `--profile` and `--all` rather than ignoring them.

`goback config edit` uses the configured `editor`, falling back to `$EDITOR`
when unset or when the configuration is malformed and needs repair.

`goback config check` validates the entire selected configuration file, including
unknown keys, value types, companion definitions, and required settings for each
declared backup type and mirror. It works without mounted drives or a matching
hostname, never prompts or changes the file, and exits nonzero on errors. Use
`--config <path>` to check another file. Backup, preview, mirror, and backup-cleanup
commands also reject unknown keys and invalid value types before doing work;
configuration editing, printing, and history remain available for repairs.

## Profiles and backup status

Use `goback profiles` to see source/destination paths, configured backup types,
hostname matches, and which profiles a run without `--profile` or `--all` would
select. It works with offline drives and unmatched hostnames. `--profile NAME`
narrows the listing without changing the meaning of the auto-selected column.

`goback status` lists every configured daily, weekly, and monthly backup, daily
companion, and the global mirror, even when no history exists. Use `--profile
NAME` to narrow it; the global mirror is omitted when a profile is selected.
Use one of `--daily`, `--weekly`, `--monthly`, `--mirror`, or `--companions` to
select a backup type. Retired profile and companion history remains available
through `usage last --summary`.

```bash
goback profiles
goback status
goback status --older-than 48h
goback status --profile default --older-than 168h --json
goback status --daily --older-than 48h --check
goback status --companions --profile default
```

`--older-than` marks a last success as `stale` when it is older than the supplied
positive duration (`48h`, `168h`, etc.). Without a threshold, freshness is not
assessed. A recent failed attempt does not make an older success fresh. `never
run` means there is no retained attempt for that configured profile/type; history
trimming can also produce this label. A failed-only history has `no recorded
success`. Timestamps are interpreted in the current local timezone, matching
existing history. Status is informational by default: stale and never-run rows
do not change its exit code. Add `--check` to exit nonzero when any selected
backup's latest attempt failed or was interrupted, has no recorded success, or
is stale. An empty selection also fails a check. Without `--older-than`, a check
assesses recorded outcomes only, not age. Filters let daily and weekly backups
use different thresholds in separate checks. `--json --check` still emits its
JSON report on stdout when unhealthy; the error is written to stderr.

The table ends with the history database's path, marked `(no backups recorded
yet)` when it does not exist. `--json` emits one object: `generated_at`,
`db_path`, `db_exists`, and `backups`, an array with `profile`, `backup_type`,
`last_success`, `latest_attempt`, `exit_code`, `result`, and `freshness` per
row. Missing timestamps and exit codes are `null`. Times are RFC 3339 with the
current local offset (history stores local wall-clock times). Errors are a
single message on stderr without the usage text, so tools such as Heartbeat's
"Mac backups" row can show it as is.

```json
{
  "generated_at": "2026-10-08T17:49:08-06:00",
  "db_path": "/Users/me/.goback.db",
  "db_exists": true,
  "backups": [
    {"profile": "default", "backup_type": "daily", "last_success": "2026-10-06T11:21:24-06:00",
     "latest_attempt": "2026-10-06T11:21:24-06:00", "exit_code": 0, "result": "succeeded",
     "freshness": "not assessed"},
    {"profile": "default", "backup_type": "weekly", "last_success": null, "latest_attempt": null,
     "exit_code": null, "result": "never run", "freshness": "no recorded success"}
  ]
}
```

Both overview commands validate configuration without prompting or changing it,
and require no mounted drives. Status opens existing history read-only and never
creates or migrates a database. Existing `usage` commands still work without a
configuration file.

## Further documentation

Detailed guides live in [`docs/`](docs/):

- [Snapshot previews, dry runs, and history](docs/snapshots-and-history.md)
- [Failed-run diagnostics and file recovery](docs/diagnostics-and-recovery.md): includes recovering a single file.
- [Cleanup and terminal output](docs/cleanup-and-output.md)
- [`goback mirror`](docs/mirror.md): preflight, dry run and confirmation, capacity, interruptions, history.
- [Ejecting](docs/ejecting.md)

---

## TODO

- [ ] Add a way to run it as a service (e.g., `gocron`).
- [ ] Should allow specifying which source/destination pair to use at runtime for each backup type. Can just do an incremental daily backup for now.
