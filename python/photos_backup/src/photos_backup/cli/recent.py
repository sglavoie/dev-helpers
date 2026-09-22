from __future__ import annotations

import datetime
from functools import partial

import click

from photos_backup.apple_photos.adapter import run_osxphotos_export
from photos_backup.apple_photos.downloads import DEFAULT_DOWNLOAD_TIMEOUT
from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.plan import ExportMode, ExportPlan
from photos_backup.apple_photos.takeover import ensure_writer
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from
from photos_backup.summary import print_export_result


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
    config = apple_photos_config_from(ctx)
    with open_archive(config, dry_run=dry_run) as archive:
        ensure_writer(config, archive)
        start = archive.now() - datetime.timedelta(days=days)
        plan = ExportPlan(
            ExportMode.RECENT, f"photos/videos taken since {start.isoformat()}", start
        )
        click.echo(
            f"Missing downloads: {download_timeout}s per asset; no total run limit."
        )
        result = ApplePhotosExport(
            config,
            archive,
            plan=plan,
            plan_only=dry_run,
            runner=partial(
                run_osxphotos_export,
                download_timeout=download_timeout,
                local_first=True,
            ),
        ).export()
    print_export_result(result, dry_run=dry_run)
    if result.report_path is not None:
        click.echo(f"Export report: {result.report_path}")
    if not result.clean or result.missing_count:
        raise click.ClickException(
            "Recent backup is incomplete; review the reports and rerun 'photos-backup recent' "
            "to retry missing items. Completed files are retained."
        )
