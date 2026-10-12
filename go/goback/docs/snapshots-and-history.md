# goback: snapshot previews, dry runs, and history

Part of the [goback README](../README.md).

## Snapshot previews, dry runs, and history

```bash
goback preview daily          # print the command and configured companions
goback run daily --dry-run    # ask rsync what would change
goback run daily              # perform the backup
goback usage last --summary   # last success and latest attempt per profile/type
```

Preview checks required configuration settings without requiring mounted drives;
an unconfigured backup type produces an error. It never creates a backup directory
or ejects a drive. Snapshot dry runs, from either `--dry-run` or the profile's
`rsync.<type>.dryRun`, create no backup
directories, record no backup history, and never automatically eject a drive.
Companions run only with their configured `dryRunArgs`; those without them are
skipped during a dry run. Both forms of snapshot dry run list individual changes
and show transfer statistics; `--quiet` suppresses statistics but keeps the
change list. The result table labels dry runs explicitly for both the main
backup and companions, including companions skipped for lack of `dryRunArgs`.

A real snapshot creates its destination directory only after validation and
confirmation. Paths and include/exclude patterns are passed literally to rsync,
so spaces, apostrophes, and shell characters are preserved. Source/destination
overlap checks resolve symlinks and compare directory boundaries.
Snapshot runs, including dry runs, require endpoints under `/Volumes/<name>`
to be on mounted drives; leftover directories and symlinks escaping the named
volume are refused. These checks run again after confirmation. Weekly and
monthly check their actual source, `daily/`, so the original source can remain
offline. Command previews do not require mounted drives.

Weekly and monthly previews and run confirmations show daily's last recorded
success with its age. If the latest daily attempt failed or was interrupted,
they warn that `daily/` may be partially updated. This describes retained history,
not an inspection of file contents; missing or unreadable history is reported
without preventing the copy. A new weekly/monthly success does not establish
that the daily source contained recent data.

Confirmation prompts accept `y`/`Y` for Yes and `n`/`N` or Escape for No.
Arrow keys and Enter still select the displayed answer; mirror confirmation
continues to default to No.

Real snapshot runs lock the entire profile destination before confirmation.
`run all` retains that lock through daily companions and the weekly/monthly
copies, so daily cannot be changed by another goback operation while it is being
copied. Cleanup uses the same lock, and mirrors coordinate with it when their
destinations overlap. A competing operation fails immediately. Symlink aliases
of the same root share a lock; snapshot and cleanup paths that resolve outside
their locked profile root are refused. Locks coordinate processes on this
machine using the same temporary directory; they do not exclude other programs
or another computer. Dry runs do not create lock files.

Declining a step in `run all` skips the remaining steps for that profile and
explains the skips in the result table. Other selected profiles still run.
Independent `run weekly` and `run monthly` commands continue to use the existing
daily backup.

Use `goback preview daily --test-pattern '*.tmp'` to inspect one exclude pattern,
or `goback preview daily --excluded` for the configured filters. Both show exact
excluded paths: a match for `Documents/scratch.tmp` does not label all of
`Documents/` as excluded. Fully excluded directories appear once with a trailing
slash. `--depth` must be nonnegative; zero means unlimited depth.
`--subdir Documents` limits the displayed exclusions to that directory while
evaluating filters from the original backup source, so anchored patterns such as
`/Documents/private.txt` keep the same meaning as in a backup. Displayed paths
remain relative to the original source. Absolute subdirectory paths must stay
inside that source. With `--subdir`, depth is measured from the selected directory;
the scan still starts at the original source to preserve parent exclusions.

`ejectOnExit` applies only to snapshot runs whose requested transfers and
companions all succeeded, with no dry runs or declined steps. Preview and
cleanup commands never automatically eject drives.

`usage last --summary` shows one row for each profile and backup type found in
history, including companions and the global mirror. Each row shows the last
successful backup, the latest attempt, and its result (including a failure's
exit code). A failed attempt does not replace the last successful backup.
If history cannot be opened or written, goback prints a warning and continues
with companions and the result report; the transfer outcome is preserved.
`Never recorded` means no success remains in the stored history. Use
`--profile NAME` to narrow the report. Without `--summary`, `--entries` applies
to each profile/type pair. The summary includes relative ages alongside exact timestamps and explanations
for common rsync failures. Companion exit codes retain their own meaning.
Timestamps in existing history have no timezone; ages use the current local
timezone. Attempts with identical timestamps are ordered by history ID. Interrupted snapshot transfers are recorded with exit code `-1`.

For scripts, `goback usage last --summary --json` emits one JSON array, ordered
by profile and backup type. It honors `--profile` and returns `[]` for no history.
Each row contains `profile`, `backup_type`, `latest_attempt`, `exit_code`, and
`last_success` (`null` when no successful attempt remains). Timestamps retain
the stored `YYYY-MM-DD HH:MM:SS` format without timezone information; no conversion
to UTC is performed.
A failed latest attempt leaves an earlier `last_success` intact. `--json`
requires `--summary` and emits no table or terminal colors.

