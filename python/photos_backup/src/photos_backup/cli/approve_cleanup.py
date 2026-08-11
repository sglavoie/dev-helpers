import click

from photos_backup.apple_photos.cleanup import approve_cleanup as run_cleanup_approval
from photos_backup.apple_photos.cleanup import discard_cleanup as run_cleanup_discard
from photos_backup.archive import open_archive
from photos_backup.cli.context import config_path_from
from photos_backup.config import load_apple_photos_config
from photos_backup.summary import print_cleanup_approval, print_cleanup_discard


@click.command(
    name="approve-cleanup",
    help="Delete the archive files a pending cleanup run listed, after "
    "revalidating every one of them, or discard the run instead.",
)
@click.argument("run_id", metavar="RUN_ID")
@click.option(
    "--discard",
    is_flag=True,
    help="Reject the run instead of applying it; nothing is deleted.",
)
@click.pass_context
def approve_cleanup(ctx: click.Context, run_id: str, discard: bool) -> None:
    config = load_apple_photos_config(config_path_from(ctx))
    with open_archive(config) as archive:
        if discard:
            print_cleanup_discard(run_cleanup_discard(archive, run_id))
            return
        approval = run_cleanup_approval(config, archive, run_id)

    print_cleanup_approval(approval)
