# Apple Photos exports

[Back to the user guide](../README.md)

## Bootstrapping an archive

`photos-backup bootstrap` builds the archive from the Photos library alone;
nothing is imported from `apple_photos.legacy_export`, so a partial local export
never becomes the baseline. It claims the archive for this Mac using the same
takeover check as `daily`, runs one full export, and initializes the archive only
when that export exits cleanly, downloads every asset from iCloud, and leaves an
export-database record for every asset in the library. Anything short of that
prints the reason and exits 1 without writing `initialized_at`.

Because initialization is the last step, an interrupted bootstrap is resumed by
running the command again: the second run sees the export database it left
behind, runs another full export, and picks up where the first stopped.
Bootstrap accepts only an empty archive or one it left incomplete itself. An
archive that holds other content but no `.photos-backup` metadata is refused
with exit 3, naming what it found, rather than exporting into someone else's
directory. Finder clutter (`.DS_Store`, `.localized`, AppleDouble `._` files)
does not count as content, and an archive that cannot be listed is refused
rather than treated as empty.

To leave the Hidden album locked and omit its photos/videos from backups, set
`exclude_hidden = true` in `[apple_photos]` in your configuration. This applies
to bootstrap, daily, recent, and manual exports. Bootstrap reports the excluded
count and checks completeness only for non-hidden assets. The default is
`false`, which includes hidden assets. Existing archived copies are retained;
hidden assets remain visible to library identity and deletion checks, so hiding
an asset does not make it a deletion candidate. Use the same setting on each
Mac sharing the archive. If you later turn this off, older hidden assets will
be included in the next full export.

A `bootstrap --dry-run` validates the archive and prints the full export plan,
but does not invoke osxphotos or scan the Photos library for coverage. Coverage
is checked after the real export. It writes nothing and never initializes.

## Daily export

`photos-backup daily` exports Apple Photos straight into the archive; nothing is
staged anywhere else. It exits 3 and does nothing when the archive has not been
initialized, 1 when the export reported errors, and 0 otherwise.

Every run uses the same osxphotos arguments so two machines produce the same
layout: `--update --update-errors` with `--download-missing --use-photokit` and
`--retry 3`, `--exiftool`, `--export-aae`, `--sidecar json`, originals, edited
versions, Live Photos, RAW files and bursts all kept, `--directory
'{created.year}/{created.mm}'`, `--filename
'{created.strftime,%Y-%m-%d-%H%M%S}_{original_name}'`, `--album-keyword` so
album names travel as metadata instead of directories, `--exportdb` pointing at
`.photos-backup/export.db`, and `--report` pointing at the per-host report.
`--cleanup` is never passed here.

The cadence comes from durable state and the clock:

| Situation | Run |
|-----------|-----|
| No full export has completed | full |
| No full export since the configured weekday (default Monday) | full |
| The last full export is at least `full_export_max_age_days` old | full |
| Otherwise | incremental from `last_successful_export_at` minus `incremental_overlap_days` |

The weekday boundary is local midnight on this Mac, not UTC midnight, and report
file names carry the local date.

A run advances `last_successful_export_at` (and `last_full_export_at` for a full
run) only when osxphotos exits 0 and its report contains no error or missing
rows. An incomplete export exits 1 and keeps completed files and reports. The
next run retries from the previous successful baseline.

The report is the evidence that the export ran: a report osxphotos never wrote,
one that cannot be read, and one carrying none of the columns osxphotos writes
are all failed exports, so neither the state nor the mirror moves on an export
nothing describes. A report holding only its header is a valid clean run that
exported nothing. Every invocation is given report paths nothing has written
yet, so the evidence always belongs to the run being judged: a second export on
the same host and day that writes no report fails instead of inheriting the
first export's report.

The supplementary late-additions CSV is best-effort: a file, encoding, or CSV
error prints a warning identifying the report, which may be partial. A clean,
complete export still advances its baseline; errors in the main osxphotos report
continue to prevent that advance.

Apple Photos dry runs (`bootstrap`, `daily`, `recent`, `apple-photos`, and the
Apple Photos step of `backup-all`) are planning-only:
they do not invoke osxphotos, write reports, or touch `export.db` or `state.json`.
The manual `apple-photos --testing` command is the deliberate exception: it runs
a limited osxphotos simulation for exporter development without writing assets.
Adding `--dry-run` to `--testing` selects the planning-only preview instead.

Every export entry point checks the archive writer before exporting, including
manual `apple-photos` and `backup-all` runs. Manual extra flags cannot override
the managed library, destination, export database, report, dry-run mode, or
cleanup behavior, nor load an osxphotos configuration that overrides them.
Use this tool's TOML configuration, `--volume`, and cleanup commands for those
changes. Other extra flags still accept `--flag value` or `--flag=value`.
Manual exports with any forwarded flags keep their exported files and reports
but do not advance daily/full backup timestamps or replace the baseline report.
This conservative rule covers asset filters, limits, and skipped components
without assuming a custom run covered the scheduled export. Manual exports
without forwarded flags retain the normal cadence behavior.
Real exports print their CSV report path, including when the export fails.

