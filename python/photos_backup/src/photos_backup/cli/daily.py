from contextlib import ExitStack

import rich_click as click

from photos_backup.apple_photos.cleanup import reconcile_mirror
from photos_backup.apple_photos.downloads import DEFAULT_DOWNLOAD_TIMEOUT
from photos_backup.cli.context import apple_photos_config_from, suggested_command
from photos_backup.cli.exporting import (
    command_timer,
    export_into_archive,
    print_export_outcome,
)
from photos_backup.errors import ActionRequired
from photos_backup.progress import ExportProgress
from photos_backup.summary import print_mirror_outcome


@click.command(
    name="daily",
    help="Export Apple Photos into the shared archive on the configured cadence.",
)
@click.option(
    "--download-timeout",
    type=click.IntRange(min=1),
    default=DEFAULT_DOWNLOAD_TIMEOUT,
    show_default=True,
    help="Seconds allowed for missing-file retrieval per asset, across retries. No total run limit.",
)
@click.option(
    "--dry-run",
    is_flag=True,
    help="Report the export that would run without writing anything.",
)
@click.pass_context
def daily(ctx: click.Context, dry_run: bool, download_timeout: int) -> None:
    with command_timer(), ExitStack() as stack:
        with ExportProgress() as progress:
            config = apple_photos_config_from(ctx)
            archive, takeover, result = export_into_archive(
                stack,
                progress,
                config,
                dry_run=dry_run,
                download_timeout=download_timeout,
                require_initialized="daily",
            )
        # The live status line is closed, so results print on their own
        # lines; the export result still precedes cleanup reconciliation.
        print_export_outcome(takeover, result, dry_run=dry_run)
        with (
            ExportProgress() as progress,
            progress.phase("Reconciling archive cleanup"),
        ):
            mirror = reconcile_mirror(config, archive, result)

    print_mirror_outcome(mirror, dry_run=dry_run)
    if not result.complete:
        raise click.ClickException(str(result.failure_reason()))
    if mirror.pending:
        assert mirror.run_id is not None
        raise ActionRequired(
            f"Cleanup run '{mirror.run_id}' needs approval because {mirror.reason}; "
            f"review '{mirror.manifest_path}' and preview with "
            f"`{suggested_command('approve-cleanup', mirror.run_id, '--dry-run')}`"
        )
