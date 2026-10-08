# SD-card, SSD, and cloud copies

[Back to the user guide](../README.md)

See [configuration and setup](configuration.md) for copy paths, exclusions,
mount checks, and locking. See [recorded status](status.md) for copy history
and freshness hints.

## SD card camera folders

Cameras store photos in numbered DCF folders such as `DCIM/100MSDCF` and start
the next one, `101MSDCF`, after 9,999 files. When `[sd_card] source` names one
of these folders, `sd-card` and `backup-all` still copy it but warn about any
sibling folder they leave behind, and `doctor` reports it as `ACTION`. Set the
source to the `DCIM` directory to copy every folder; the copy then lands at
`destination/DCIM` instead of `destination/100MSDCF`.

SD-card copies never overwrite a file that already exists at the destination
(`rsync --ignore-existing`). After a camera's file counter resets, a new
`DSC00001.ARW` is skipped instead of replacing the archived original; move or
rename the archived files first if you want the new ones copied beside them.

## Progress, previews, and failures

SSD, SD-card, and remote transfers display progress while they run. Failure
summaries include a short diagnostic excerpt from the retained output tail,
preferring explicit error lines when available. Full streamed output remains
available in the terminal or redirected log. Their
`--dry-run` options invoke rsync/rclone in preview mode and create no destination
directories. Paths and exclude files may contain spaces or quotes. A failed
standalone `remote` command exits 1. Transfer preview summaries say `DRY RUN`
and label counts as proposed transfers; Apple Photos uses `PLANNED` because
its exporter is not invoked. A successful pipeline preview ends with
`PREVIEW COMPLETE`. `backup-all` exits 3 for action-required steps, such as a
blocked Apple Photos takeover or unavailable SSD source, unless another step
fails, in which case it exits 1.
If every pipeline step is skipped, the summary says
`NOTHING TO DO — all steps skipped` and exits 0, including in dry runs.

The Apple Photos step of `backup-all` shows the same live export phases and
per-asset download budget as `daily`. Use `backup-all --download-timeout 300`
to allow five minutes per missing asset across retries; this does not limit the
whole pipeline. Skipping Apple Photos also skips its progress display, and
`--dry-run` remains planning-only for that step.

SSD summaries retain each completed copy if a later copy fails, in both `ssd`
and `backup-all`. The failed copy is named, and any remaining SSD copy is
reported as skipped because the previous copy did not complete. SSD source
and destination checks still run before any copying starts.

If an SSD step fails or requires action, `backup-all` skips remote backup when
its source overlaps the SSD destination (including symlink aliases, subdirectories,
and parent directories). An independent remote source can still run. Explicitly
using `--skip-ssd` allows uploading the existing SSD backup. These rules also
apply to dry runs; the SSD error still determines the pipeline exit code.

Transfer summaries explicitly show zero files or zero proposed transfers when
reported by the transfer tool. If statistics are unavailable, the summary says
so instead of implying that no files needed copying.

## Remote backup (rclone)

The `remote` command copies your backup to a cloud storage provider using
[rclone](https://rclone.org/).

**Behavior change:** remote backup now uses `rclone copy` by default, preserving
files that exist only at the destination. To retain the previous deletion-mirroring
behavior, use `photos-backup remote --delete` (which uses `rclone sync`). Review
the effect first with `photos-backup remote --delete --dry-run`.

For the full pipeline, `backup-all --delete-ssd` (or its older spelling
`--delete`) enables SSD deletions only. Use `backup-all --delete-remote` to enable cloud deletions, or pass both
flags to mirror deletions at both destinations. Neither flag changes Apple Photos
archive cleanup approval rules. Update any scripts that relied on automatic remote
deletions to pass the appropriate flag.

1. Install rclone: `brew install rclone` or see https://rclone.org/install/
2. Configure a remote: `rclone config`
3. Set `remote` in the `[rclone]` section (e.g. `b2:my-photos-bucket`)
4. Optionally set `source` (defaults to `ssd.destination`)
