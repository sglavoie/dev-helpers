import click

from photos_backup.apple_photos.export import ApplePhotosExport


@click.command(name="apple-photos", help="Work with Apple Photos.")
@click.option(
    "--testing",
    is_flag=True,
    help="Set useful flags when testing, along with --dry-run.",
)
@click.option(
    "--dl-missing",
    is_flag=True,
    help="Download missing photos from iCloud.",
)
def apple_photos(
    dl_missing: bool,
    testing: bool,
) -> None:
    ApplePhotosExport(
        testing=testing,
        dl_missing=dl_missing,
    ).export()
