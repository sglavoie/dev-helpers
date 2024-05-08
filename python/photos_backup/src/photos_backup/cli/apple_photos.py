import click

from photos_backup.apple_photos.export import ApplePhotosExport


@click.command(name="apple-photos", help="Work with Apple Photos.")
@click.option(
    "--dry-run",
    is_flag=True,
    help="Dry run.",
)
def apple_photos(
    dry_run: bool,
) -> None:
    ApplePhotosExport(
        dry_run=dry_run,
    ).export()
