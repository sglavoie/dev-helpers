"""The one-line-per-stage short status."""

from __future__ import annotations

import datetime
from typing import TYPE_CHECKING, Any

import click

from photos_backup.cli.context import suggested_command
from photos_backup.status_report.downloads import (
    print_download_summary,
    print_verification_history,
)
from photos_backup.status_report.timefmt import _relative_age
from photos_backup.status_report.transfer_history import (
    _FRESHNESS_HINTS,
    _retry_action,
    _transfer_suggestions,
)

if TYPE_CHECKING:
    pass


def print_short_status(
    document: dict[str, Any], *, now: datetime.datetime
) -> list[str]:
    """Print one line per stage; return the suggested next commands."""
    click.echo("Recorded status — files have not been verified by this command")
    commands: list[str] = []
    _print_short_archive(document, now, commands)
    if document["export_attempt_error"]:
        click.echo(f"Warning: {document['export_attempt_error']}", err=True)
    print_download_summary(
        document["download_summary"], document["download_report_error"]
    )
    for row in document["configured_transfers"]:
        _print_short_transfer(row, now)
    transfers = _transfer_suggestions(document["configured_transfers"])
    commands.extend(command for command, _ in transfers)
    for error in document["transfer_history_errors"]:
        click.echo(f"Warning: {error}", err=True)
    if document["archive_configured"]:
        receipt = document["last_verification"]
        print_verification_history(
            receipt, document["verification_history_error"], now=now
        )
        # Verification cannot run while the archive itself is unavailable.
        if not document["archive_error"] and (
            receipt is None
            or not receipt["passed"]
            or receipt["archive_exported_since_verification"]
            or receipt["archive_may_have_changed_since_verification"]
        ):
            commands.append(suggested_command("verify", "--record"))
    commands = list(dict.fromkeys(commands))
    if commands:
        click.echo("Suggested next commands (mirror retries are previews):")
        for command in commands:
            click.echo(f"  {command}")
    waiting = {command for command, within_grace in transfers if within_grace}
    return [command for command in commands if command not in waiting]


def _print_short_archive(
    document: dict[str, Any], now: datetime.datetime, commands: list[str]
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
            export_status = _short_baseline(document, now, commands)
        click.echo(
            f"Apple Photos: {export_status}; next export "
            + _short_next_export(document["next_export"])
        )
        pending = state["pending_cleanup_run_id"]
        click.echo(f"Cleanup: {'pending ' + pending if pending else 'none pending'}")
        if pending:
            commands.append(suggested_command("approve-cleanup", pending, "--dry-run"))


def _short_baseline(
    document: dict[str, Any], now: datetime.datetime, commands: list[str]
) -> str:
    """Describe a successful export baseline, suggesting daily once it is overdue."""
    timestamp = document["state"]["last_successful_export_at"]
    attempt = document["last_export_attempt"]
    export_status = (
        "baseline " + _relative_age(datetime.datetime.fromisoformat(timestamp), now)
        if timestamp
        else "no successful baseline recorded"
    )
    if attempt and not attempt["baseline_advanced"]:
        export_status += (
            f"; latest {attempt['mode']} export succeeded (baseline unchanged)"
        )
    if document["next_export"]["overdue"]:
        export_status += "; daily export overdue"
        commands.append(suggested_command("daily"))
    return export_status


def _short_next_export(next_export: dict[str, Any]) -> str:
    if not next_export["full_due_at"]:
        return str(next_export["mode"])
    due = datetime.datetime.fromisoformat(next_export["full_due_at"])
    return f"{next_export['mode']} (full {due.astimezone():%a %Y-%m-%d})"


def _print_short_transfer(row: dict[str, Any], now: datetime.datetime) -> None:
    latest, success = row["last_attempt"], row["last_success"]
    hints = [row.get(field) for field in _FRESHNESS_HINTS]
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
            (
                "may need updating"
                if row.get("archive_may_have_changed_since_copy")
                else "needs updating"
            )
            + "; copied "
            + _relative_age(
                datetime.datetime.fromisoformat(success["completed_at"]), now
            )
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
    if row.get("awaiting_card") and _retry_action(row) is not None:
        detail += "; waits for the occasional SD card, not counted by --check"
    click.echo(f"{row['step']}: {detail}")
