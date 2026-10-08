from __future__ import annotations

import rich_click as click

from photos_backup.cli.context import config_path_from
from photos_backup.cli.outcome import raise_for_summaries
from photos_backup.cli.notify import notify_on_problems
from photos_backup.config import load_optional, load_sd_card_config, load_ssd_config
from photos_backup.ssd.backup import Backup
from photos_backup.summary import print_summary
from photos_backup.transfers import TransferHistory


@click.command(
    name="ssd",
    help="Copy the archive and the SD card copy to the second on-site drive.",
)
@click.option(
    "--delete",
    is_flag=True,
    help="Mirror deletions: remove SSD files that are absent from their source.",
)
@click.option(
    "--dry-run",
    is_flag=True,
    help="List the changes rsync would make without changing the SSD.",
)
@notify_on_problems
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
        sd_card=load_optional(lambda: load_sd_card_config(config_path)),
        history=TransferHistory(config_path),
    ).backup()
    for summary in summaries:
        print_summary(summary)
    raise_for_summaries(summaries)
