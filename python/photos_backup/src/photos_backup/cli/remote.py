import click

from photos_backup.cli.context import config_path_from
from photos_backup.config import load_rclone_config, resolve_rclone_source
from photos_backup.remote.backup import Backup
from photos_backup.summary import print_summary


@click.command(name="remote", help="Sync backup to cloud via rclone.")
@click.option("--dry-run", is_flag=True, help="Preview without transferring files.")
@click.pass_context
def remote(ctx: click.Context, dry_run: bool) -> None:
    config_path = config_path_from(ctx)
    config = load_rclone_config(config_path)
    summary = Backup(
        config=config,
        source=resolve_rclone_source(config, config_path),
        dry_run=dry_run,
    ).backup()
    print_summary(summary)
