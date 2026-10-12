"""Printers for Apple Photos export, bootstrap, mirror, and cleanup results."""

from __future__ import annotations

from typing import TYPE_CHECKING

import click

from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.cli.context import suggested_command
from photos_backup.summary.models import human_size
from photos_backup.summary.steps import print_summary

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
    from photos_backup.apple_photos.plan import ExportResult
    from photos_backup.apple_photos.takeover import TakeoverCheck


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
    if result.initialized and result.initialized_at is not None:
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
        # A manifest is only ever written for a run with an ID.
        assert outcome.run_id is not None
        click.echo(f"  Manifest: {outcome.manifest_path}")
        click.echo(
            f"  Preview with: {suggested_command('approve-cleanup', outcome.run_id, '--dry-run')}"
        )
        click.echo(
            f"  Approve with: {suggested_command('approve-cleanup', outcome.run_id)}"
        )
        click.echo(
            f"  Discard with: {suggested_command('approve-cleanup', outcome.run_id, '--discard')}"
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
    if discard.manifest is None:
        click.echo(f"  Manifest unavailable: {discard.manifest_problem}")
        return
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


def _print_counts(*labelled: tuple[str, tuple[object, ...]]) -> None:
    for label, items in labelled:
        if items:
            click.echo(f"  {label}: {len(items)}")
