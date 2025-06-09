import os

import click
from dotenv import load_dotenv

from photos_backup.cli.apple_photos import apple_photos
from photos_backup.cli.sd_card import sd_card
from photos_backup.cli.ssd import ssd


HOME = os.path.expanduser("~")
ENV_PATH = f"{HOME}/.osxphotos.env"

if not os.path.exists(ENV_PATH):
    raise FileNotFoundError(f"Could not find {ENV_PATH}")

load_dotenv(ENV_PATH)


@click.group()
def cli() -> None:
    """Pass `--help` to any command to see its usage."""


cli.add_command(apple_photos)
cli.add_command(sd_card)
cli.add_command(ssd)


if __name__ == "__main__":
    cli()
