from __future__ import annotations

import csv
import datetime
import re
from dataclasses import dataclass
from dataclasses import field as dataclass_field
from pathlib import Path
from typing import TYPE_CHECKING

import click

from photos_backup.cli.context import suggested_command
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.late_additions import csv_flag
from photos_backup.presentation import print_terminal_table

if TYPE_CHECKING:
    from photos_backup.apple_photos.bootstrap import BootstrapResult
    from photos_backup.apple_photos.cleanup import (
        CleanupApproval,
        CleanupDiscard,
        CleanupPreview,
        MirrorOutcome,
    )
    from photos_backup.apple_photos.local_export import (
        LocalExportCleanup,
        LocalExportPlan,
    )
    from photos_backup.apple_photos.plan import ExportPlan, ExportResult
    from photos_backup.apple_photos.takeover import TakeoverCheck
    from photos_backup.apple_photos.verify import VerificationReport
    from photos_backup.archive import Archive, ArchiveState


_SCALE = 1024
_SECONDS_PER_MINUTE = 60


@dataclass
class BackupSummary:
    step_name: str
    files_transferred: int | None = None
    total_size: str = ""
    elapsed_seconds: float = 0.0
    skipped: bool = False
    planned: bool = False
    error: str | None = None
    dry_run: bool = False
    action_required: bool = False
    skip_reason: str | None = None


def parse_rsync_stats(output: str) -> dict[str, int | str | None]:
    """Parse rsync --stats output for file count and total size."""
    result: dict[str, int | str | None] = {"files_transferred": None, "total_size": ""}

    files_match = re.search(r"Number of regular files transferred:\s*([\d,]+)", output)
    if files_match:
        result["files_transferred"] = int(files_match.group(1).replace(",", ""))

    size_match = re.search(r"Total transferred file size:\s*([\d,.]+ \S+)", output)
    if size_match:
        result["total_size"] = size_match.group(1)

    return result


ONE_HOT_REPORT_FIELDS = frozenset(
    {
        "exported",
        "new",
        "updated",
        "skipped",
        "exif_updated",
        "touched",
        "converted_to_jpeg",
        "sidecar_xmp",
        "sidecar_json",
        "sidecar_exiftool",
        "missing",
        "error",
        "exiftool_warning",
        "exiftool_error",
        "extended_attributes_written",
        "extended_attributes_skipped",
        "cleanup_deleted_file",
        "cleanup_deleted_directory",
        "sidecar_user",
        "sidecar_user_error",
        "user_written",
        "user_skipped",
        "user_error",
        "aae_written",
        "aae_skipped",
    }
)


@dataclass(frozen=True)
class ExportReport:
    """One osxphotos CSV report, or the reason it could not be read."""

    counts: dict[str, int] = dataclass_field(default_factory=dict)
    problem: str | None = None


def read_export_report(report_path: Path) -> ExportReport:
    """Read an osxphotos export CSV report and count rows by status.

    A report that was never written, cannot be read, or carries none of the
    columns osxphotos writes is a problem rather than a run with nothing to
    report; a report holding only its header is a clean run that exported
    nothing.
    """
    if not report_path.is_file():
        return ExportReport(problem=f"osxphotos wrote no report at '{report_path}'")

    counts: dict[str, int] = {}
    try:
        with report_path.open(newline="") as f:
            reader = csv.DictReader(f)
            fieldnames = set(reader.fieldnames or [])
            one_hot_fields = ONE_HOT_REPORT_FIELDS.intersection(fieldnames)
            if not one_hot_fields and "export_status" not in fieldnames:
                return ExportReport(
                    problem=(
                        f"the report at '{report_path}' has no column this tool "
                        "recognizes"
                    )
                )

            for row in reader:
                if one_hot_fields:
                    for name in one_hot_fields:
                        counts[name] = counts.get(name, 0) + _csv_count(row.get(name))
                    continue

                status = row.get("export_status", "").lower().strip()
                if status:
                    counts[status] = counts.get(status, 0) + 1
    except (OSError, UnicodeDecodeError, csv.Error) as error:
        return ExportReport(
            problem=f"the report at '{report_path}' could not be read ({error})"
        )
    return ExportReport(counts=counts)


