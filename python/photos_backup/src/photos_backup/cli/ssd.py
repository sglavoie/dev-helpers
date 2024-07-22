import click

from photos_backup.ssd.backup import Backup


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
    Backup(
        delete_at_destination=delete,
        dry_run=dry_run,
    ).backup()
