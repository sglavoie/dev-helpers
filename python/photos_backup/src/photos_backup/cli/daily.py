import click

from photos_backup.apple_photos.cleanup import reconcile_mirror
from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.takeover import ensure_writer
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from
from photos_backup.errors import ActionRequired
from photos_backup.summary import (
    print_export_result,
    print_mirror_outcome,
    print_takeover_check,
)


@click.command(
    name="daily",
    help="Export Apple Photos into the shared archive on the configured cadence.",
)
@click.option(
    "--dry-run",
    is_flag=True,
    help="Report the export that would run without writing anything.",
)
@click.pass_context
def daily(ctx: click.Context, dry_run: bool) -> None:
    config = apple_photos_config_from(ctx)
    with open_archive(config, dry_run=dry_run) as archive:
        if not archive.state_store.load().initialized:
            raise ActionRequired(
                f"Archive '{config.archive}' is not initialized; "
                "run 'photos-backup bootstrap' before the first daily run"
            )
        takeover = ensure_writer(config, archive)
        result = ApplePhotosExport(
            config=config, archive=archive, plan_only=dry_run
        ).export()
        mirror = reconcile_mirror(config, archive, result)

    if takeover.status is not WriterStatus.UNCHANGED:
        print_takeover_check(takeover, dry_run=dry_run)
    print_export_result(result, dry_run=dry_run)
    print_mirror_outcome(mirror, dry_run=dry_run)
    if not result.clean:
        raise click.ClickException(str(result.failure_reason()))
    if mirror.pending:
        raise ActionRequired(
            f"Cleanup run '{mirror.run_id}' needs approval because {mirror.reason}; "
            f"review '{mirror.manifest_path}' and run "
            f"'photos-backup approve-cleanup {mirror.run_id}'"
        )
