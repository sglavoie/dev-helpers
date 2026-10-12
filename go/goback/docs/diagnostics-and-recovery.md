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

Copy the candidate into a new temporary directory first. For the quick-start
configuration, recovering `notes.txt` from daily looks like this:

```bash
recovery_dir="$(mktemp -d "${TMPDIR:-/tmp}/goback-recovery.XXXXXX")"
rsync -a --dry-run -- "/Volumes/Backup/Documents/daily/notes.txt" "$recovery_dir/"
rsync -a -- "/Volumes/Backup/Documents/daily/notes.txt" "$recovery_dir/"
open "$recovery_dir/notes.txt"
```

Substitute `weekly` or `monthly` to inspect another copy. If the original text
file still exists, compare it:

```bash
diff -u "$HOME/Documents/notes.txt" "$recovery_dir/notes.txt"
```

Once you have inspected the recovered file, copy it back with an overwrite
confirmation:

```bash
cp -ip "$recovery_dir/notes.txt" "$HOME/Documents/notes.txt"
```

