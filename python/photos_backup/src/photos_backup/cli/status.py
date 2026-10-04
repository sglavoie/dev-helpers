import click

from photos_backup.apple_photos.plan import plan_export
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from
from photos_backup.summary import print_archive_status


@click.command(
    name="status",
    help="Show recorded backup state and the next export, without scanning files.",
)
@click.pass_context
def status(ctx: click.Context) -> None:
    config = apple_photos_config_from(ctx)
    with open_archive(config, dry_run=True) as archive:
        state = archive.state_store.load()
        now = archive.now()
        plan = plan_export(config, state, now)
        print_archive_status(archive, state, plan, now=now)
