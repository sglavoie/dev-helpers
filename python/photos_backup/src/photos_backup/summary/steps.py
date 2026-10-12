"""Printers for backup steps, the pipeline, and archive verification."""

from __future__ import annotations

from pathlib import Path
from typing import TYPE_CHECKING

import click

from photos_backup.presentation import print_terminal_table
from photos_backup.summary.models import BackupSummary

if TYPE_CHECKING:
    from photos_backup.apple_photos.verify import VerificationReport


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
