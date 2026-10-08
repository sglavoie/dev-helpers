# Development reference

[Back to the user guide](../README.md)

```bash
cd python/photos_backup
uv sync                                  # create .venv and install the package
uv run python3 -m unittest discover -v   # tests
uv run ruff check .                      # lint
uv run mypy                              # check Photos integration and report types
uv run ruff format .                     # format
uv run photos-backup --help              # run without installing
```

The package lives under `src/photos_backup/`:

| Module | Responsibility |
|--------|----------------|
| `cli/` | One module per command; parsing, prompting, and exit codes only |
| `config.py` | Load and validate the TOML file into frozen dataclasses |
| `archive/` | Path resolution, mount and symlink safety, locking, versioned state |
| `apple_photos/` | Export planning, identity and takeover, reconciliation, cleanup, verification |
| `summary.py` | Every line the commands print |
| `presentation.py` | Terminal tables and redirected error formatting |
| `errors.py` | `ActionRequired`, the exit-code-3 exception |
| `exclude.py` | The shared `--exclude-from` argument helper |
| `sd_card/`, `ssd/`, `remote/` | rsync and rclone workflows |

The pattern throughout is to classify first and act second: a pure function
takes plain data and returns a decision plus the reason behind it, and a thin
caller performs the effect. That is why the tests need neither a Photos library
nor an external drive — the seams (`SystemProbes`, `PhotosProbes`) are injected,
and deletion tests only ever touch temporary directories.

Every test runs offline. Deletion tests use temporary fixtures or mocked effects;
they must never target real photo libraries or backups. `cleanup-local-export`
also refuses nonterminal stdin; `approve-cleanup` has no terminal requirement.

## Dependency compatibility

The CLI uses rich-click on top of Click; command parsing, prompts, and exit codes
remain Click's responsibility. Rich renders summary tables only on capable
terminals. Pass data as `Text` so filenames and error messages containing brackets
are never interpreted as markup. Keep Rich within the range required by the pinned
`osxphotos` release (currently `>=13.5.2,<14`).

`osxphotos` is pinned to exactly `0.77.2` in both package requirements and the
lockfile, including for `uv tool install`. The adapter depends on private export
and PhotoKit staging hooks. Upgrade deliberately: update the pin and the supported
version in `tests/test_osxphotos_compatibility.py`, refresh the lockfile, then run
the full test suite and `uv run mypy`. The offline tests check upstream call
signatures, staging serialization, and actual upstream staging with native Photos
I/O mocked, including originals, edits, Live Photos, video, and RAW pairs.
`apple-photos` parses forwarded flags with the upstream `export` command's own
Click parser and parameter types, so an upgrade that renames or retypes an export
option changes what those flags accept. A real
export on a disposable archive is still needed to validate native PhotoKit behavior
before deploying an upgrade to both Macs.

The type check covers the adapter, download worker, export argument builder,
late-additions report, configuration, copy safety, subprocess helpers, transfer
receipts, and SD card folder detection; add modules to `[tool.mypy] files` as
they are annotated. It checks our code; upstream `osxphotos` internals remain
outside static checking and are covered by the compatibility tests.

## Profiling metadata reports

Late-additions enrichment runs up to four `mdls` processes concurrently, each with
the existing ten-second timeout. It schedules at most 32 changed rows per batch,
preserves source CSV order, and reads metadata once per distinct path across the
report. The per-path cache still grows with the number of distinct changed files.
Missing metadata remains a warning and does not decide export success. Injected
Python metadata readers must be thread-safe; callers can pass `metadata_workers=1`
to `generate_late_photo_additions_report` for serial execution.

Compare serial and concurrent enrichment without starting an export:

```bash
uv run python scripts/profile_reports.py                         # simulated I/O
uv run python scripts/profile_reports.py --spotlight             # mdls on temporary files
uv run python scripts/profile_reports.py --export-report /path/to/photos_export.csv
```

The last command reads an existing export report and queries Spotlight for its
new or updated files. All generated CSVs and fixtures live in a temporary directory;
the input report and exported files are not changed. Results describe only report
enrichment, not total backup throughput. `--profile /tmp/photos-report.prof` also
writes a cProfile file for the coordinator; worker time appears as waiting. Inspect
it with `uv run python -m pstats /tmp/photos-report.prof`.
