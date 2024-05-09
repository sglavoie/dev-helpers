import click

from photos_backup.apple_photos.export import ApplePhotosExport


@click.command(name="apple-photos", help="Work with Apple Photos.")
@click.option(
    "--testing",
    is_flag=True,
    help="Set useful flags when testing, along with --dry-run.",
)
def apple_photos(
    testing: bool,
) -> None:
    ApplePhotosExport(
        testing=testing,
    ).export()
