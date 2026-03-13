import click

from photos_backup.config import Config
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
def ssd(
    delete: bool,
    dry_run: bool,
) -> None:
    config = Config.from_env()
    summaries = Backup(
        config=config,
        delete_at_destination=delete,
        dry_run=dry_run,
    ).backup()
    for summary in summaries:
        print_summary(summary)
