# Configuration and setup

[Back to the user guide](../README.md)

## Configuration

Settings live in `~/.config/osxphotos-backup/photos-backup.toml`. Pass
`--config PATH` before a command to use another file. Copy
[`photos-backup.example.toml`](../photos-backup.example.toml) as a starting point; it holds no secrets and none
should be added to it.

Pass `--volume PATH` before a command to send one run somewhere else without
editing the file, for instance when the usual drive is not connected:

```bash
photos-backup --volume /Volumes/T7 apple-photos --use-photokit
photos-backup --volume ~/Pictures/archive daily
```

The archive's sub-path under the configured volume is preserved, so a
`volume` of `/Volumes/SanDisk` and an `archive` of
`/Volumes/SanDisk/Media/Apple Photos` become `/Volumes/T7/Media/Apple Photos`.
The configuration file is still validated as usual. Only the Apple Photos
workflow is re-pointed: under `backup-all`, the `[ssd]` and `[rclone]` steps
keep reading their own configured sources.

Each section maps to one workflow and is loaded only by the commands that need
it: `[apple_photos]`, `[sd_card]`, `[ssd]`, and `[rclone]`. Paths accept `~` and
environment variables and must be absolute once expanded. Invalid values (wrong
type, out-of-range cadence or cleanup limit, an archive outside its volume, an
unknown key) fail with an error naming the file, section, and key.

Only the sections a command needs are read, so an Apple Photos export works on a
machine that has no SD card, SSD, or rclone configuration. `backup-all` and `ssd`
report a workflow whose section is absent as skipped instead of failing.
Pipeline summaries distinguish `Skipped by request` from
`Not configured: [section]`. Apple Photos is the exception: `backup-all` requires
`[apple_photos]` unless you pass `--skip-apple-photos` for a copy-only run.
Before `backup-all` starts any export or transfer, it validates every enabled
section and any configuration it depends on. Invalid configuration exits 2
immediately. `[rclone] remote` must be an rclone remote such as
`b2:my-photos-bucket` (or an on-the-fly `:backend:path`); a value without
`name:` is refused because rclone would treat it as a local path. Skipped
sections are not loaded unless another enabled step needs
them (for example, remote backup's default source uses `ssd.destination`).
After configuration validation, `backup-all` checks all required executables
before starting any step: ExifTool for a real Apple Photos export, rsync for
enabled SD-card/SSD copies, and rclone for an enabled remote copy. Missing tools
are listed together and exit 1. Skipped or unconfigured steps require no tools.
A dry run still needs rsync/rclone for transfer previews, but its planning-only
Apple Photos step needs no ExifTool executable.

After preflight, `backup-all` prints the enabled source → destination pairs and
whether each step can delete destination files. Local copies show the actual
target including the source directory name. This overview makes `--volume`
overrides visible alongside the independently configured SSD and remote sources.

SSD and SD-card copies check configured sources before creating the destination.
A missing source requires attention (exit 3), including a configured SD-card
backup directory. For copy source and destination paths under `/Volumes/<drive>`, the
drive must be mounted, even during previews; an absent mount is never created.
The check ignores letter case, so `/volumes/<drive>` is held to the same rule.
Remote uploads apply the same mount and source-directory checks before starting
rclone. Ordinary local paths remain supported without adding configuration keys.

SD-card and SSD copies also reject overlapping source and destination paths,
including symlink aliases, before creating directories or starting rsync. Paths
are compared without letter case, as on a default macOS volume. Each
source is copied into `destination/source-name`; SSD inputs must map to distinct,
non-overlapping directories. An omitted `exclude_file` is optional. An explicitly
configured file that is missing or is not a regular file prints a warning to
stderr, and ordinary copying continues without those exclusions, including in dry
runs. With SSD deletions enabled (`ssd --delete` or `backup-all --delete-ssd`), every
configured SSD and SD-card exclusion file must exist and be a regular file.
Otherwise the SSD step exits 3 before creating its destination or starting either
copy. The same refusal applies to previews. Restore the exclusion file or rerun
without `--delete`; omitting an exclusion file from configuration remains supported.
SSD and remote deletions also refuse (exit 3) a source that is empty apart from
Finder's `.DS_Store` and `._*` files; see [transfers](transfers.md).

When an SSD or remote source is the managed archive or a directory within it, the copy
holds the archive's read lock for the transfer. A running export or cleanup
blocks the copy with exit 3 before any destination is created; a copy already
in progress blocks new archive writers. Previews take the same read lock and
create nothing. An archive with missing or invalid lock metadata is refused.
Ordinary directory copies need no Apple Photos configuration. Configure
`ssd.source` or `rclone.source` as the archive itself: copying a parent directory
does not discover and lock archives nested inside it.

Transfer output adapts to stdout: terminals keep live progress; redirected output
keeps rsync statistics and rclone's one-line statistics at one-minute intervals,
plus errors and the final summary. SSD logs omit the verbose per-file listing;
rsync dry runs retain itemized proposed changes even when redirected.
The rclone log mode explicitly enables statistics at `NOTICE` level so they remain
visible without `--progress` ([rclone logging options](https://rclone.org/docs/#stats-log-level-string)).

Local export, SD-card, and SSD workflows print available destination filesystem
space after their path checks, including previews. For a destination not yet
created, this uses the nearest existing ancestor. This is informational: the tool
does not estimate incremental backup size or promise that the run will fit. If
space cannot be read, it prints that fact and continues. No remote capacity query
is performed.

### Checking setup

`photos-backup doctor` collects configuration, local path, archive-access, and
exclusion-file problems in one pass. It reports Python, application, and dependency
versions, plus paths and versions of tools needed by configured workflows. Version
queries have a five-second timeout. It creates no directories, receipts, or archive
state, does not load the Photos library, and does not contact cloud storage.
A missing optional section is skipped; an uninitialized archive is identified with
a bootstrap hint. This is setup diagnosis, not file verification or a guarantee
that a subsequent backup will succeed. Exit codes are 2 for configuration errors,
1 for tool/I/O failures, and 3 for paths or exclusions requiring attention, in that
priority order; otherwise 0. Configured but disconnected SD cards are reported.

Each check is `PASS`, `SKIP`, `ACTION`, or `FAIL`. `ACTION` marks something a
person can fix, such as connecting a drive or restoring an exclusion file; `FAIL`
marks broken configuration, missing tools, or I/O errors. A copy-layout check is
skipped while its own source or destination is unavailable, and the SSD copy of
the SD card folder is skipped until `sd-card` has created it. The export database
check reports its schema version and needs action when a newer osxphotos wrote it
than this installation pins. In a terminal, results appear as a table.
When `[rclone]` is configured, doctor also runs `rclone listremotes` (local
configuration only, never prompting for a password) and needs action if the
remote's name is missing. `doctor --json` prints the versions, every check, and
the exit code as one JSON document, with the same exit code.

