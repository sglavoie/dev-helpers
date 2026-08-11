import click

from photos_backup.cli.context import config_path_from
from photos_backup.config import load_sd_card_config
from photos_backup.sd_card.backup import Backup
from photos_backup.summary import print_summary


@click.command(name="sd-card", help="Work with an SD card.")
@click.option(
    "--dry-run",
    is_flag=True,
    help="Dry run.",
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
    ).backup()
    print_summary(summary)
