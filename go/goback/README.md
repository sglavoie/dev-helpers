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

## `goback mirror`

`goback mirror` makes the destination an exact copy of the source. It has
nothing to do with the `daily`/`weekly`/`monthly` snapshot layout, it never
touches a profile, and it never ejects a drive.

The rsync policy is fixed and not configurable:

- `--archive --hard-links --acls --xattrs --crtimes`: hidden files, symlinks,
  hard links, ACLs, extended attributes, and creation times are all reproduced.
  An rsync built without those capabilities is refused rather than silently
  degraded, so the rsync shipped with macOS does not work; install rsync 3.
- `--delete --delete-delay`: content the destination holds and the source does
  not is **deleted**, but only after the transfer finished. rsync's own
  protection still applies, so a transfer that hit read errors deletes nothing.
- Size and modification time decide what to copy; there is no `--checksum`.
- The source is passed with a trailing slash, so its *contents* land in the
  destination and no extra directory level appears.

Refusals that happen before anything is written:

- The source is missing, unreadable, empty, or not a directory.
- The source and the destination are the same path, one contains the other, or
  they are on the same filesystem.
- A `/Volumes/<name>` endpoint whose mount point is a leftover directory on the
  system disk rather than a mounted drive.
- The destination does not exist: the mirror never creates it, so a missing
  destination has to be created by hand before mirroring.
- The destination is not a directory, or is not writable.
- The destination does not have enough free space (see below).

### Preflight, dry run, and confirmation

Every invocation begins with the same non-writing preflight, which is exactly
what `--dry-run` prints: the source, the destination, the command, the number of
entries that would be created, updated, and deleted, the size of the transfer,
the free space on the destination, and **every path that would be deleted**.
`--dry-run` stops there.

A real mirror then asks for a confirmation that defaults to No, and asking is
not optional: `confirmExec` governs snapshot backups and is not read here. After
the answer, both endpoints are bound (see below) and everything the preflight
measured is measured again *through those bindings*, so the second plan and the
deletions it lists are about the very directories the transfer acts on rather
than about whatever the configured paths hold while rsync reads them. The
transfer runs against that second plan. The mirror refuses to write when:

- the source or destination is no longer on the filesystem it was on, which is
  what catches a drive unplugged, remounted, or swapped while the prompt waited;
- the destination names another directory than it did when the mirror took its
  lock, which is what catches a symlinked destination repointed inside the same
  drive while the prompt waited: the transfer would write into a directory this
  mirror never locked and another mirror may be writing to;
- the destination is no longer the same directory it was when the mirror took
  its lock, even though the path and the drive are unchanged, which is what
  catches one ordinary directory moved onto the destination in place of another;
- the destination gained content the approved plan never listed, so nothing is
  deleted without having been reviewed;
- the destination gained anything at all after that second plan was computed,
  since the transfer is a separate rsync process that scans the destination once
  more on its own and would delete whatever it finds there;
- an endpoint no longer resolves where it did when the second plan was
  validated, which is what catches a source leaf or a destination leaf replaced
  by a symlink pointing off the drive.

The mirror is bound to the directories, not to their paths. Both endpoints are
opened right after the confirmation, and everything that follows — the second
preflight, the two listings of the destination, and rsync itself — is given the
identity of each opened directory (`/.vol/<volume>/<inode>`) instead of the path
it was found at. For the second preflight and transfer, the rsync child starts
inside the destination's bound directory and receives `.` as its destination;
the source argument remains its bound identity path. This supports rsync 3.5's
destination checks, which cannot walk the intermediate `/.vol/<volume>` path.
Command history includes this working-directory context. rsync opens its own
arguments when it starts, which is later
than the last thing the mirror can check, so this is what makes a replacement in
that window harmless: renaming the source or destination away and putting a
symlink in its place afterwards changes what the path means and nothing else,
and the transfer still reads from, writes into, and deletes inside the
directories that were reviewed. It is also what keeps the second preflight
honest, since a directory that stands in for an endpoint only while rsync scans
it would otherwise make the plan describe one directory while the transfer acts
on another.

The binding lands on the directories the approved preflight inspected or on
nothing at all. That preflight records what each endpoint *is*, not only where it
is, and the open has to find exactly that directory. A path is not proof: an
ordinary directory moved onto an endpoint keeps the path resolving where it did
and keeps the drive it is on, so a substitution that lasts only for the instant
of the binding would otherwise be handed to rsync through its own retained
identity while every check before and after it saw the validated endpoint. A
volume that cannot name its directories by identity is refused too, since nothing
could be guaranteed about a transfer to it.

The mirror never creates its destination. A destination that does not exist is
refused by the preflight, with an error saying to create it first, and nothing
is asked, locked, or run. macOS has no call that creates a directory and hands
back the directory it created, only a name to look up afterwards, and nothing
the kernel offers distinguishes the directory this process made from one another
writer put at that name in the meantime. Every directory the mirror writes into
is therefore one whose identity was recorded by the preflight, before anything
was decided, and the binding is what holds the transfer to it.

