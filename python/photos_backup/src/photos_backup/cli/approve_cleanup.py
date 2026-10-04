import click

from photos_backup.apple_photos.cleanup import approve_cleanup as run_cleanup_approval
from photos_backup.apple_photos.cleanup import discard_cleanup as run_cleanup_discard
from photos_backup.apple_photos.cleanup import preview_cleanup
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from
from photos_backup.summary import (
    print_cleanup_approval,
    print_cleanup_discard,
    print_cleanup_preview,
)


@click.command(
    name="approve-cleanup",
    help="Delete the archive files a pending cleanup run listed, after "
    "revalidating every one of them, or discard the run instead.",
)
@click.argument("run_id", metavar="RUN_ID")
@click.option(
    "--dry-run",
    is_flag=True,
    help="Revalidate and list proposed deletions without writing.",
)
@click.option(
    "--discard",
    is_flag=True,
    help="Reject the run instead of applying it; nothing is deleted.",
)
@click.pass_context
def approve_cleanup(
    ctx: click.Context, run_id: str, discard: bool, dry_run: bool
) -> None:
    if discard and dry_run:
        raise click.UsageError("--discard and --dry-run cannot be combined")
    config = apple_photos_config_from(ctx)
    with open_archive(config, dry_run=dry_run) as archive:
        if dry_run:
            print_cleanup_preview(preview_cleanup(config, archive, run_id))
            return
        if discard:
            print_cleanup_discard(run_cleanup_discard(archive, run_id))
            return
        approval = run_cleanup_approval(config, archive, run_id)

    print_cleanup_approval(approval)
