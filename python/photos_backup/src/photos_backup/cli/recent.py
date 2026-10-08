from __future__ import annotations

import datetime
from contextlib import ExitStack

import rich_click as click

from photos_backup.apple_photos.downloads import DEFAULT_DOWNLOAD_TIMEOUT
from photos_backup.apple_photos.plan import ExportMode, ExportPlan
from photos_backup.archive import Archive
from photos_backup.cli.context import apple_photos_config_from, suggested_command
from photos_backup.cli.exporting import (
    command_timer,
    export_into_archive,
    print_export_outcome,
)
from photos_backup.progress import ExportProgress


@click.command(help="Back up recent photos/videos without completing a full bootstrap.")
@click.option(
    "--days",
    type=click.IntRange(min=1),
    default=30,
    show_default=True,
    help="Include photos and videos taken in the last N days.",
)
@click.option(
    "--download-timeout",
    type=click.IntRange(min=1),
    default=DEFAULT_DOWNLOAD_TIMEOUT,
    show_default=True,
    help="Seconds allowed for missing-file retrieval per asset, across retries. No total run limit.",
)
@click.option(
    "--dry-run", is_flag=True, help="Show the plan without exporting or downloading."
)
@click.pass_context
def recent(ctx: click.Context, days: int, download_timeout: int, dry_run: bool) -> None:
    def since(archive: Archive) -> ExportPlan:
        start = archive.now() - datetime.timedelta(days=days)
        return ExportPlan(
            ExportMode.RECENT, f"photos/videos taken since {start.isoformat()}", start
        )

    with command_timer(), ExitStack() as stack, ExportProgress() as progress:
        config = apple_photos_config_from(ctx)
        _, takeover, result = export_into_archive(
            stack,
            progress,
            config,
            dry_run=dry_run,
            download_timeout=download_timeout,
            plan=since,
            local_first=True,
        )
    print_export_outcome(takeover, result, dry_run=dry_run)
    if not result.complete:
        raise click.ClickException(
            "Recent backup is incomplete; review the reports and rerun "
            f"`{suggested_command('recent', '--days', str(days), '--download-timeout', str(download_timeout))}` "
            "to retry missing items. Completed files are retained."
        )
