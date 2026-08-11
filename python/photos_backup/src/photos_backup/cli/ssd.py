from __future__ import annotations

from pathlib import Path

import click

from photos_backup.cli.context import config_path_from
from photos_backup.config import (
    MissingSection,
    SdCardConfig,
    load_sd_card_config,
    load_ssd_config,
)
from photos_backup.ssd.backup import Backup
from photos_backup.summary import print_summary


@click.command(name="ssd", help="Backup to an external drive.")
@click.option(
    "--delete",
    is_flag=True,
    help="Whether to delete files at the destination that are not at the source.",
)
@click.option(
    "--dry-run",
    is_flag=True,
    help="Dry run.",
)
@click.pass_context
def ssd(
    ctx: click.Context,
    delete: bool,
    dry_run: bool,
) -> None:
    config_path = config_path_from(ctx)
    summaries = Backup(
        config=load_ssd_config(config_path),
        delete_at_destination=delete,
        dry_run=dry_run,
        sd_card=load_optional_sd_card_config(config_path),
    ).backup()
    for summary in summaries:
        print_summary(summary)


def load_optional_sd_card_config(config_path: Path | None) -> SdCardConfig | None:
    try:
        return load_sd_card_config(config_path)
    except MissingSection:
        return None
