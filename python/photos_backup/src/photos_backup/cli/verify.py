import datetime
import json
import time
from contextlib import ExitStack
from dataclasses import asdict
from pathlib import Path
from typing import Any

import rich_click as click

from photos_backup.apple_photos.verify import PENDING_CLEANUP, verify_archive
from photos_backup.archive import ArchiveError, open_archive
from photos_backup.cli.context import apple_photos_config_from, config_path_from
from photos_backup.errors import ActionRequired
from photos_backup.progress import ExportProgress
from photos_backup.summary import print_verification_report
from photos_backup.verification_history import VerificationHistory


@click.command(
    name="verify",
    help="Report the health of the shared archive without changing anything.",
)
@click.option(
    "--report",
    "report_path",
    type=click.Path(dir_okay=False, path_type=Path),
    help="Save all findings as JSON to a new file outside the archive.",
)
@click.option("--json", "as_json", is_flag=True, help="Print all findings as JSON.")
@click.option(
    "--record",
    is_flag=True,
    help="Remember this result locally for status; never write to the archive.",
)
@click.pass_context
def verify(
    ctx: click.Context, report_path: Path | None, as_json: bool, record: bool
) -> None:
    config = apple_photos_config_from(ctx)
    started_at = datetime.datetime.now(datetime.UTC)
    started = time.monotonic()

    def timing() -> dict[str, Any]:
        return {
            "started_at": started_at.isoformat(),
            "completed_at": datetime.datetime.now(datetime.UTC).isoformat(),
            "elapsed_seconds": time.monotonic() - started,
        }

    with ExitStack() as stack:
        try:
            archive = stack.enter_context(open_archive(config, dry_run=True))
        except (ArchiveError, OSError) as error:
            document = {
                "version": 1,
                "archive": str(config.archive),
                "passed": False,
                "checks": [],
                "archive_error": str(error),
                **timing(),
            }
            if record:
                VerificationHistory(config_path_from(ctx), config.archive).record(
                    document
                )
            if as_json:
                click.echo(json.dumps(document, indent=2))
            if isinstance(error, ArchiveError):
                raise
            raise click.ClickException(str(error)) from error
        if report_path is not None:
            _validate_report_path(report_path, archive.paths.archive)
        with ExportProgress(
            item_label="files checked", show_downloads=False
        ) as progress:
            report = verify_archive(archive, progress=progress)

    document = {
        "version": 1,
        "archive": str(archive.paths.archive),
        "passed": report.passed,
        "archive_error": None,
        "checks": [asdict(check) for check in report.checks],
        **timing(),
    }
    if record:
        VerificationHistory(config_path_from(ctx), config.archive).record(document)
    if as_json:
        click.echo(json.dumps(document, indent=2, default=str))
    else:
        print_verification_report(report)
    if report_path is not None:
        try:
            with report_path.open("x", encoding="utf-8") as handle:
                handle.write(json.dumps(document, indent=2, default=str) + "\n")
        except OSError as error:
            raise click.ClickException(
                f"Could not write report '{report_path}': {error}"
            ) from error
        click.echo(f"Verification report: {report_path}", err=as_json)
    if not report.passed:
        if all(check.name == PENDING_CLEANUP for check in report.failed):
            raise ActionRequired(report.failed[0].detail)
        failed = ", ".join(check.name for check in report.failed)
        raise click.ClickException(f"Archive check(s) failed: {failed}")


def _validate_report_path(report_path: Path, archive_path: Path) -> None:
    """Reject unsafe report destinations before scanning any files."""
    if report_path.resolve().is_relative_to(archive_path.resolve()):
        raise click.UsageError("The verification report must be outside the archive")
    if report_path.exists() or report_path.is_symlink():
        raise click.UsageError(
            f"Report '{report_path}' already exists; choose a new file"
        )
    if not report_path.parent.is_dir():
        raise click.UsageError(
            f"Report parent '{report_path.parent}' must be an existing directory"
        )
