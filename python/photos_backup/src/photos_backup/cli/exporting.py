"""The Apple Photos export sequence shared by daily, recent, and backup-all."""

from __future__ import annotations

import time
from collections.abc import Callable, Iterator
from contextlib import ExitStack, contextmanager
from functools import partial

import click

from photos_backup.apple_photos.adapter import run_osxphotos_export
from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.plan import ExportPlan, ExportResult
from photos_backup.apple_photos.takeover import TakeoverCheck, ensure_writer
from photos_backup.archive import Archive, open_archive
from photos_backup.cli.context import suggested_command
from photos_backup.config import ApplePhotosConfig
from photos_backup.errors import ActionRequired
from photos_backup.progress import ExportProgress
from photos_backup.summary import print_export_result, print_takeover_check


@contextmanager
def command_timer() -> Iterator[None]:
    """Print the wall-clock time of a command, even when it fails."""
    started = time.monotonic()
    try:
        yield
    finally:
        click.echo(f"Total command time: {time.monotonic() - started:.1f}s")


def export_into_archive(
    stack: ExitStack,
    progress: ExportProgress,
    config: ApplePhotosConfig,
    *,
    dry_run: bool,
    download_timeout: int,
    plan: Callable[[Archive], ExportPlan] | None = None,
    local_first: bool = False,
    require_initialized: str | None = None,
) -> tuple[Archive, TakeoverCheck, ExportResult]:
    """Open the archive on `stack`, claim its writer, and export into it.

    `require_initialized` names the command that refuses an archive bootstrap
    has not finished; the archive stays open on `stack` for any follow-up work.
    """
    with progress.phase("Checking archive"):
        archive = stack.enter_context(open_archive(config, dry_run=dry_run))
        if require_initialized and not archive.state_store.load().initialized:
            raise ActionRequired(
                f"Archive '{config.archive}' is not initialized; run "
                f"`{suggested_command('bootstrap')}` before the first "
                f"{require_initialized} run"
            )
    with progress.phase("Checking archive writer"):
        takeover = ensure_writer(config, archive)
    progress.message(
        f"Missing downloads: {download_timeout}s per asset; no total run limit."
    )
    result = ApplePhotosExport(
        config=config,
        archive=archive,
        plan=plan(archive) if plan is not None else None,
        plan_only=dry_run,
        progress=progress,
        runner=partial(
            run_osxphotos_export,
            download_timeout=download_timeout,
            local_first=local_first,
            progress=progress,
        ),
    ).export()
    return archive, takeover, result


def print_export_outcome(
    takeover: TakeoverCheck, result: ExportResult, *, dry_run: bool
) -> None:
    """Print a writer change, then the export; call once progress has closed."""
    if takeover.status is not WriterStatus.UNCHANGED:
        print_takeover_check(takeover, dry_run=dry_run)
    print_export_result(result, dry_run=dry_run)
