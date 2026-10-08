import rich_click as click

from photos_backup.cli.context import config_path_from
from photos_backup.config import load_sd_card_config
from photos_backup.sd_card.backup import Backup
from photos_backup.summary import print_summary
from photos_backup.transfers import TransferHistory


@click.command(
    name="sd-card",
    help="Copy the SD card folder into its configured local destination; never deletes.",
)
@click.option(
    "--dry-run",
    is_flag=True,
    help="List the files rsync would copy without copying them.",
)
@click.pass_context
def sd_card(
    ctx: click.Context,
    dry_run: bool,
) -> None:
    config = load_sd_card_config(config_path_from(ctx))
    summary = Backup(
        config=config,
        dry_run=dry_run,
        history=TransferHistory(config_path_from(ctx)),
    ).backup()
    print_summary(summary)