def _csv_count(value: str | None) -> int:
    if not csv_flag(value):
        return 0

    try:
        return int((value or "").strip())
    except ValueError:
        return 1


def print_summary(summary: BackupSummary) -> None:
    """Print a formatted summary for a single backup step."""
    rows: list[tuple[str, str]] = []
    if summary.error:
        status = "ACTION REQUIRED" if summary.action_required else "ERROR"
        rows.append(("Status", f"{status} — {summary.error}"))
    elif summary.planned:
        rows.append(("Status", "PLANNED — command was not invoked"))
    elif summary.skipped:
        reason = f" — {summary.skip_reason}" if summary.skip_reason else ""
        rows.append(("Status", f"SKIPPED{reason}"))
    else:
        rows.append(("Status", "DRY RUN" if summary.dry_run else "OK"))
        if summary.files_transferred is not None:
            label = "Proposed transfers" if summary.dry_run else "Files transferred"
            rows.append((label, str(summary.files_transferred)))
        else:
            rows.append(("Transfer count", "unavailable"))
        if summary.total_size:
            label = "Proposed size" if summary.dry_run else "Total size"
            rows.append((label, summary.total_size))
    rows.append(("Elapsed", f"{summary.elapsed_seconds:.1f}s"))
    if print_terminal_table(
        summary.step_name,
        (),
        rows,
        styles=[_summary_style(summary)] + [""] * (len(rows) - 1),
    ):
        return
    click.echo()
    click.echo(f"--- {summary.step_name} ---")
    for label, value in rows:
        click.echo(f"  {label}: {value}")


def _summary_style(summary: BackupSummary) -> str:
    if summary.error:
        return "bold yellow" if summary.action_required else "bold red"
    if summary.planned or summary.dry_run:
        return "cyan"
    if summary.skipped:
        return "dim"
    return "green"


def print_export_result(result: ExportResult, *, dry_run: bool = False) -> None:
    """Print the export plan, its reports, and the resulting step summary."""
    prefix = "Would export" if dry_run else "Exported"
    click.echo(f"{prefix} ({result.plan.mode.value}): {result.plan.reason}")
    if result.report_path is not None and not dry_run:
        click.echo(f"Export report: {result.report_path}")
    if result.missing_count:
        click.echo(f"  Missing export files: {result.missing_count}")
    if result.performed:
        # Import here because plan also uses BackupSummary.
        from photos_backup.apple_photos.plan import ERROR_COUNT_FIELDS  # noqa: PLC0415

        outcomes = ("new", "updated", "skipped", "missing", "error") + tuple(
            name
            for name in ERROR_COUNT_FIELDS
            if name != "error" and result.counts.get(name, 0)
        )
        click.echo(
            "  File outcomes: "
            + ", ".join(f"{name}={result.counts.get(name, 0)}" for name in outcomes)
        )
    if result.phase_timings:
        click.echo(
            "  Phase timings (nested phases overlap): "
            + ", ".join(
                f"{name}: {seconds:.1f}s"
                for name, seconds in result.phase_timings.items()
            )
        )
    if result.late_additions_path is not None:
        click.echo(
            f"  Late additions: {result.late_additions_rows} "
            f"({result.late_additions_path})"
        )
    print_summary(result.summary())


def print_takeover_check(check: TakeoverCheck, *, dry_run: bool = False) -> None:
    """Print how this Mac came to be the archive writer, and what it cost."""
    if check.previous_hostname:
        click.echo(f"Archive writer: '{check.previous_hostname}' -> '{check.hostname}'")
    else:
        click.echo(f"Archive writer: '{check.hostname}'")

    if check.verdict is not None:
        comparison = check.verdict.comparison
        click.echo(
            f"  Library matched: {comparison.matched} of {comparison.comparable} "
            "export-database record(s) by iCloud cloud GUID"
        )
    if check.migration is not None:
        prefix = "Would migrate" if dry_run else "Migrated"
        click.echo(
            f"  {prefix} export database: {check.migration.matched} matched, "
            f"{check.migration.unmatched} unmatched"
        )
    if check.deletion_candidates:
        click.echo(f"  Deletion candidates: {len(check.deletion_candidates)}")