## Back up recent photos without waiting for bootstrap

```sh
photos-backup recent                         # photos/videos taken in the last 30 days
photos-backup recent --days 60               # a larger capture-date window
photos-backup recent --days 30 --dry-run      # show the plan without exporting
photos-backup recent --download-timeout 300  # allow 5 minutes per missing asset
```

`recent` uses the existing archive and export database, including an interrupted
bootstrap, and applies the normal writer check. It processes fully local assets
first and then attempts missing originals and other requested components through
PhotoKit. Unchanged exported files are skipped by osxphotos's update checks.
The date filter uses capture dates, not import dates; newly imported older photos
are outside the selected window.

There is **no total run time limit**. Missing-file retrieval has a 120-second
budget per asset, shared across its original/edited versions and any retries
within a run. Local copies and metadata writes do not consume that budget.
One download worker is started lazily and reused for serial retrievals, avoiding
repeated Python/PhotoKit imports. Worker startup and request overhead count toward
the asset's budget. A stalled worker is killed and reaped without interrupting an
archive write; the next eligible asset gets a fresh worker. Ctrl+C also reaps the
worker. This is an elapsed-time
limit, so a large download taking longer than the budget is also deferred; use
`--download-timeout` to allow it more time. Bootstrap and daily exports offer
the same timeout option and live phase/download progress:

```sh
photos-backup bootstrap --download-timeout 300
photos-backup daily --download-timeout 300
```

Both default to 120 seconds per asset and report total command time, including
failed or interrupted runs. Bootstrap also reports its library coverage check;
daily reports cleanup reconciliation. Their dry runs remain planning-only.

Failed or timed-out downloads are printed immediately. Failures still unresolved
at the end of the run are recorded beside the CSV export report in a `.downloads.json` file, with asset UUIDs, filenames, and
reasons. Other unavailable components appear in the CSV's `missing` rows.
The recent command exits nonzero when files remain missing or errors occur.
When PhotoKit classifies a Live Photo as a still but retains its paired-video
resources, the download worker retrieves the requested video resource directly.
Original and edited videos use their respective resources; unavailable video
components still fail the export. This fallback shares the asset's timeout.
Rerun it to retry; completed exports are retained, and each run gets a fresh
per-asset budget. Items that age out of the date window require a larger `--days`
value or a full export.

`recent` announces archive checks, library loading, asset selection, export, and
report generation. During export, status shows the current filename, **assets
processed / selected**, operation elapsed time, and unresolved download count.
Retrieval status also shows the attempt number and remaining per-asset budget.
Processed assets include skipped and unsuccessful items; one asset can produce
multiple files. Final file outcomes come from the export CSV, not that counter.
No byte percentage or estimated completion time is inferred from a PhotoKit wait.

Terminal status refreshes every second. Redirected output uses plain lines every
10 seconds, plus phase transitions and failures. Phase timings include nested
work and must not be added together. Export elapsed time includes report
creation; total command time also includes archive checks and library loading.

Late-additions reports reuse Spotlight metadata for repeated paths within a run.
Each `mdls` call has a 10-second timeout. Unavailable metadata remains blank and
produces an enrichment warning; it does not turn an otherwise successful media
backup into a failed export.

A recent export never initializes the archive, advances its full/daily export
timestamps, or mirrors deletions. Finish bootstrap separately when a complete
library backup is wanted. Stop an existing bootstrap with Ctrl+C and wait for
it to exit before starting `recent`; both use the same archive lock.

## Finding late shared photos

Apple Photos exports are organized by the photo's content creation date, so a
photo taken in March or April can appear in an older month directory even when
it is first synchronized in June. Each export writes an additional
`late_photo_additions_<host>_YYYY-MM-DD.csv` report next to the
`photos_export_<host>_YYYY-MM-DD.csv` report in `.photos-backup/reports/`. Both
names gain a `_2`, `_3`, ... suffix for every further export the same host runs
on the same day, so an export never reads a report an earlier run left behind
and no earlier report is overwritten. It lists files that were `new` or
`updated` during the run, their Spotlight/Finder dates, device make/model, and
whether the file matches configured spouse-device models.

Configure spouse-device models in the `[apple_photos]` section:

```toml
spouse_device_models = ["iPhone SE (2nd generation)", "iPhone 17"]
```

For an immediate Finder-compatible search, use Spotlight metadata directly.
Replace the example path with your configured `apple_photos.archive`:

```sh
mdfind -onlyin "/Volumes/SanDisk/Media/Apple Photos" \
'kMDItemDateAdded >= $time.iso(2026-06-01T00:00:00Z) &&
 kMDItemContentCreationDate < $time.iso(2026-06-01T00:00:00Z) &&
 kMDItemAcquisitionModel == "*iPhone SE*"'
```

The same fields can be used in a Finder Smart Folder scoped to the export
directory: `Date Added`, `Content created`, and, when Finder exposes it,
`Device model`. Use the generated CSV when Finder cannot show `Device model` as
a list column.
