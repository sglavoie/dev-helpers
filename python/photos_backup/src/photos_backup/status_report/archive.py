"""Archive and export-attempt status lines."""

from __future__ import annotations

import datetime
from typing import TYPE_CHECKING, Any

import click

from photos_backup.cli.context import suggested_command
from photos_backup.status_report.timefmt import _weekday_when, _when

if TYPE_CHECKING:
    from photos_backup.apple_photos.plan import ExportPlan
    from photos_backup.archive import Archive, ArchiveState


def print_archive_status(
    archive: Archive,
    state: ArchiveState,
    plan: ExportPlan,
    next_full: datetime.datetime | None,
    *,
    now: datetime.datetime,
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
        click.echo(f"  {label}: {_when(value, now) if value else '(never)'}")
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
    if next_full is not None:
        click.echo(f"  Next full export: {_weekday_when(next_full, now)}")


def print_export_attempt(
    attempt: dict[str, Any] | None,
    error: str | None,
    *,
    now: datetime.datetime,
    baseline_report: str | None = None,
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
    click.echo(f"  Started: {_when(attempt['started_at'], now)}")
    if attempt["completed_at"]:
        click.echo(f"  Completed: {_when(attempt['completed_at'], now)}")
    click.echo(f"  Mac: {attempt['hostname']}")
    same_as_baseline = baseline_report and attempt["report_path"] == baseline_report
    click.echo(
        "  Export report: the baseline report above"
        if same_as_baseline
        else f"  Export report (if written): {attempt['report_path']}"
    )
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
