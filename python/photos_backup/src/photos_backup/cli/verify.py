import click

from photos_backup.apple_photos.verify import verify_archive
from photos_backup.archive import open_archive
from photos_backup.cli.context import config_path_from
from photos_backup.config import load_apple_photos_config
from photos_backup.summary import print_verification_report


@click.command(
    name="verify",
    help="Report the health of the shared archive without changing anything.",
)
@click.pass_context
def verify(ctx: click.Context) -> None:
    config = load_apple_photos_config(config_path_from(ctx))
    with open_archive(config, dry_run=True) as archive:
        report = verify_archive(archive)

    print_verification_report(report)
    if not report.passed:
        failed = ", ".join(check.name for check in report.failed)
        raise click.ClickException(f"Archive check(s) failed: {failed}")
