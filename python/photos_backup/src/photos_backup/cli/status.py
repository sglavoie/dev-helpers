import datetime
import json
from dataclasses import asdict
from pathlib import Path
from typing import Any

import rich_click as click

from photos_backup.apple_photos.plan import (
    ExportMode,
    full_export_overdue,
    next_full_export_at,
    plan_export,
)
from photos_backup.apple_photos.attempt import ExportAttemptStore
from photos_backup.apple_photos.download_report import summarize_downloads
from photos_backup.archive import Archive, ArchiveError, open_archive
from photos_backup.cli.context import apple_photos_config_from, config_path_from
from photos_backup.cli.notify import notify_on_problems
from photos_backup.config import (
    ApplePhotosConfig,
    MissingSection,
    load_sd_card_config,
    load_ssd_config,
    load_rclone_config,
    load_status_config,
    resolve_rclone_source,
)
from photos_backup.copy_safety import disconnected_volume
from photos_backup.errors import ActionRequired
from photos_backup.status_report import (
    print_archive_status,
    print_export_attempt,
    print_transfer_history,
    print_short_status,
    print_download_summary,
    print_verification_history,
)
from photos_backup.verification_history import (
    VerificationHistory,
    annotate_verification,
)
from photos_backup.transfers import (
    TransferHistory,
    classify_transfers,
    annotate_archive_freshness,
    annotate_copy_grace,
    annotate_upstream_freshness,
)


def _configured_transfers(config_path: Path | None) -> list[dict[str, Any]]:
    try:
        sd_card = load_sd_card_config(config_path)
    except MissingSection:
        sd_card = None
    try:
        ssd = load_ssd_config(config_path)
    except MissingSection:
        ssd = None
    try:
        remote = load_rclone_config(config_path)
    except MissingSection:
        remote = None

    copies: list[tuple[str, Path, Path | str]] = []
    if sd_card is not None:
        copies.append(
            ("SD Card", sd_card.source, sd_card.destination / sd_card.source.name)
        )
    if ssd is not None:
        copies.append(
            ("SSD: All Photos", ssd.source, ssd.destination / ssd.source.name)
        )
        if sd_card is not None:
            copies.append(
                (
                    "SSD: SD Card",
                    sd_card.destination,
                    ssd.destination / sd_card.destination.name,
                )
            )
    if remote is not None:
        copies.append(
            ("Remote", resolve_rclone_source(remote, config_path), remote.remote)
        )
    return [
        {"step": step, "source": str(source), "destination": str(destination)}
        for step, source, destination in copies
    ]


