from __future__ import annotations

import csv
import re
from dataclasses import dataclass
from pathlib import Path

import click


@dataclass
class BackupSummary:
    step_name: str
    files_transferred: int = 0
    total_size: str = ""
    elapsed_seconds: float = 0.0
    skipped: bool = False
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


def parse_apple_photos_csv(csv_path: str) -> dict[str, int]:
    """Parse an osxphotos export CSV report and count rows by status.

    Returns a dict with keys like 'exported', 'updated', 'skipped', 'error',
    'missing', etc. mapped to their counts.
    """
    counts: dict[str, int] = {}
    path = Path(csv_path)
    if not path.exists():
        return counts
    with path.open(newline="") as f:
        reader = csv.DictReader(f)
        for row in reader:
            # osxphotos CSV has an 'export_status' or similar column
            # Common statuses: exported, updated, skipped, error, missing
            status = row.get("export_status", "unknown").lower().strip()
            counts[status] = counts.get(status, 0) + 1
    return counts


def print_summary(summary: BackupSummary) -> None:
    """Print a formatted summary for a single backup step."""
    click.echo()
    click.echo(f"--- {summary.step_name} ---")
    if summary.skipped:
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
