"""Transfer receipts, freshness, and retry suggestions."""

from __future__ import annotations

import datetime
from typing import TYPE_CHECKING, Any

import click

from photos_backup.cli.context import suggested_command
from photos_backup.status_report.timefmt import _when

if TYPE_CHECKING:
    pass


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
        return _when(value, now)

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


def _retry_action(row: dict[str, Any]) -> tuple[str, str, bool] | None:
    """Return the action, command, and mirror mode that update one transfer row.

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
    mirror = latest is not None and latest.get("mode") == "mirror"
    return action, command, mirror


def _transfer_retry(row: dict[str, Any]) -> tuple[str, str] | None:
    """Return the label and command that bring one transfer row up to date."""
    if (retry := _retry_action(row)) is None:
        return None
    action, command, mirror = retry
    if mirror:
        label = f"Preview mirror {action}"
        suggestion = suggested_command(command, "--delete", "--dry-run")
    else:
        label = f"{action.capitalize()} copy"
        suggestion = suggested_command(command)
    return label, suggestion + _connect_note([row])


def _connect_note(rows: list[dict[str, Any]]) -> str:
    """A shell comment naming unplugged drives keeps a suggestion pasteable."""
    drives: list[str] = list(
        dict.fromkeys(
            row["disconnected_drive"] for row in rows if row.get("disconnected_drive")
        )
    )
    if not drives:
        return ""
    listed = " and ".join(filter(None, [", ".join(drives[:-1]), drives[-1]]))
    return f"  # connect {listed} first"


def _transfer_suggestions(rows: list[dict[str, Any]]) -> list[tuple[str, bool]]:
    """Return each suggested copy command and whether `--check` may let it wait.

    Two or more plain copies collapse into one copy-only `backup-all`, which
    skips the configured steps that are already current. Mirror previews stay
    separate, since `backup-all` would not preview them. So does a step held up
    only by an absent occasional SD card, so nothing else asks for that card.
    """
    copies: dict[str, list[dict[str, Any]]] = {}
    awaiting: dict[str, list[dict[str, Any]]] = {}
    previews: list[tuple[str, list[dict[str, Any]]]] = []
    for row in rows:
        if (retry := _retry_action(row)) is None:
            continue
        _, step, mirror = retry
        if mirror:
            command = suggested_command(step, "--delete", "--dry-run")
            previews.append((command, [row]))
        elif row.get("awaiting_card"):
            awaiting.setdefault(step, []).append(row)
        else:
            copies.setdefault(step, []).append(row)
    for step in [step for step in awaiting if step in copies]:
        copies[step].extend(awaiting.pop(step))
    groups: list[tuple[str, list[dict[str, Any]]]] = []
    if len(copies) > 1:
        configured = dict.fromkeys(_TRANSFER_COMMANDS[row["step"]] for row in rows)
        skips = [f"--skip-{step}" for step in configured if step not in copies]
        merged = [row for group in copies.values() for row in group]
        command = suggested_command("backup-all", "--skip-apple-photos", *skips)
        groups.append((command, merged))
    else:
        groups.extend(
            (suggested_command(step), group) for step, group in copies.items()
        )
    groups.extend((suggested_command(step), group) for step, group in awaiting.items())
    return [
        (
            command + _connect_note(group),
            all(row.get("within_grace") or row.get("awaiting_card") for row in group),
        )
        for command, group in groups + previews
    ]


def _print_transfer_retry(receipt: dict[str, Any]) -> None:
    if retry := _transfer_retry(receipt):
        label, command = retry
        click.echo(f"    {label}: {command}")


def _transfer_mode(attempt: dict[str, Any]) -> str:
    return {
        "copy": "copy (preserves destination-only files)",
        "mirror": "mirror (deletions enabled)",
    }.get(attempt.get("mode") or "", "mode not recorded")