def _annotate_disconnected_drives(rows: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Note an unplugged copy drive, so status can say to connect it first."""
    return [
        {
            **row,
            "disconnected_drive": disconnected_volume(row["source"])
            or disconnected_volume(row["destination"]),
        }
        for row in rows
    ]


@click.command(
    name="status",
    help="Show recorded backup state and the next export, without scanning files.",
)
@click.option("--json", "as_json", is_flag=True, help="Print recorded status as JSON.")
@click.option(
    "--short",
    "short",
    is_flag=True,
    help="Show a compact status and suggested next commands.",
)
@click.option(
    "--check",
    is_flag=True,
    help="Show the compact status and exit 3 when it suggests a command; "
    "for scheduled checks.",
)
@notify_on_problems
@click.pass_context
def status(ctx: click.Context, as_json: bool, short: bool, check: bool) -> None:
    if (short or check) and as_json:
        raise click.UsageError("Choose either --short/--check or --json")
    short = short or check
    try:
        config = apple_photos_config_from(ctx)
    except MissingSection:
        config = None
    config_path = config_path_from(ctx)
    configured = _configured_transfers(config_path)
    receipts, errors = TransferHistory(config_path).read()
    current, historical = classify_transfers(configured, receipts)
    current = _annotate_disconnected_drives(current)
    current = annotate_upstream_freshness(current)
    current = annotate_archive_freshness(
        current, config.archive if config else None, None
    )
    now = datetime.datetime.now(datetime.UTC)
    current = annotate_copy_grace(
        current, load_status_config(config_path).copy_max_age_days, now
    )
    verification, verification_error = (
        VerificationHistory(config_path, config.archive).read()
        if config
        else (None, None)
    )
    document: dict[str, Any] = {
        "version": 1,
        "archive": str(config.archive) if config is not None else None,
        "archive_configured": config is not None,
        "observed_at": now.isoformat(),
        "files_verified": False,
        "state": None,
        "next_export": None,
        "archive_error": None,
        "last_export_attempt": None,
        "export_attempt_error": None,
        "download_summary": None,
        "download_report_error": None,
        "last_verification": annotate_verification(verification, None, None, False),
        "verification_history_error": verification_error,
        "transfers": receipts,
        "configured_transfers": current,
        "historical_transfers": historical,
        "transfer_history_errors": errors,
    }
    archive_error = None
    suggested: list[str] = []
    if config is None:
        if not as_json and not short:
            click.echo("Archive status: not configured ([apple_photos] is absent)")
    else:
        try:
            with open_archive(config, dry_run=True) as archive:
                now = _read_archive_status(
                    archive,
                    config,
                    document,
                    verification,
                    detailed=not as_json and not short,
                )
                current = document["configured_transfers"]
        except (ArchiveError, OSError) as error:
            archive_error = (
                error
                if isinstance(error, ArchiveError)
                else click.ClickException(str(error))
            )
            document["archive_error"] = str(error)
            if not as_json and not short:
                click.echo(f"Archive unavailable: {config.archive} — {error}")
    if as_json:
        click.echo(json.dumps(document, indent=2, default=str))
    elif short:
        suggested = print_short_status(document, now=now)
    else:
        if config:
            print_verification_history(
                document["last_verification"], verification_error, now=now
            )
        print_transfer_history(current, errors, now=now, historical_receipts=historical)
    if archive_error is not None:
        raise archive_error
    _require_nothing_suggested(suggested, check=check)


def _require_nothing_suggested(suggested: list[str], *, check: bool) -> None:
    """`status --check`: exit 3 so a scheduled check can notify a person."""
    if check and suggested:
        # A suggestion may end in a `# connect ... first` shell comment, which
        # reads badly in a notification, so lead with the drive instead.
        command, _, note = suggested[0].partition("  # ")
        note = note.removesuffix(" first")
        message = f"{note}, then run: {command}" if note else command
        message = f"Next: {message}"
        if len(suggested) > 1:
            message += f" (+{len(suggested) - 1} more; see status --short)"
        raise ActionRequired(message)


def _latest_export_at(
    baseline: datetime.datetime | None, attempt: dict[str, Any] | None
) -> datetime.datetime | None:
    candidates = [baseline] if baseline else []
    if attempt:
        recorded = attempt.get("last_successful_export_at")
        if recorded:
            candidates.append(datetime.datetime.fromisoformat(recorded))
        if attempt["status"] == "succeeded":
            candidates.append(datetime.datetime.fromisoformat(attempt["completed_at"]))
    return max(candidates) if candidates else None


def _read_archive_status(
    archive: Archive,
    config: ApplePhotosConfig,
    document: dict[str, Any],
    verification: dict[str, Any] | None,
    *,
    detailed: bool,
) -> datetime.datetime:
    attempt, attempt_error = ExportAttemptStore(archive).read()
    document.update(last_export_attempt=attempt, export_attempt_error=attempt_error)
    downloads, download_error = summarize_downloads(attempt, archive.paths.archive)
    document.update(download_summary=downloads, download_report_error=download_error)
    state = archive.state_store.load()
    exported_at = _latest_export_at(state.last_successful_export_at, attempt)
    current = annotate_archive_freshness(
        document["configured_transfers"], config.archive, exported_at, attempt
    )
    document["last_verification"] = annotate_verification(
        verification, exported_at, attempt, True
    )
    document["configured_transfers"] = current
    now = archive.now()
    plan = plan_export(config, state, now)
    next_full = (
        next_full_export_at(config, state)
        if plan.mode is ExportMode.INCREMENTAL
        else None
    )
    state_document = asdict(state)
    for name, value in state_document.items():
        if hasattr(value, "isoformat"):
            state_document[name] = value.isoformat()
    document.update(
        observed_at=now.isoformat(),
        state=state_document,
        next_export={
            "mode": plan.mode.value,
            "reason": plan.reason,
            "overdue": state.initialized and full_export_overdue(config, state, now),
            "full_due_at": next_full.isoformat() if next_full else None,
        },
    )
    if detailed:
        print_archive_status(archive, state, plan, next_full, now=now)
        print_export_attempt(
            attempt,
            attempt_error,
            now=now,
            baseline_report=str(state.last_report_path)
            if state.last_report_path
            else None,
        )
        print_download_summary(downloads, download_error)
    return now