def print_bootstrap_result(result: BootstrapResult, *, dry_run: bool = False) -> None:
    """Print what the bootstrap exported and whether it may initialize."""
    prefix = "Would bootstrap" if dry_run else "Bootstrapping"
    stage = "resuming an earlier attempt" if result.resumed else "fresh archive"
    click.echo(f"{prefix} ({stage})")
    if result.takeover.status is not WriterStatus.UNCHANGED:
        print_takeover_check(result.takeover, dry_run=dry_run)
    print_export_result(result.export, dry_run=dry_run)

    coverage = result.coverage
    if result.excluded_hidden:
        click.echo(f"Excluded from coverage: {result.excluded_hidden} hidden asset(s)")
    if coverage is None:
        click.echo("Coverage: deferred until the real export")
    else:
        click.echo(
            f"Coverage: {coverage.library - len(coverage.unrecorded)} of "
            f"{coverage.library} library asset(s) recorded in the export database"
        )

    reason = result.blocking_reason()
    if result.initialized:
        click.echo(f"Archive initialized at {result.initialized_at.isoformat()}")
    elif dry_run:
        click.echo("Dry run: nothing was written and the archive was not initialized")
    else:
        click.echo(f"Archive not initialized: {reason}")


def print_mirror_outcome(outcome: MirrorOutcome, *, dry_run: bool = False) -> None:
    """Print what reconciling the archive against the library did or would do."""
    prefix = "Mirror preview" if dry_run else "Mirror"
    click.echo(f"{prefix} ({outcome.status.value}): {outcome.reason}")

    reconciliation = outcome.reconciliation
    if reconciliation is not None:
        click.echo(f"  Deletion candidates: {len(reconciliation.candidates)}")
        _print_counts(
            ("Modified since export", reconciliation.changed),
            ("Without an export record", reconciliation.unknown),
            ("Claimed by several assets", reconciliation.ambiguous),
        )
    if outcome.manifest_path is not None:
        click.echo(f"  Manifest: {outcome.manifest_path}")
        click.echo(
            f"  Preview with: {suggested_command('approve-cleanup', outcome.run_id, '--dry-run')}"
        )
        click.echo(
            f"  Approve with: {suggested_command('approve-cleanup', outcome.run_id)}"
        )


def print_cleanup_preview(preview: CleanupPreview) -> None:
    """Show exactly what revalidation permits, without implying approval."""
    total = sum(candidate.size for candidate in preview.deletable)
    click.echo(f"Cleanup preview for run '{preview.run_id}'")
    click.echo(f"  Would delete: {len(preview.deletable)} archive file(s)")
    click.echo(f"  Proposed size: {human_size(total)} ({total} bytes)")
    for candidate in preview.deletable:
        click.echo(f"    Would delete: {candidate.path}")
    for label, paths in (
        ("No longer deletable, so kept", preview.stale),
        ("Never reviewed, so kept", preview.unreviewed),
    ):
        click.echo(f"  {label}: {len(paths)}")
        for path in paths:
            click.echo(f"    Kept: {path}")
    click.echo("Dry run: nothing was written or deleted; cleanup remains pending")


def print_cleanup_approval(approval: CleanupApproval) -> None:
    """Print what approving one pending cleanup run deleted, and what it kept."""
    click.echo(f"Approved cleanup run '{approval.run_id}'")
    click.echo(f"  Deleted: {len(approval.deleted)} archive file(s)")
    _print_counts(
        ("No longer deletable, so kept", approval.stale),
        ("Never reviewed, so kept", approval.unreviewed),
    )
    if approval.complete:
        click.echo(f"  Mirror completed at {approval.completed_at.isoformat()}")
    else:
        click.echo(
            "  The mirror is not complete; the next full export will propose the "
            "remaining deletions"
        )


def print_cleanup_discard(discard: CleanupDiscard) -> None:
    """Print that a pending run was rejected, and where the record of it stayed."""
    click.echo(f"Discarded cleanup run '{discard.run_id}'; nothing was deleted")
    click.echo(f"  Reviewed: {len(discard.manifest.candidates)} archive file(s)")
    click.echo(f"  Manifest kept at: {discard.manifest_path}")
    click.echo(
        "  The next full export will propose these deletions again if they are "
        "still deletable"
    )


