import rich_click as click

from photos_backup.cli.context import config_path_from
from photos_backup.cli.outcome import raise_for_summaries
from photos_backup.config import load_rclone_config, resolve_rclone_source
from photos_backup.remote.backup import Backup
from photos_backup.summary import print_summary
from photos_backup.transfers import TransferHistory


@click.command(name="remote", help="Copy backup to cloud via rclone.")
@click.option("--dry-run", is_flag=True, help="Preview without transferring files.")
@click.option(
    "--delete", is_flag=True, help="Delete remote files absent from the source."
)
@click.pass_context
def remote(ctx: click.Context, dry_run: bool, delete: bool) -> None:
    config_path = config_path_from(ctx)
    config = load_rclone_config(config_path)
    summary = Backup(
        config=config,
        source=resolve_rclone_source(config, config_path),
        dry_run=dry_run,
        delete_at_destination=delete,
        history=TransferHistory(config_path),
    ).backup()
    print_summary(summary)
    raise_for_summaries([summary])
