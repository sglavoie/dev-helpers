# Recorded status and transfer history

[Back to the user guide](../README.md)

## Compact status

`photos-backup status --short` shows one line per configured backup stage,
pending cleanup, a small download-failure sample, the last recorded verification,
and suggested next commands. Historical transfer destinations are omitted. Retry
commands preserve `--config` and `--volume`; mirror retries are previews using
`--delete --dry-run`. Review each result before continuing to the next command.
Unknown freshness is labeled explicitly. A successful copy with no newer recorded
upstream success is not proof that its files are current. The compact view retains
normal status exit codes. `--short` and `--json` are mutually exclusive.

`photos-backup status --check` prints the same compact view and exits 3 when it
suggests any command, so a scheduled `status --check --notify` tells you when
something needs attention. An unavailable archive also exits 3; while it is
unavailable, `verify --record` is not suggested because it could not run.
It also suggests `photos-backup daily` once the last full export is
`full_export_max_age_days` old or older: a full export on cadence never gets that
old, so this catches a schedule that has quietly stopped running `daily`.

## Latest download failures

Both text views summarize the latest incomplete export's `.downloads.json` report:
asset count, counts grouped as "timed out" or "retrieval error", and up to three
filenames with their reasons. An asset with any timeout reason is counted in the
timeout group. This is a recorded sample, not a fresh retry or the full set of
missing export components. The CSV remains the export's authoritative report.
Only the report named in the latest export attempt is read; old reports are never
used as a fallback. Missing reports are normal for interrupted runs and produce no
sample. Malformed, unreadable, or symlinked reports produce a warning without
changing status's exit code. Successful attempts do not show old download failures.

`status --json` adds `download_summary` (count, grouped reasons, up to three
samples, and report path) and `download_report_error`, both nullable. Text warnings
are represented in JSON fields so stdout remains one JSON document.

## Additional freshness hints

Current SSD SD-card routes now compare the last successful SD-card import's
completion with the SSD copy's start. The import destination must lie within the
SSD source using lexical paths. `sd_imported_since_copy` is true for a newer
import, false for an import no newer than the copy's start, and null when no
comparison is available. Imports from historical routes do not contribute.

An incomplete export can retain new files without advancing the successful export
time. Direct archive copies therefore also expose
`archive_may_have_changed_since_copy`: it compares the latest failed/interrupted
attempt's completion (or start when completion is missing) with the successful
copy's start. This is deliberately a possible-change hint; the attempt might have
failed before exporting anything. Unknown or inapplicable comparisons are null.
Neither hint probes copy drives, scans media, or changes cadence or cleanup rules.

## Recorded verification

Use `photos-backup verify --record` to remember a result on this Mac. Status reads
it even when the archive is unavailable and shows its age and failed check names.
It notes successful exports or possible incomplete-export changes since the
verification started. When archive state cannot be read, freshness is unknown.
The comparison uses the scan's start to avoid claiming it covered a later export.
Cleanup and other external file changes are not inferred from export timestamps.

JSON adds `last_verification` and `verification_history_error`. The receipt exposes
`archive_exported_since_verification` and
`archive_may_have_changed_since_verification` as nullable time comparisons, plus
`archive_state_available`. `files_verified` remains false: status does not verify
files. See [verification](verification.md) for storage and recording behavior.

## Checking recorded status

`photos-backup status` shows the archive writer, initialization and export
timestamps, last successful baseline report, last archive cleanup reconciliation, pending
cleanup, and the next export mode with its cadence reason. When the next export
is incremental, it also shows when the next full export is due: the configured
weekday or `full_export_max_age_days`, whichever comes first; `--short` shows
its date beside the mode. Timestamps include relative ages; pending
cleanup includes commands to approve or discard it. It reads state without
scanning exported files, opening the Photos library, taking over ownership, or
writing anything.
Cleanup reconciliation is separate from SSD and cloud copying; its timestamp
does not indicate when either secondary backup last succeeded. The secondary
transfer history below it records those copies separately.
An uninitialized archive suggests `photos-backup bootstrap`.
The baseline report belongs to the last successful full or incremental export;
it does not describe newer failed, recent, or custom manual exports.

Status also shows the latest Apple Photos export attempt: mode, outcome, times,
Mac, report path (if the exporter wrote it), error, and whether it advanced the
baseline. This receipt travels with the archive and includes failed, interrupted,
recent, and custom exports without changing cadence or cleanup decisions.
New receipts also record missing-file and export-error counts when a usable
export report exists. A failed or interrupted attempt shows the download report
path (if written) and a copyable retry command when the original options are
known. Recent retries retain `--days` and the download timeout; their rolling
date window is recalculated when rerun. Pipeline export retries skip the SD-card,
SSD, and remote steps. Custom/limited manual exports ask you to reuse the original
options instead of guessing. Older receipts remain readable without these fields.
It describes the export step, not subsequent bootstrap coverage or cleanup.
Attempts start when the exporter is invoked; earlier configuration, mount, and
takeover refusals do not replace the receipt. A process killed before it records
completion leaves an explicit "completion not recorded (running or interrupted)"
status. Dry runs never update it, and existing archives show "not recorded" until
their next real export. Receipt write failures warn without masking export results;
an unreadable receipt is reported without repairing it or hiding baseline state.

