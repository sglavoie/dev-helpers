import click

from photos_backup.apple_photos.bootstrap import bootstrap_archive
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from
from photos_backup.summary import print_bootstrap_result


@click.command(
    name="bootstrap",
    help="Fill a fresh archive with one complete export, then initialize it.",
)
@click.option(
    "--dry-run",
    is_flag=True,
    help="Explain the bootstrap without writing or initializing anything.",
)
@click.pass_context
def bootstrap(ctx: click.Context, dry_run: bool) -> None:
    config = apple_photos_config_from(ctx)
    with open_archive(config, dry_run=dry_run) as archive:
        result = bootstrap_archive(config, archive)

    print_bootstrap_result(result, dry_run=dry_run)
    reason = result.blocking_reason()
    if reason is not None and not dry_run:
        raise click.ClickException(
            f"The archive was not initialized because {reason}; "
            "fix the cause and run 'photos-backup bootstrap' again to resume"
        )
