"""Download and verification history lines."""

from __future__ import annotations

import datetime
from typing import Any

import click

from photos_backup.status_report.timefmt import _relative_age


def print_download_summary(summary: dict[str, Any] | None, error: str | None) -> None:
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
    receipt: dict[str, Any] | None, error: str | None, *, now: datetime.datetime
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