Status retains the last successful export time across subsequent failed or
interrupted exports, including successful recent and custom runs. Older receipts
fall back to the success they still record and the baseline; lost history cannot
be reconstructed. Direct archive copies get a recorded-time freshness hint.
Cloud routes also show when a currently configured SSD copy into their source
completed after the last successful upload started. Matching is lexical and never
probes disconnected copy drives; symlink aliases are not inferred. These hints do
not verify destination contents. Current transfer routes whose latest attempt
failed, was interrupted, or never recorded completion show a copyable retry
command, and routes made stale by a newer upstream run show an update command,
in both detailed and `--short` status; mirrors suggest `--delete --dry-run`
first. An SSD retry runs its configured SSD routes together. Historical routes
have no retry hint.
When a route's source or destination is under a `/Volumes` drive that is not
mounted, its suggestion ends with a shell comment such as
`# connect /Volumes/Data first`, and its `configured_transfers` row in JSON
carries that drive as `disconnected_drive` (null otherwise). Only the mount point
is checked; status still never reads the copies themselves.

Transfer history records the mode of both the latest attempt and the last
successful copy: `copy` preserves destination-only files, while `mirror` enables
deletions subject to the transfer's exclusions. A failed mirror therefore does
not relabel an earlier successful copy. Older receipts remain readable and show
"mode not recorded"; the current configuration is never used to guess their mode.

Suggested recovery and cleanup commands preserve any explicit `--config` and
`--volume` options and quote paths for pasting into a shell, so they target the
same archive as the command that printed them.

Status exits 0 when state can be read, even if bootstrap or cleanup is pending;
it is an informational view, not a health check. Use `verify` to check the files.
When the archive is unavailable, locked, or its state cannot be read, status still
shows local transfer history alongside an archive error. It preserves the archive
error's nonzero exit code; it does not infer archive state from transfer receipts.
When `[apple_photos]` is absent, status shows that archive status is not configured,
prints local transfer history, and exits 0. The configuration file must still exist
and contain valid TOML. Status validates all configured workflow sections and
remote-source dependencies; invalid configuration exits 2.

`status --json` prints one versioned JSON document with `archive`, `archive_configured`, `observed_at`,
`files_verified` (always false), `state`, `next_export`, `archive_error`, `transfers`, and
`transfer_history_errors`. Dates are ISO 8601 strings with timezones; missing
state values are null. It preserves the normal status exit codes, including exit
0 for pending cleanup or an uninitialized archive. Errors reading local receipts
are included in `transfer_history_errors` rather than mixed into JSON output.
The additive `last_export_attempt` and `export_attempt_error` fields contain the
latest export receipt and any read error, respectively, or null when unavailable.
Transfer attempts expose `mode` when recorded; older attempts may omit it.
`next_export` holds the `mode`, its cadence `reason`, `full_due_at` (when the
next full export is due, or null when the next export is already full), and
`overdue`, which is true when the last full export is older than
`full_export_max_age_days`.
If the archive cannot be read, `state` and `next_export` are null and
`archive_error` contains the reason; otherwise `archive_error` is null. The JSON
is still printed on archive errors, with CLI diagnostics on stderr.
For copy-only configurations, `archive_configured` is false and `archive`, `state`,
`next_export`, and `archive_error` are null.
The additive `configured_transfers` field lists every current transfer route,
including routes with no receipt (`last_attempt` and `last_success` are null).
`historical_transfers` contains receipts whose step, source, or destination no
longer matches the configuration. `transfers` still contains all recorded receipts.
Each configured route also has `archive_exported_since_copy`: true when a known
successful archive export is newer than the last successful copy's start, false
when the compared export is no newer, or null when comparison is unavailable.
This uses the baseline export timestamp and, when successful, the latest export
attempt's completion (including recent/custom exports). Only an exact configured
archive source is compared; parent directories, descendants, symlink aliases,
and indirect SSD-to-cloud routes are not inferred. The text view highlights true
values. This is a recorded-time hint, not destination verification or a guarantee
that a false value means everything is current. Status still performs no file scan.

## Transfer history

The text view starts with an attention summary when latest attempts failed or were
interrupted, completion was not recorded, or a copy has never succeeded. An attempt
without a recorded completion may still be running. These informational counts can
overlap and do not change the status exit code. Attention counts cover current
configured routes only, including those with no successful copy recorded.

Real `sd-card`, `ssd`, `remote`, and `backup-all` copies record local receipts under
`~/.local/state/photos-backup/transfers/` (or
`$XDG_STATE_HOME/photos-backup/transfers/` when that variable is an absolute path).
Each named configuration file has separate history, with one receipt per step,
source, and destination. SSD and SD-card destinations include the source directory
name. SD-card receipts identify the configured source path, not a physical card.
Status shows the last attempt and last successful copy with relative ages, plus
available file count, size, and duration from the successful copy for each route;
routes with no receipt explicitly say no successful copy is recorded. When
configuration changes, older routes appear separately under historical destinations.
Matching uses the configured paths without probing copy drives, so an unmounted
destination still appears. `--volume` continues to affect only Apple Photos.

A failed or interrupted transfer does not erase the previous success. An attempt
whose completion was never recorded is labeled as running or interrupted, never
successful. Receipts retain counts, size, duration, and any transfer error. Skipped
steps and dry runs do not update history. SSD path, lock, exclusion, and destination
preparation failures record a failed preflight attempt for each configured SSD
route while preserving its previous success. Pipeline configuration and executable
checks still run before any workflow is attempted. History begins when receipts
are enabled for each workflow;
older transfers and transfers from another Mac cannot be inferred.

Receipts are replaced atomically under a local lock. History write failures warn
without changing the transfer's result. Reading status reports malformed receipts
without modifying them. On the next real transfer, an invalid receipt is preserved
under a unique `.corrupt-…` filename beside the original, and a fresh receipt starts
with no previous success. If preservation fails, the invalid receipt stays in place
and recording warns without changing the transfer result. Reading status creates
nothing. These records describe completed commands,
not verification of the destination files, and never modify archive writer state.
