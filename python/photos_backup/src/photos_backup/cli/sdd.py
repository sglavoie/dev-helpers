import click

from photos_backup.sdd.backup import Backup


@click.command(name="sdd", help="Backup to an external drive.")
@click.option(
    "--dry-run",
    is_flag=True,
    help="Dry run.",
)
def sdd(
    dry_run: bool,
) -> None:
    Backup(
        dry_run=dry_run,
    ).backup()