The transfer is also given an explicit deletion boundary. rsync scans the
destination itself, later than any listing the mirror made, so it is handed the
list of paths the mirror did see and is allowed to delete those and nothing
else. Anything another program writes into the destination between the last
listing and the transfer is therefore left alone rather than deleted as content
nobody reviewed; the next mirror lists it, shows it, and asks.

When the second plan finds nothing left to do, the mirror stops as up to date.

Only one mirror of a destination runs at a time. The destination is locked
before the confirmation is asked and stays locked until the transfer is over, so
a second `goback mirror` of the same destination on this machine is refused
outright instead of queueing behind a prompt nobody may be watching. The lock
names the destination after every symlink has been followed, so two configured
paths that are different names for one directory exclude each other, and a
destination that comes to name another directory during the confirmation, or
that stops being the directory it was, is refused rather than transferred under
the lock of the directory it used to name. What rsync is bound to is therefore
always the directory the held lock was taken for. Mirrors of non-overlapping
destinations are independent. The same lock protocol also excludes snapshot
runs and cleanup operations whose destinations overlap the mirror's.

### Capacity

Free space is measured on the destination before the run and compared with the
size of the transfer. Deletions happen after the transfer, so the space they
free is not counted; a mirror that would need more than is currently free is
refused before the confirmation is asked.

### Interruptions and partial transfers

Partial data is kept in a hidden `.goback-partial` directory inside the
destination, and the next mirror resumes from it. That directory is excluded
from the transfer and never deleted for being absent from the source.

`Ctrl-C` terminates rsync rather than leaving it running: no deletion is
performed, the destination is left partially updated, and the run is recorded
with exit code `-1`. A transfer that fails leaves the destination's extra
content in place too.

### History

An attempted mirror is one row in the backup history with backup type `mirror`
and profile `global`. Only a run that actually started rsync is an attempt, so a
dry run, a failed preflight, a declined confirmation, an already-matching
destination, and an interruption before the transfer record nothing. Because the
mirror belongs to no profile, filtering history by profile never shows it:

```bash
goback usage view --mirror     # mirror rows only
goback usage reset --mirror    # trim mirror rows only
```

## Ejecting

`goback eject` unmounts the volume holding the active profile's destination.

`goback eject --all` unmounts every external volume the configuration points at:
each profile's source and destination plus the mirror's endpoints, restricted to
paths under `/Volumes` and unmounted once each, in mount-point order. It needs no
profile, so it also works on a machine whose hostname matches none. A volume that
is not mounted is skipped rather than failed, one volume refusing never stops the
others, and the command exits nonzero only when a mounted volume could not be
ejected.

`goback eject --volume NAME` unmounts exactly one configured volume, named
either `Elements` or `/Volumes/Elements`, whichever profile or mirror references
it. It needs no profile, so it is how a drive only the mirror uses is ejected on
its own. A volume the configuration does not reference is refused with the list
of configured ones, a volume that is not mounted is skipped without failing, and
`--volume` cannot be combined with `--profile`, `--all`, or `--list`.

`goback eject --list` ejects nothing. It prints every configured volume that is
currently mounted, each once and in mount-point order, with every profile or
mirror path that references it and the commands that would eject it:

```text
$ goback eject --list
MOUNT POINT        REFERENCED BY                                   EJECT WITH
/Volumes/Elements  mirror destination (/Volumes/Elements/Media)    goback eject --volume Elements
/Volumes/SanDisk   profile default destination (/Volumes/SanDisk)  goback eject --profile default
                   mirror source (/Volumes/SanDisk/Media)

goback eject --all ejects every configured volume that is mounted, whichever profile or mirror references it.
```

`goback eject --profile NAME` only ever unmounts the volume holding that
profile's destination, so it is suggested only for destinations; a volume
referenced solely as a source or by the mirror gets `goback eject --volume NAME`
instead. The listing always covers every profile, needs no hostname
match, rejects `--profile`, and treats `--all` as redundant. An explicit
`--config` is carried into the suggested commands, shell-quoted. When nothing
qualifies it prints `No configured volumes are currently mounted.` and succeeds.

A volume counts as mounted only when `/Volumes/<name>` is a real directory, not a
symlink, on a different filesystem device than `/Volumes` itself, so a leftover
empty directory is neither listed nor ejected. Being listed does not guarantee
the eject succeeds: a busy drive can still refuse.

Ejection is always explicit: `goback mirror` ignores `ejectOnExit` and unmounts
nothing, whatever its outcome.

---

## TODO

- [ ] Add a way to run it as a service (e.g., `gocron`).
- [ ] Should allow specifying which source/destination pair to use at runtime for each backup type. Can just do an incremental daily backup for now.