def print_local_export_plan(plan: LocalExportPlan, *, dry_run: bool = False) -> None:
    """Print the exact directory, file count, and size a person is asked about."""
    click.echo()
    click.echo("--- Local export cleanup ---")
    click.echo(f"  Directory: {plan.target or '(none configured)'}")
    if plan.scan is not None:
        click.echo(f"  Files: {plan.scan.file_count}")
        click.echo(f"  Size: {human_size(plan.scan.total_bytes)}")
    if plan.refusal is not None:
        click.echo(f"  Refused: {plan.refusal}")
    elif dry_run:
        click.echo("  Dry run: nothing was deleted")


def print_local_export_cleanup(cleanup: LocalExportCleanup) -> None:
    """Print what emptying the local export deleted and reclaimed."""
    click.echo(f"Deleted {len(cleanup.deleted)} file(s) from '{cleanup.target}'")
    if cleanup.pruned:
        click.echo(f"  Emptied directories removed: {len(cleanup.pruned)}")
    click.echo(f"  Reclaimed: {human_size(cleanup.freed_bytes)}")


def human_size(total: int) -> str:
    """Render a byte count the way a person reading a deletion prompt needs it."""
    size = float(total)
    for unit in ("B", "KB", "MB", "GB"):
        if size < _SCALE:
            return f"{size:.0f} {unit}" if unit == "B" else f"{size:.1f} {unit}"
        size /= _SCALE
    return f"{size:.1f} TB"


def _print_counts(*labelled: tuple[str, tuple]) -> None:
    for label, items in labelled:
        if items:
            click.echo(f"  {label}: {len(items)}")


