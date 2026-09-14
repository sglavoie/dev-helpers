import sys

import click

from photos_backup.apple_photos.local_export import (
    confirmation_matches,
    delete_local_export,
    plan_local_export_cleanup,
)
from photos_backup.apple_photos.verify import verify_archive
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from
from photos_backup.errors import ActionRequired
from photos_backup.summary import (
    print_local_export_cleanup,
    print_local_export_plan,
    print_verification_report,
)

NOT_A_TERMINAL = (
    "This command deletes photographs, so it only runs with a person watching: "
    "run it from a terminal, never from a script, a scheduled job, or a test"
)
CONFIRMATION = (
    "Type the directory in full to delete its contents, or anything else to cancel"
)
CANCELLED = "Cancelled; nothing was deleted."


@click.command(
    name="cleanup-local-export",
    help="Delete the legacy local export once the shared archive proves it is "
    "redundant.",
)
@click.option(
    "--dry-run",
    is_flag=True,
    help="Report what would be deleted without prompting or deleting.",
)
@click.pass_context
def cleanup_local_export(ctx: click.Context, dry_run: bool) -> None:
    config = apple_photos_config_from(ctx)
    if not dry_run and not sys.stdin.isatty():
        raise ActionRequired(NOT_A_TERMINAL)

    # Opened as a dry run throughout: this command only ever reads the archive.
    with open_archive(config, dry_run=True) as archive:
        if not archive.state_store.load().initialized:
            raise ActionRequired(
                f"Archive '{config.archive}' is not initialized; run "
                "'photos-backup bootstrap' before deleting the local export"
            )
        report = verify_archive(archive)

    print_verification_report(report)
    if not report.passed:
        failed = ", ".join(check.name for check in report.failed)
        raise ActionRequired(
            f"Archive check(s) failed: {failed}; the local export stays until the "
            "archive that replaces it is healthy"
        )

    plan = plan_local_export_cleanup(config)
    print_local_export_plan(plan, dry_run=dry_run)
    if plan.refusal is not None:
        raise ActionRequired(f"Nothing was deleted because {plan.refusal}")
    if dry_run:
        return

    typed = click.prompt(CONFIRMATION, default="", show_default=False)
    if not confirmation_matches(typed, plan.target):
        click.echo(CANCELLED)
        return

    print_local_export_cleanup(delete_local_export(plan))
