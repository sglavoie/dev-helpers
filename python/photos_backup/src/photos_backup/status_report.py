"""Recorded archive, transfer, and verification status for `status`."""

from __future__ import annotations

import datetime
from typing import TYPE_CHECKING, Any

import click

from photos_backup.cli.context import suggested_command

if TYPE_CHECKING:
    from photos_backup.apple_photos.plan import ExportPlan
    from photos_backup.archive import Archive, ArchiveState


_SECONDS_PER_MINUTE = 60


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
    attempt: dict[str, Any] | None, error: str | None, *, now: datetime.datetime
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


def _transfer_attention(receipts: list[dict[str, Any]]) -> list[str]:
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
    receipts: list[dict[str, Any]],
    errors: list[str],
    *,
    now: datetime.datetime | None = None,
    historical_receipts: list[dict[str, Any]] | None = None,
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
    receipt: dict[str, Any], *, now: datetime.datetime, retry: bool = False
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


def _print_transfer_freshness(receipt: dict[str, Any]) -> None:
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


_TRANSFER_COMMANDS = {
    "SD Card": "sd-card",
    "SSD: All Photos": "ssd",
    "SSD: SD Card": "ssd",
    "Remote": "remote",
}
_FRESHNESS_HINTS = (
    "archive_exported_since_copy",
    "ssd_copied_since_upload",
    "sd_imported_since_copy",
    "archive_may_have_changed_since_copy",
)


def _transfer_retry(row: dict[str, Any]) -> tuple[str, str] | None:
    """Return the label and command that bring one transfer row up to date.

    Unfinished attempts (including ones whose completion was never recorded)
    are retried; successful copies made stale by a newer upstream run are
    updated. Mirror commands are always previews.
    """
    latest = row["last_attempt"]
    unfinished = latest is not None and latest["status"] != "succeeded"
    stale = any(row.get(field) is True for field in _FRESHNESS_HINTS)
    command = _TRANSFER_COMMANDS.get(row["step"])
    if command is None or not (unfinished or stale or row["last_success"] is None):
        return None
    action = "retry" if unfinished or row["last_success"] is None else "update"
    if latest is not None and latest.get("mode") == "mirror":
        return f"Preview mirror {action}", suggested_command(
            command, "--delete", "--dry-run"
        )
    return f"{action.capitalize()} copy", suggested_command(command)


def _print_transfer_retry(receipt: dict[str, Any]) -> None:
    if retry := _transfer_retry(receipt):
        label, command = retry
        click.echo(f"    {label}: {command}")


def _transfer_mode(attempt: dict[str, Any]) -> str:
    return {
        "copy": "copy (preserves destination-only files)",
        "mirror": "mirror (deletions enabled)",
    }.get(attempt.get("mode") or "", "mode not recorded")


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


def print_short_status(document: dict[str, Any], *, now: datetime.datetime) -> None:
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
    row: dict[str, Any], now: datetime.datetime, commands: list[str]
) -> None:
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
    if retry := _transfer_retry(row):
        commands.append(retry[1])
