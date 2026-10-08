from contextlib import ExitStack

import rich_click as click

from photos_backup.apple_photos.cleanup import reconcile_mirror
from photos_backup.apple_photos.downloads import DEFAULT_DOWNLOAD_TIMEOUT
from photos_backup.cli.context import (
    apple_photos_config_from,
    config_path_from,
    suggested_command,
)
from photos_backup.cli.exporting import (
    command_timer,
    export_into_archive,
    print_export_outcome,
)
from photos_backup.cli.notify import notify_on_problems
from photos_backup.cli.verify import scan_archive, verification_failure
from photos_backup.errors import ActionRequired
from photos_backup.progress import ExportProgress
from photos_backup.summary import print_mirror_outcome, print_verification_report
from photos_backup.verification_history import VerificationHistory


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
@click.option(
    "--verify",
    "verify_after",
    is_flag=True,
    help="After a complete export, verify the archive and record the result "
    "for status, as `verify --record` would.",
)
@notify_on_problems
@click.pass_context
def daily(
    ctx: click.Context, dry_run: bool, download_timeout: int, verify_after: bool
) -> None:
    report = None
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
        if verify_after and result.complete and not dry_run:
            # Still under the archive lock, so nothing can export in between.
            report, document = scan_archive(archive)
            VerificationHistory(config_path_from(ctx), config.archive).record(document)

    print_mirror_outcome(mirror, dry_run=dry_run)
    if report is not None:
        print_verification_report(report)
    if not result.complete:
        raise click.ClickException(str(result.failure_reason()))
    failure = verification_failure(report) if report is not None else None
    if failure is not None and not isinstance(failure, ActionRequired):
        raise failure
    if mirror.pending:
        assert mirror.run_id is not None
        raise ActionRequired(
            f"Cleanup run '{mirror.run_id}' needs approval because {mirror.reason}; "
            f"review '{mirror.manifest_path}' and preview with "
            f"`{suggested_command('approve-cleanup', mirror.run_id, '--dry-run')}`"
        )
    if failure is not None:
        raise failure
