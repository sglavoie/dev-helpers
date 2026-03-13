import click

from photos_backup.config import Config
from photos_backup.sd_card.backup import Backup
from photos_backup.summary import print_summary


@click.command(name="sd-card", help="Work with an SD card.")
@click.option(
    "--dry-run",
    is_flag=True,
    help="Dry run.",
)
def sd_card(
    dry_run: bool,
) -> None:
    config = Config.from_env()
    summary = Backup(
        config=config,
        dry_run=dry_run,
    ).backup()
    print_summary(summary)
