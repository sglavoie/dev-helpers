import click

from photos_backup.cli.apple_photos import apple_photos
from photos_backup.cli.backup_all import backup_all
from photos_backup.cli.remote import remote
from photos_backup.cli.sd_card import sd_card
from photos_backup.cli.ssd import ssd


@click.group()
def cli() -> None:
    """Pass `--help` to any command to see its usage."""


cli.add_command(apple_photos)
cli.add_command(backup_all)
cli.add_command(remote)
cli.add_command(sd_card)
cli.add_command(ssd)


if __name__ == "__main__":
    cli()
