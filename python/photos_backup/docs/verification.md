# Archive verification

[Back to the user guide](../README.md)

## Remember a verification

```sh
photos-backup verify --record
photos-backup verify --record --json
photos-backup status --short
```

`--record` saves a small local receipt of the latest requested verification: start
and completion times, pass/fail, failed check names, and any archive-opening error.
It can be combined with `--json` and `--report`; recording notices go to stderr.
Ordinary `verify` does not record anything automatically, and an unrecorded run
does not replace a previous receipt.

`photos-backup daily --verify` runs the same checks right after a complete
export, while it still holds the archive lock, and records the receipt as
`--record` would. Status then treats the archive as verified since its latest
export instead of suggesting `verify --record` after every scheduled run. A
failed or dry-run export skips verification. A failed check makes `daily` exit 1
after its export result prints; a pending cleanup still exits 3. Configuration and argument errors are not
verification results and do not create receipts.

Receipts live under `~/.local/state/photos-backup/verifications/`, or
`$XDG_STATE_HOME/photos-backup/verifications/` when that variable is absolute.
They are separate for each configuration path and archive path, including volume
overrides. They stay on this Mac, outside the archive. A state directory resolving
inside the archive is refused. Writes use a lock and atomic replacement; an older
overlapping scan cannot replace a more recently started scan's result. Receipt I/O
failures warn without changing verification's result or exit code. Status reads
malformed receipts without repairing them.

This records archive checks, not checksums or verification of SSD/cloud copies.
Use the [restore rehearsal](restore.md) to test actual recovery.

## Verifying an archive

`photos-backup verify` reads the archive and reports one `PASS`/`FAIL` line per
property with the evidence behind it. It exits 0 when all checks pass, 3 when
pending cleanup approval is the only failed check, and 1 when any other check
fails. It writes nothing at all: no directory, no state, no lock file.

If the archive cannot be opened, `verify --json` still prints one JSON document:
`passed` is false, `checks` is empty, and `archive_error` contains the reason.
The diagnostic remains on stderr and the original nonzero exit code is preserved.
When verification runs, `archive_error` is null. Configuration and command-line
errors retain normal CLI diagnostics. `--report` saves completed verification
findings only; an archive-open error does not create a report file.

Verification prints its current phase and file-check count to stderr, with
periodic updates during long scans. Progress counts inspected records, including
missing or invalid files; the final findings determine whether verification
passed. JSON reports retain their existing fields and add timing information.

| Check | Fails when |
|-------|------------|
| `export database` | It is missing or unreadable, `PRAGMA integrity_check` is not `ok`, the schema version is one this osxphotos cannot read, or records point outside the archive |
| `signatures` | An exported file no longer matches its recorded size and modification time |
| `missing assets` | An exported file is missing, unreadable, not a regular file, or reached through a symlink beneath the archive |
| `state` | State is unreadable, the archive was never initialized, no export is recorded, the last full export is newer than the last successful one, or the last report is gone |
| `ownership` | No Mac has claimed the archive |
| `pending cleanup` | A cleanup run is still waiting for approval |

Every read that can fail is caught and reported as a failed check, so one corrupt
file never hides the state of everything else. Another Mac owning the archive is
a pass that names both hostnames, since that is normal for a shared archive.

Signature output counts matched files, changed files, records missing a size or
modification time, and signed records unavailable for comparison. A missing
signature is reported as incomplete coverage, not a mismatch; the signatures
check can still pass for the records it could compare. Missing or unreadable
files fail the separate `missing assets` check. Verification checks size and
modification time only; it does not checksum file contents.

To save the full findings, including every missing, changed, or out-of-archive
file path rather than just the terminal's three-path sample:

```sh
photos-backup verify --report ~/Desktop/photos-verification.json
```

The JSON contains `version`, `archive`, `passed`, `archive_error`, and a `checks`
array, plus UTC `started_at` and `completed_at` timestamps and `elapsed_seconds`
measured with a monotonic clock. Timing covers archive opening and verification,
not report writing; archive-open errors also include timing in `--json` output.
Each check has `name`, `passed`, `detail`, and `paths`. The report is saved even
when verification fails, and verification keeps its usual exit code. Choose a new file outside the
archive in an existing directory: invalid parent directories are rejected before
the scan starts. Reports never overwrite existing files or modify archive contents.
A report write failure exits 1.

Use `photos-backup verify --json` to print the same document directly to stdout,
including failed findings, with the same exit codes. Progress and diagnostics stay
on stderr. It can be combined with `--report` to save an identical document while
also printing JSON; the saved-report notice then goes to stderr. Configuration or
argument errors produce CLI diagnostics without a JSON document; archive-opening
errors produce the error document described above.
