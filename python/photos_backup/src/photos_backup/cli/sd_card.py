import click

from photos_backup.sd_card.backup import Backup


@click.command(name="sd-card", help="Work with an SD card.")
@click.option(
    "--dry-run",
    is_flag=True,
    help="Dry run.",
)
def sd_card(
    dry_run: bool,
) -> None:
    Backup(
        dry_run=dry_run,
    ).backup()
