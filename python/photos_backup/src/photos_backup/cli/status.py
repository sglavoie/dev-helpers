import datetime
import json
from dataclasses import asdict
from pathlib import Path

import click

from photos_backup.apple_photos.plan import plan_export
from photos_backup.archive import ArchiveError, open_archive
from photos_backup.cli.context import apple_photos_config_from, config_path_from
from photos_backup.config import (
    MissingSection,
    load_sd_card_config,
    load_ssd_config,
    load_rclone_config,
    resolve_rclone_source,
)
from photos_backup.summary import print_archive_status, print_transfer_history
from photos_backup.transfers import TransferHistory, classify_transfers


def _configured_transfers(config_path: Path | None) -> list[dict]:
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


@click.command(
    name="status",
    help="Show recorded backup state and the next export, without scanning files.",
)
@click.option("--json", "as_json", is_flag=True, help="Print recorded status as JSON.")
@click.pass_context
def status(ctx: click.Context, as_json: bool) -> None:
    try:
        config = apple_photos_config_from(ctx)
    except MissingSection:
        config = None
    config_path = config_path_from(ctx)
    configured = _configured_transfers(config_path)
    receipts, errors = TransferHistory(config_path).read()
    current, historical = classify_transfers(configured, receipts)
    now = datetime.datetime.now(datetime.UTC)
    document = {
        "version": 1,
        "archive": str(config.archive) if config is not None else None,
        "archive_configured": config is not None,
        "observed_at": now.isoformat(),
        "files_verified": False,
        "state": None,
        "next_export": None,
        "archive_error": None,
        "transfers": receipts,
        "configured_transfers": current,
        "historical_transfers": historical,
        "transfer_history_errors": errors,
    }
    archive_error = None
    if config is None:
        if not as_json:
            click.echo("Archive status: not configured ([apple_photos] is absent)")
    else:
        try:
            with open_archive(config, dry_run=True) as archive:
                state = archive.state_store.load()
                now = archive.now()
                plan = plan_export(config, state, now)
                state_document = asdict(state)
                for name, value in state_document.items():
                    if hasattr(value, "isoformat"):
                        state_document[name] = value.isoformat()
                document.update(
                    observed_at=now.isoformat(),
                    state=state_document,
                    next_export={"mode": plan.mode.value, "reason": plan.reason},
                )
                if not as_json:
                    print_archive_status(archive, state, plan, now=now)
        except (ArchiveError, OSError) as error:
            archive_error = (
                error
                if isinstance(error, ArchiveError)
                else click.ClickException(str(error))
            )
            document["archive_error"] = str(error)
            if not as_json:
                click.echo(f"Archive unavailable: {config.archive} — {error}")
    if as_json:
        click.echo(json.dumps(document, indent=2, default=str))
    else:
        print_transfer_history(current, errors, now=now, historical_receipts=historical)
    if archive_error is not None:
        raise archive_error
