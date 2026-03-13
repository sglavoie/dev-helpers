import click

from photos_backup.config import Config
from photos_backup.remote.backup import Backup
from photos_backup.summary import print_summary


@click.command(name="remote", help="Sync backup to cloud via rclone.")
@click.option("--dry-run", is_flag=True, help="Preview without transferring files.")
def remote(dry_run: bool) -> None:
    config = Config.from_env()
    summary = Backup(config=config, dry_run=dry_run).backup()
    print_summary(summary)
