# goback: failed-run diagnostics and file recovery

Part of the [goback README](../README.md).

## Failed-run diagnostics

Failed or interrupted snapshot transfers, mirror transfers, and daily companions
save the last 64 KiB of combined
stdout/stderr in `~/.goback/logs/`. The result report prints the full log path.
Each log includes the profile, backup type, exit code, and executed command;
filenames include the UTC timestamp. Files are private (mode 0600). Only the
newest 20 failure logs are retained across all profiles and backup types,
separately from SQLite history. Successes and dry runs create no diagnostic logs.
Mirror preflight failures and companions that never start create no log.
Failure to save a log produces a warning and preserves the original result.
Companion exit codes retain their own meaning and are not interpreted as rsync
errors; interruptions are recorded with exit code `-1`.

```bash
ls -lt ~/.goback/logs/
less ~/.goback/logs/failure-REPLACE-WITH-THE-REPORTED-NAME.log
```

## Recovering a file

Use `goback profiles` to find the destination and `goback usage last --summary`
to check when daily, weekly, or monthly last succeeded. Look for the desired
file inside the corresponding directory. These are maintained copies, so a
previous version exists only if one of those copies still contains it.

Restore the candidate into a separate recovery directory first. For the
quick-start configuration, recovering `notes.txt` from daily looks like this:

```bash
recovery_dir="$(mktemp -d "${TMPDIR:-/tmp}/goback-recovery.XXXXXX")"
goback restore daily notes.txt --to "$recovery_dir" --dry-run
goback restore daily notes.txt --to "$recovery_dir"
open "$recovery_dir/notes.txt"
```

Substitute `weekly` or `monthly` to inspect another copy. Use `--profile NAME`
to select a profile; if automatic selection would include multiple profiles,
restore requires an explicit choice. `--all` is rejected. A retained copy is
recoverable even when its backup type is no longer configured for new runs.
The original source can be offline or deleted.

`FILE` is a relative path inside the selected backup directory, not inside the
live source. For example, `goback restore weekly Projects/notes.txt --to
"$recovery_dir"` reads `<destination>/weekly/Projects/notes.txt` and creates
`$recovery_dir/notes.txt`. Parent directories are not reproduced. Absolute paths,
`..` components, directories, special files, and symlinks anywhere inside the
selected backup path are refused.

`--to` is required and must name an existing directory. Its resolved path must
neither be inside the profile's backup destination nor contain that destination.
Endpoints under `/Volumes/<name>` must be on mounted drives. Existing recovery
entries are never overwritten, including dangling symlinks. Choose another
directory if a file with the same name already exists.

Every invocation shows the exact source, destination, size, and permissions.
`--dry-run` only inspects paths and metadata: it creates no files, directories,
locks, or history, and does not check file contents. A real restore needs no
confirmation. It locks the backup and recovery directories against overlapping
goback operations, copies into a private staging file, then rereads both the
staged file and backup to check their SHA-256 content against the bytes copied.
The recovered file becomes visible under its final filename only after those
checks pass. Publication is atomic and refuses an entry that appeared during the
restore. This requires a recovery filesystem supporting hard links (such as
APFS); a hard link is used only to publish the verified staged copy, never to
link it to the backup. Handled errors and interruptions remove the staging file.
An uncatchable termination, such as SIGKILL or a power loss, may leave a hidden
`.goback-restore-*` staging file in the recovery directory.

Restore preserves file data, ordinary Unix permissions, and modification time
(to microsecond precision). It does not restore ownership, ACLs, extended
attributes, resource forks, special permission bits, or hard-link relationships.
It does not run companions, eject drives, or create or update backup history.
Restoring therefore does not make `goback status` report a fresher backup.
Content verification establishes a match with the stored copy; it cannot prove
that the stored copy was free of earlier corruption.

If the original text file still exists, compare it:

```bash
diff -u "$HOME/Documents/notes.txt" "$recovery_dir/notes.txt"
```

Once you have inspected the recovered file, copy it back with an overwrite
confirmation:

```bash
cp -ip "$recovery_dir/notes.txt" "$HOME/Documents/notes.txt"
```