def _relative_age(value: datetime.datetime, now: datetime.datetime) -> str:
    seconds = (now - value).total_seconds()
    if abs(seconds) < _SECONDS_PER_MINUTE:
        return "just now" if seconds >= 0 else "in less than a minute"
    for unit, duration in (
        ("day", 86400),
        ("hour", 3600),
        ("minute", _SECONDS_PER_MINUTE),
    ):
        count = int(abs(seconds) // duration)
        if count:
            age = f"{count} {unit}{'s' if count != 1 else ''}"
            return f"{age} ago" if seconds >= 0 else f"in {age}"
    raise AssertionError("unreachable age")


def print_archive_status(
    archive: Archive, state: ArchiveState, plan: ExportPlan, *, now: datetime.datetime
) -> None:
    """Display durable state only; this is not a verification of archive files."""
    click.echo(f"Archive: {archive.paths.archive}")
    click.echo("Recorded backup status (files have not been verified)")
    click.echo(f"  Writer: {state.writer_hostname or '(unclaimed)'}")
    for label, value in (
        ("Initialized", state.initialized_at),
        ("Last successful export", state.last_successful_export_at),
        ("Last full export", state.last_full_export_at),
        ("Last archive cleanup reconciliation", state.last_mirror_completed_at),
    ):
        timestamp = (
            f"{value.isoformat()} ({_relative_age(value, now)})" if value else "(never)"
        )
        click.echo(f"  {label}: {timestamp}")
    click.echo(
        f"  Last successful baseline report: {state.last_report_path or '(none)'}"
    )
    click.echo(f"  Pending cleanup: {state.pending_cleanup_run_id or '(none)'}")
    if state.pending_cleanup_run_id:
        click.echo(
            f"  Review: {archive.paths.cleanup_manifest(state.pending_cleanup_run_id)}"
        )
        click.echo(
            f"  Preview with: {suggested_command('approve-cleanup', state.pending_cleanup_run_id, '--dry-run')}"
        )
        click.echo(
            f"  Approve with: {suggested_command('approve-cleanup', state.pending_cleanup_run_id)}"
        )
        click.echo(
            f"  Discard with: {suggested_command('approve-cleanup', state.pending_cleanup_run_id, '--discard')}"
        )
    if not state.initialized:
        click.echo(f"  Next step: {suggested_command('bootstrap')}")
    click.echo(f"  Next export: {plan.mode.value} — {plan.reason}")


def print_export_attempt(
    attempt: dict | None, error: str | None, *, now: datetime.datetime
) -> None:
    if error:
        click.echo(f"Warning: {error}", err=True)
    if attempt is None:
        click.echo("Latest Apple Photos export attempt: (not recorded)")
        return
    status = attempt["status"]
    if status == "started":
        status = "completion not recorded (running or interrupted)"
    mode = attempt["mode"] + ("; custom options/limit" if attempt["restricted"] else "")
    click.echo(f"Latest Apple Photos export attempt: {mode} — {status}")
    started = datetime.datetime.fromisoformat(attempt["started_at"])
    click.echo(f"  Started: {started.isoformat()} ({_relative_age(started, now)})")
    if attempt["completed_at"]:
        click.echo(f"  Completed: {attempt['completed_at']}")
    click.echo(f"  Mac: {attempt['hostname']}")
    click.echo(f"  Export report (if written): {attempt['report_path']}")
    click.echo(
        f"  Baseline advanced: {'yes' if attempt['baseline_advanced'] else 'no'}"
    )
    if attempt["error"]:
        click.echo(f"  Error: {attempt['error']}")
    if attempt.get("missing_count") is not None:
        click.echo(f"  Missing files: {attempt['missing_count']}")
    if attempt.get("error_count") is not None:
        click.echo(f"  Export errors: {attempt['error_count']}")
    if attempt["status"] in ("failed", "interrupted", "started"):
        if attempt.get("download_report_path"):
            click.echo(
                f"  Download report (if written): {attempt['download_report_path']}"
            )
        if attempt.get("retry_arguments"):
            click.echo(
                f"  Retry export: {suggested_command(*attempt['retry_arguments'])}"
            )
        elif attempt["restricted"]:
            click.echo("  Retry with the original manual export options/limit.")


def _transfer_attention(receipts: list[dict]) -> list[str]:
    attention = []
    for status, label in (
        ("failed", "latest attempt failed"),
        ("interrupted", "latest attempt interrupted"),
        ("started", "completion not recorded (running or interrupted)"),
    ):
        count = sum(
            row["last_attempt"] is not None and row["last_attempt"]["status"] == status
            for row in receipts
        )
        if count:
            attention.append(f"{count} with {label}")
    never_succeeded = sum(row["last_success"] is None for row in receipts)
    if never_succeeded:
        attention.append(f"{never_succeeded} with no successful copy recorded")
    older_copies = sum(
        row.get("archive_exported_since_copy") is True for row in receipts
    )
    if older_copies:
        attention.append(
            f"{older_copies} with an archive export since the copy started"
        )
    newer_ssd = sum(row.get("ssd_copied_since_upload") is True for row in receipts)
    if newer_ssd:
        attention.append(f"{newer_ssd} with an SSD copy since the cloud upload started")
    for field, label in (
        ("sd_imported_since_copy", "an SD-card import since the SSD copy started"),
        (
            "archive_may_have_changed_since_copy",
            "possible archive changes since the copy started",
        ),
    ):
        count = sum(row.get(field) is True for row in receipts)
        if count:
            attention.append(f"{count} with {label}")
    return attention


def print_transfer_history(
    receipts: list[dict],
    errors: list[str],
    *,
    now: datetime.datetime | None = None,
    historical_receipts: list[dict] | None = None,
) -> None:
    now = now or datetime.datetime.now(datetime.UTC)

    click.echo("Transfer history (this Mac, this configuration)")
    if historical_receipts is not None:
        click.echo("  Current configured destinations")
    attention = _transfer_attention(receipts)
    if attention:
        click.echo(f"  Attention: {'; '.join(attention)}")
    if not receipts:
        click.echo(
            "  No configured transfer destinations"
            if historical_receipts is not None
            else "  No transfer receipts recorded"
        )
    for receipt in receipts:
        _print_transfer_receipt(receipt, now=now, retry=True)
    if historical_receipts:
        click.echo("  Historical destinations (not currently configured)")
        for receipt in historical_receipts:
            _print_transfer_receipt(receipt, now=now)
    for error in errors:
        click.echo(f"Warning: {error}", err=True)


def _print_transfer_receipt(
    receipt: dict, *, now: datetime.datetime, retry: bool = False
) -> None:
    def timestamp(value: str) -> str:
        return f"{value} ({_relative_age(datetime.datetime.fromisoformat(value), now)})"

    click.echo(f"  {receipt['step']}: {receipt['source']} → {receipt['destination']}")
    _print_transfer_freshness(receipt)
    attempt = receipt["last_attempt"]
    if attempt is None:
        click.echo("    No transfer receipts recorded; no successful copy recorded")
        return
    outcome = attempt["status"]
    if outcome == "started":
        outcome = "completion not recorded (running or interrupted)"
    click.echo(f"    Last attempt: {timestamp(attempt['started_at'])} — {outcome}")
    click.echo(f"    Attempt mode: {_transfer_mode(attempt)}")
    if attempt.get("error"):
        click.echo(f"    Error: {attempt['error']}")
    if retry:
        _print_transfer_retry(receipt)
    success = receipt["last_success"]
    click.echo(
        f"    Last successful copy: {timestamp(success['completed_at']) if success else '(never)'}"
    )
    if success:
        click.echo(f"    Successful copy mode: {_transfer_mode(success)}")
        details = []
        count = success.get("files_transferred")
        if type(count) is int and count >= 0:
            details.append(f"{count} files")
        size = success.get("total_size")
        if isinstance(size, str) and size:
            details.append(size)
        elapsed = success.get("elapsed_seconds")
        if type(elapsed) in (int, float) and elapsed >= 0:
            details.append(f"{elapsed:.1f}s")
        if details:
            click.echo(f"    Successful copy details: {', '.join(details)}")


def _print_transfer_freshness(receipt: dict) -> None:
    if receipt.get("archive_exported_since_copy"):
        click.echo(
            "    Archive exported since this copy started (recorded-time hint; "
            "destination files have not been checked)"
        )
    if receipt.get("ssd_copied_since_upload"):
        click.echo(
            "    SSD copy completed since the last cloud upload started "
            "(recorded-time hint; destination files have not been checked)"
        )
    if receipt.get("sd_imported_since_copy"):
        click.echo(
            "    SD-card import completed since this SSD copy started (recorded-time hint; destination files have not been checked)"
        )
    if receipt.get("archive_may_have_changed_since_copy"):
        click.echo(
            "    Archive may have changed since this copy started: an incomplete export can retain new files"
        )


def _print_transfer_retry(receipt: dict) -> None:
    attempt = receipt["last_attempt"]
    if attempt["status"] in ("failed", "interrupted"):
        command = {
            "SD Card": "sd-card",
            "SSD: All Photos": "ssd",
            "SSD: SD Card": "ssd",
            "Remote": "remote",
        }.get(receipt["step"])
        if command:
            arguments = [command]
            if attempt.get("mode") == "mirror":
                arguments.extend(["--delete", "--dry-run"])
                label = "Preview mirror retry"
            else:
                label = "Retry copy"
            click.echo(f"    {label}: {suggested_command(*arguments)}")


def _transfer_mode(attempt: dict) -> str:
    return {
        "copy": "copy (preserves destination-only files)",
        "mirror": "mirror (deletions enabled)",
    }.get(attempt.get("mode"), "mode not recorded")


def print_download_summary(summary: dict | None, error: str | None) -> None:
    if error:
        click.echo(f"Warning: {error}", err=True)
    if summary is None:
        return
    reasons = "; ".join(
        f"{count} {reason}" for reason, count in summary["reasons"].items()
    )
    click.echo(
        f"  Unresolved downloads in latest report: {summary['count']}"
        + (f" ({reasons})" if reasons else "")
    )
    for row in summary["samples"]:
        filename = " ".join(row["filename"].split())[:120]
        reason = " ".join(row["reason"].split())[:180]
        click.echo(f"    {filename}: {reason}")
    if summary["count"] > len(summary["samples"]):
        click.echo(
            f"    and {summary['count'] - len(summary['samples'])} more; see {summary['report_path']}"
        )


def print_verification_history(
    receipt: dict | None, error: str | None, *, now: datetime.datetime
) -> None:
    if error:
        click.echo(f"Warning: {error}", err=True)
    if receipt is None:
        click.echo("Last verification (this Mac): not recorded")
        return
    age = _relative_age(datetime.datetime.fromisoformat(receipt["completed_at"]), now)
    details = ["passed" if receipt["passed"] else "failed", age]
    if receipt.get("archive_error"):
        details.append(receipt["archive_error"])
    elif receipt["failed_checks"]:
        details.append(", ".join(receipt["failed_checks"]))
    if not receipt["archive_state_available"]:
        details.append("archive freshness unknown")
    elif receipt["archive_exported_since_verification"]:
        details.append("archive exported since then")
    elif receipt["archive_may_have_changed_since_verification"]:
        details.append("archive may have changed since then")
    click.echo(f"Last verification (this Mac): {'; '.join(details)}")


def print_short_status(document: dict, *, now: datetime.datetime) -> None:
    click.echo("Recorded status — files have not been verified by this command")
    commands: list[str] = []
    _print_short_archive(document, now, commands)
    if document["export_attempt_error"]:
        click.echo(f"Warning: {document['export_attempt_error']}", err=True)
    print_download_summary(
        document["download_summary"], document["download_report_error"]
    )
    for row in document["configured_transfers"]:
        _print_short_transfer(row, now, commands)
    for error in document["transfer_history_errors"]:
        click.echo(f"Warning: {error}", err=True)
    if document["archive_configured"]:
        receipt = document["last_verification"]
        print_verification_history(
            receipt, document["verification_history_error"], now=now
        )
        if (
            receipt is None
            or not receipt["passed"]
            or receipt["archive_exported_since_verification"]
            or receipt["archive_may_have_changed_since_verification"]
        ):
            commands.append(suggested_command("verify", "--record"))
    if commands:
        click.echo("Suggested next commands (mirror retries are previews):")
        for command in dict.fromkeys(commands):
            click.echo(f"  {command}")


def _print_short_archive(
    document: dict, now: datetime.datetime, commands: list[str]
) -> None:
    state, attempt = document["state"], document["last_export_attempt"]
    if not document["archive_configured"]:
        click.echo("Apple Photos: not configured")
    elif document["archive_error"]:
        click.echo(f"Apple Photos: unavailable — {document['archive_error']}")
    else:
        if not state["initialized_at"]:
            export_status = "bootstrap incomplete"
            commands.append(suggested_command("bootstrap"))
        elif attempt and attempt["status"] != "succeeded":
            export_status = {
                "failed": "latest export incomplete",
                "interrupted": "latest export interrupted",
                "started": "completion not recorded (running or interrupted)",
            }[attempt["status"]]
            if attempt.get("retry_arguments"):
                commands.append(suggested_command(*attempt["retry_arguments"]))
            elif attempt["restricted"]:
                export_status += "; retry with original manual options/limit"
            else:
                commands.append(suggested_command("daily"))
        else:
            timestamp = state["last_successful_export_at"]
            export_status = (
                "baseline "
                + _relative_age(datetime.datetime.fromisoformat(timestamp), now)
                if timestamp
                else "no successful baseline recorded"
            )
            if attempt and not attempt["baseline_advanced"]:
                export_status += (
                    f"; latest {attempt['mode']} export succeeded (baseline unchanged)"
                )
        click.echo(
            f"Apple Photos: {export_status}; next export {document['next_export']['mode']}"
        )
        pending = state["pending_cleanup_run_id"]
        click.echo(f"Cleanup: {'pending ' + pending if pending else 'none pending'}")
        if pending:
            commands.append(suggested_command("approve-cleanup", pending, "--dry-run"))


def _print_short_transfer(
    row: dict, now: datetime.datetime, commands: list[str]
) -> None:
    latest, success = row["last_attempt"], row["last_success"]
    hints = [
        row.get(field)
        for field in (
            "archive_exported_since_copy",
            "ssd_copied_since_upload",
            "sd_imported_since_copy",
            "archive_may_have_changed_since_copy",
        )
    ]
    needs_action = (
        success is None or True in hints or (latest and latest["status"] != "succeeded")
    )
    if latest and latest["status"] != "succeeded":
        detail = (
            "completion not recorded (running or interrupted)"
            if latest["status"] == "started"
            else "latest attempt " + latest["status"]
        )
    elif success is None:
        detail = "no successful copy recorded; freshness unknown"
    elif True in hints:
        detail = (
            "may need updating"
            if row.get("archive_may_have_changed_since_copy")
            else "needs updating"
        )
    else:
        age = _relative_age(
            datetime.datetime.fromisoformat(success["completed_at"]), now
        )
        detail = f"copied {age}; " + (
            "no newer upstream success recorded"
            if False in hints
            else "freshness unknown"
        )
    click.echo(f"{row['step']}: {detail}")
    if needs_action:
        command = {
            "SD Card": "sd-card",
            "SSD: All Photos": "ssd",
            "SSD: SD Card": "ssd",
            "Remote": "remote",
        }[row["step"]]
        arguments = [command]
        if latest and latest.get("mode") == "mirror":
            arguments += ["--delete", "--dry-run"]
        commands.append(suggested_command(*arguments))


def print_verification_report(report: VerificationReport) -> None:
    """Print one pass/fail line per check, then the overall verdict."""
    verdict = (
        f"All {len(report.checks)} check(s) passed"
        if report.passed
        else f"{len(report.failed)} of {len(report.checks)} check(s) failed"
    )
    if print_terminal_table(
        "Archive verification",
        ("Result", "Check", "Details"),
        [
            ("PASS" if check.passed else "FAIL", check.name, check.detail)
            for check in report.checks
        ],
        styles=["green" if check.passed else "bold red" for check in report.checks],
        caption=verdict,
    ):
        return
    click.echo()
    click.echo("--- Archive verification ---")
    for check in report.checks:
        status = "PASS" if check.passed else "FAIL"
        click.echo(f"  [{status}] {check.name}: {check.detail}")
    click.echo()
    click.echo(verdict)


def print_pipeline_destinations(
    steps: list[tuple[str, Path | str, Path | str, bool]], *, dry_run: bool
) -> None:
    """Show effective targets before the pipeline starts any work."""
    if not steps:
        return
    click.echo("Backup destinations" + (" (dry run)" if dry_run else ""))
    for name, source, destination, delete in steps:
        policy = "ON" if delete else "off"
        click.echo(f"  {name}: {source} → {destination} | deletions: {policy}")


def print_pipeline_summary(summaries: list[BackupSummary]) -> None:
    """Print a table summarizing all pipeline steps."""
    rows: list[tuple[str, str]] = []
    total_elapsed = 0.0
    for s in summaries:
        total_elapsed += s.elapsed_seconds
        if s.error and s.action_required:
            status = f"ACTION REQUIRED: {s.error}"
        elif s.error:
            status = f"ERROR: {s.error}"
        elif s.planned:
            status = "PLANNED"
        elif s.skipped:
            status = "SKIPPED"
            if s.skip_reason:
                status += f" — {s.skip_reason}"
        else:
            parts = ["DRY RUN" if s.dry_run else "OK"]
            if s.files_transferred is not None:
                label = "proposed transfers" if s.dry_run else "files"
                parts.append(f"{s.files_transferred} {label}")
            else:
                parts.append("transfer count unavailable")
            if s.total_size:
                parts.append(s.total_size)
            status = " | ".join(parts)

        rows.append((s.step_name, status))

    overall = _pipeline_outcome(summaries)
    if print_terminal_table(
        "BACKUP PIPELINE SUMMARY",
        ("Step", "Result", "Elapsed"),
        [
            (name, status, f"{s.elapsed_seconds:.1f}s")
            for (name, status), s in zip(rows, summaries)
        ],
        styles=[_summary_style(s) for s in summaries],
        caption=f"{overall} ({total_elapsed:.1f}s)",
    ):
        return
    click.echo()
    click.echo("=" * 60)
    click.echo("BACKUP PIPELINE SUMMARY")
    click.echo("=" * 60)
    for name, status in rows:
        click.echo(f"  {name:<25} {status}")
    click.echo("-" * 60)
    click.echo(f"  {'Total':<25} {overall} ({total_elapsed:.1f}s)")
    click.echo("=" * 60)


def _pipeline_outcome(summaries: list[BackupSummary]) -> str:
    if any(s.error and not s.action_required for s in summaries):
        return "COMPLETED WITH ERRORS"
    if any(s.error and s.action_required for s in summaries):
        return "ACTION REQUIRED"
    if all(s.skipped for s in summaries):
        return "NOTHING TO DO — all steps skipped"
    if any(s.dry_run or s.planned for s in summaries):
        return "PREVIEW COMPLETE"
    return "ALL OK"
