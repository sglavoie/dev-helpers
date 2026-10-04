import time
from contextlib import ExitStack
from functools import partial

import click

from photos_backup.apple_photos.adapter import run_osxphotos_export
from photos_backup.apple_photos.bootstrap import bootstrap_archive
from photos_backup.apple_photos.downloads import DEFAULT_DOWNLOAD_TIMEOUT
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from, suggested_command
from photos_backup.summary import print_bootstrap_result
from photos_backup.progress import ExportProgress


@click.command(
    name="bootstrap",
    help="Fill a fresh archive with one complete export, then initialize it.",
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
    help="Explain the bootstrap without writing or initializing anything.",
)
@click.pass_context
def bootstrap(ctx: click.Context, dry_run: bool, download_timeout: int) -> None:
    started = time.monotonic()
    try:
        with ExportProgress() as progress, ExitStack() as stack:
            with progress.phase("Checking archive"):
                config = apple_photos_config_from(ctx)
                archive = stack.enter_context(open_archive(config, dry_run=dry_run))
            progress.message(
                f"Missing downloads: {download_timeout}s per asset; no total run limit."
            )
            result = bootstrap_archive(
                config,
                archive,
                progress=progress,
                runner=partial(
                    run_osxphotos_export,
                    download_timeout=download_timeout,
                    progress=progress,
                ),
            )
    finally:
        click.echo(f"Total command time: {time.monotonic() - started:.1f}s")

    print_bootstrap_result(result, dry_run=dry_run)
    reason = result.blocking_reason()
    if reason is not None and not dry_run:
        raise click.ClickException(
            f"The archive was not initialized because {reason}; "
            f"fix the cause and run `{suggested_command('bootstrap', '--download-timeout', str(download_timeout))}` again to resume"
        )
