import json
from dataclasses import asdict
from pathlib import Path

import click

from photos_backup.apple_photos.verify import PENDING_CLEANUP, verify_archive
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from
from photos_backup.errors import ActionRequired
from photos_backup.summary import print_verification_report


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
@click.pass_context
def verify(ctx: click.Context, report_path: Path | None) -> None:
    config = apple_photos_config_from(ctx)
    with open_archive(config, dry_run=True) as archive:
        if report_path is not None:
            if report_path.resolve().is_relative_to(archive.paths.archive.resolve()):
                raise click.UsageError(
                    "The verification report must be outside the archive"
                )
            if report_path.exists() or report_path.is_symlink():
                raise click.UsageError(
                    f"Report '{report_path}' already exists; choose a new file"
                )
        report = verify_archive(archive)

    print_verification_report(report)
    if report_path is not None:
        document = {
            "version": 1,
            "archive": str(archive.paths.archive),
            "passed": report.passed,
            "checks": [asdict(check) for check in report.checks],
        }
        try:
            with report_path.open("x", encoding="utf-8") as handle:
                handle.write(json.dumps(document, indent=2, default=str) + "\n")
        except OSError as error:
            raise click.ClickException(
                f"Could not write report '{report_path}': {error}"
            ) from error
        click.echo(f"Verification report: {report_path}")
    if not report.passed:
        if all(check.name == PENDING_CLEANUP for check in report.failed):
            raise ActionRequired(report.failed[0].detail)
        failed = ", ".join(check.name for check in report.failed)
        raise click.ClickException(f"Archive check(s) failed: {failed}")
