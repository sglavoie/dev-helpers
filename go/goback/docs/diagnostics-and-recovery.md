# goback: failed-run diagnostics and recovery

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

`PATH` is a relative path inside the selected backup directory, not inside the
live source. For example, `goback restore weekly Projects/notes.txt --to
"$recovery_dir"` reads `<destination>/weekly/Projects/notes.txt` and creates
`$recovery_dir/notes.txt`. Parent directories are not reproduced for a single
file. Absolute paths, `..` components, special files, and symlinks anywhere
inside the selected backup path are refused.

## Recovering a directory

Add `--recursive` to recover a folder and everything beneath it. For example:

```bash
recovery_dir="$(mktemp -d "${TMPDIR:-/tmp}/goback-recovery.XXXXXX")"
goback restore weekly Projects/my-app --to "$recovery_dir" --recursive --dry-run
goback restore weekly Projects/my-app --to "$recovery_dir" --recursive
open "$recovery_dir/my-app"
```

This reads `<destination>/weekly/Projects/my-app/` and creates
`$recovery_dir/my-app/`, preserving nested paths, hidden files, empty files,
and empty directories. Different files with the same basename stay in their
respective subdirectories. The original source can be offline or deleted.

Directory recovery requires `--recursive`; the flag also accepts a regular
file and keeps the usual single-file behavior. Only regular files and
directories are supported. A symlink or special file anywhere in the selected
tree stops the entire restore before copying, with the offending relative path
in the error. Nothing is silently skipped. Choose a smaller supported subtree
if the folder contains entries that cannot be recovered this way.

## Preview, verification, and overwrite protection

`--to` is required and must name an existing directory. Its resolved path must
neither be inside the profile's backup destination nor contain that destination.
Endpoints under `/Volumes/<name>` must be on mounted drives. Existing recovery
entries are never overwritten or merged, including empty directories and
dangling symlinks. Choose another recovery directory if an entry with the same
name already exists.

Every invocation shows the exact source, destination, total file bytes, file
count, and permissions. Directory previews also show a directory count,
including the selected directory itself.
`--dry-run` only inspects paths and metadata: it creates no files, directories,
locks, or history, and does not check file contents. A directory preview
inspects the entire selected tree, so it may take time for large folders.

A real restore needs no confirmation. It locks the backup and recovery
directories against overlapping goback operations and copies into a staging
file or directory created with private permissions. After copying, it rereads
every staged file and its backup to check their SHA-256 content against the
bytes copied. It also rechecks paths and backup metadata before publication.
The recovered file or entire directory becomes visible under its final name
only after all checks pass. A changed or unreadable backup, failed verification,
or interruption stops recovery and removes the staged copy. Cleanup errors are
reported.

Publication is atomic and refuses an entry that appeared during the restore.
Single-file recovery requires a filesystem supporting hard links (such as
APFS); the hard link publishes the staged copy and never links it to the backup.
Directory recovery requires exclusive rename support (available on APFS).
It never falls back to a rename that could replace an existing directory.
An uncatchable termination, such as SIGKILL or a power loss, may leave a hidden
`.goback-restore-*` staging file or directory in the recovery directory.

Restore preserves file data and structure, ordinary Unix permissions, and modification time
(to microsecond precision). It does not restore ownership, ACLs, extended
attributes, resource forks, special permission bits, or hard-link relationships.
Directory permissions and modification times are preserved too, including
read-only directories.
It does not run companions, eject drives, or create or update backup history.
Restoring therefore does not make `goback status` report a fresher backup.
Content verification establishes a match with the stored copy; it cannot prove
that the stored copy was free of earlier corruption.

## Inspecting a recovered file

If the original text file still exists, compare it:

```bash
diff -u "$HOME/Documents/notes.txt" "$recovery_dir/notes.txt"
```

Once you have inspected the recovered file, copy it back with an overwrite
confirmation:

```bash
cp -ip "$recovery_dir/notes.txt" "$HOME/Documents/notes.txt"
```

