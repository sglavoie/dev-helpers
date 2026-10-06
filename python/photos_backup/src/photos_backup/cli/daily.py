import time
from contextlib import ExitStack
from functools import partial

import click

from photos_backup.apple_photos.adapter import run_osxphotos_export
from photos_backup.apple_photos.cleanup import reconcile_mirror
from photos_backup.apple_photos.downloads import DEFAULT_DOWNLOAD_TIMEOUT
from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.takeover import ensure_writer
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from, suggested_command
from photos_backup.errors import ActionRequired
from photos_backup.progress import ExportProgress
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
    started = time.monotonic()
    try:
        with ExportProgress() as progress, ExitStack() as stack:
            with progress.phase("Checking archive"):
                config = apple_photos_config_from(ctx)
                archive = stack.enter_context(open_archive(config, dry_run=dry_run))
                if not archive.state_store.load().initialized:
                    raise ActionRequired(
                        f"Archive '{config.archive}' is not initialized; "
                        f"run `{suggested_command('bootstrap')}` before the first daily run"
                    )
            with progress.phase("Checking archive writer"):
                takeover = ensure_writer(config, archive)
            progress.message(
                f"Missing downloads: {download_timeout}s per asset; no total run limit."
            )
            result = ApplePhotosExport(
                config=config,
                archive=archive,
                plan_only=dry_run,
                progress=progress,
                runner=partial(
                    run_osxphotos_export,
                    download_timeout=download_timeout,
                    progress=progress,
                ),
            ).export()
            if takeover.status is not WriterStatus.UNCHANGED:
                print_takeover_check(takeover, dry_run=dry_run)
            print_export_result(result, dry_run=dry_run)
            with progress.phase("Reconciling archive cleanup"):
                mirror = reconcile_mirror(config, archive, result)
    finally:
        click.echo(f"Total command time: {time.monotonic() - started:.1f}s")

    print_mirror_outcome(mirror, dry_run=dry_run)
    if not result.complete:
        raise click.ClickException(str(result.failure_reason()))
    if mirror.pending:
        raise ActionRequired(
            f"Cleanup run '{mirror.run_id}' needs approval because {mirror.reason}; "
            f"review '{mirror.manifest_path}' and preview with "
            f"`{suggested_command('approve-cleanup', mirror.run_id, '--dry-run')}`"
        )
