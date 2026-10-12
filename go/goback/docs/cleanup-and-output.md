# goback: cleanup and terminal output

Part of the [goback README](../README.md).

## Cleanup and terminal output

```bash
goback clean backup daily --dry-run       # review candidates without deleting
goback clean backup daily                 # review, then confirm deletion
goback clean logs --keep 5 --dry-run       # preview log cleanup
goback clean logs --keep 5                 # retain the newest five failure logs
goback usage reset --profile default --keep 20
goback usage view --no-pager
goback usage view > backup-history.txt
```

Cleanup uses the same ordered include/exclude rules as a backup, including
inherited daily exclusions for weekly/monthly backups. Explicit includes take
precedence; an include list also excludes everything it does not select.
Filenames are preserved exactly and quoted in the cleanup list. A partial or
unrecognized rsync listing stops cleanup before confirmation. Real cleanup
reports deleted and failed entry counts and exits nonzero if any deletion fails.
Counts refer to listed roots; deleting a directory also removes its contents.

`clean logs` works without a configuration file or matching hostname. `--keep`
controls current failure logs in `~/.goback/logs/` (default 20, zero removes all).
The existing `--keep-daily`, `--keep-weekly`, and `--keep-monthly` flags apply only
to legacy logs directly in the home directory, retaining 14, 12, and 6 by default.
`--dry-run` previews both groups without deleting anything. Log cleanup covers
all profiles, so `--profile` and `--all` are rejected.

History reset applies both retention and deletion within `--profile` and any
backup-type selector. Without `--profile`, it covers all profiles; `--keep`
retains that many rows across the selected scope, with history ID breaking
same-timestamp ties. History commands work even without a configuration file,
and can filter profiles that are no longer configured. Shell completion also
works without configuration or a matching hostname. Profile names complete after
`--profile` (or `-p`), using `--config` when supplied. Missing or malformed
configuration produces no profile suggestions and is never changed.

The pager is automatically bypassed when input or output is not a terminal.
Use the global `--no-pager` flag to bypass it interactively as well. Run reports
and history tables omit terminal colors when redirected.

