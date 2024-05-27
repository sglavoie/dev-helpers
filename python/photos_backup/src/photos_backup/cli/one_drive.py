import click

from photos_backup.one_drive.backup import Backup


@click.command(name="one-drive", help="Backup to One Drive.")
@click.option(
    "--dry-run",
    is_flag=True,
    help="Dry run.",
)
def one_drive(
    dry_run: bool,
) -> None:
    Backup(
        dry_run=dry_run,
    ).backup()
