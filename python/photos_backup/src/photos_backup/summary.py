from __future__ import annotations

import csv
import re
from dataclasses import dataclass
from dataclasses import field as dataclass_field
from pathlib import Path
from typing import TYPE_CHECKING

import click

from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.late_additions import csv_flag

if TYPE_CHECKING:
    from photos_backup.apple_photos.bootstrap import BootstrapResult
    from photos_backup.apple_photos.cleanup import (
        CleanupApproval,
        CleanupDiscard,
        MirrorOutcome,
    )
    from photos_backup.apple_photos.local_export import (
        LocalExportCleanup,
        LocalExportPlan,
    )
    from photos_backup.apple_photos.plan import ExportResult
    from photos_backup.apple_photos.takeover import TakeoverCheck
    from photos_backup.apple_photos.verify import VerificationReport


_SCALE = 1024


@dataclass
class BackupSummary:
    step_name: str
    files_transferred: int = 0
    total_size: str = ""
    elapsed_seconds: float = 0.0
    skipped: bool = False
    planned: bool = False
    error: str | None = None


def parse_rsync_stats(output: str) -> dict[str, int | str]:
    """Parse rsync --stats output for file count and total size."""
    result: dict[str, int | str] = {"files_transferred": 0, "total_size": ""}

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
    click.echo()
    click.echo(f"--- {summary.step_name} ---")
    if summary.planned:
        click.echo("  Status: PLANNED — command was not invoked")
    elif summary.skipped:
        click.echo("  Status: SKIPPED")
    elif summary.error:
        click.echo(f"  Status: ERROR — {summary.error}")
    else:
        click.echo("  Status: OK")
        if summary.files_transferred:
            click.echo(f"  Files transferred: {summary.files_transferred}")
        if summary.total_size:
            click.echo(f"  Total size: {summary.total_size}")
    click.echo(f"  Elapsed: {summary.elapsed_seconds:.1f}s")


def print_export_result(result: ExportResult, *, dry_run: bool = False) -> None:
    """Print the export plan, its reports, and the resulting step summary."""
    prefix = "Would export" if dry_run else "Exported"
    click.echo(f"{prefix} ({result.plan.mode.value}): {result.plan.reason}")
    if result.missing_count:
        click.echo(f"  Missing from iCloud: {result.missing_count}")
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
        click.echo(f"  Approve with: photos-backup approve-cleanup {outcome.run_id}")


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


def print_verification_report(report: VerificationReport) -> None:
    """Print one pass/fail line per check, then the overall verdict."""
    click.echo()
    click.echo("--- Archive verification ---")
    for check in report.checks:
        status = "PASS" if check.passed else "FAIL"
        click.echo(f"  [{status}] {check.name}: {check.detail}")
    click.echo()
    if report.passed:
        click.echo(f"All {len(report.checks)} check(s) passed")
    else:
        click.echo(f"{len(report.failed)} of {len(report.checks)} check(s) failed")


def print_pipeline_summary(summaries: list[BackupSummary]) -> None:
    """Print a table summarizing all pipeline steps."""
    click.echo()
    click.echo("=" * 60)
    click.echo("BACKUP PIPELINE SUMMARY")
    click.echo("=" * 60)

    total_elapsed = 0.0
    has_errors = False

    for s in summaries:
        total_elapsed += s.elapsed_seconds
        if s.error:
            has_errors = True
            status = f"ERROR: {s.error}"
        elif s.planned:
            status = "PLANNED"
        elif s.skipped:
            status = "SKIPPED"
        else:
            parts = ["OK"]
            if s.files_transferred:
                parts.append(f"{s.files_transferred} files")
            if s.total_size:
                parts.append(s.total_size)
            status = " | ".join(parts)

        click.echo(f"  {s.step_name:<25} {status}")

    click.echo("-" * 60)
    overall = "COMPLETED WITH ERRORS" if has_errors else "ALL OK"
    click.echo(f"  {'Total':<25} {overall} ({total_elapsed:.1f}s)")
    click.echo("=" * 60)
