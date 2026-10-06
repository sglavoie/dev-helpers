"""Informational destination capacity, after the workflow's path safety checks."""

from pathlib import Path
from shutil import disk_usage

import click

from photos_backup.summary import human_size


def print_destination_space(destination: Path) -> None:
    """Use an existing ancestor for a target that has not been created yet."""
    existing = destination
    try:
        while True:
            try:
                free = disk_usage(existing).free
                break
            except FileNotFoundError:
                if existing == existing.parent:
                    raise
                existing = existing.parent
    except OSError as error:
        click.echo(
            f"Destination free space unavailable for '{destination}': {error}", err=True
        )
        return
    click.echo(
        f"Destination filesystem free space: {human_size(free)} ({destination}); "
        "required backup size has not been estimated.",
        err=True,
    )
