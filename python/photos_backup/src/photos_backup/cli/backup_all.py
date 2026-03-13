from __future__ import annotations

import click

from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.config import Config
from photos_backup.remote.backup import Backup as RemoteBackup
from photos_backup.sd_card.backup import Backup as SdCardBackup
from photos_backup.ssd.backup import Backup as SsdBackup
from photos_backup.summary import BackupSummary, print_pipeline_summary


@click.command(name="backup-all", help="Run the full backup pipeline.")
@click.option("--dry-run", is_flag=True, help="Dry run for all steps.")
@click.option(
    "--delete",
    is_flag=True,
    help="Delete extra files on SSD destination.",
)
@click.option("--skip-apple-photos", is_flag=True, help="Skip Apple Photos export.")
@click.option("--skip-sd-card", is_flag=True, help="Skip SD card backup.")
@click.option("--skip-ssd", is_flag=True, help="Skip SSD backup.")
@click.option("--skip-remote", is_flag=True, help="Skip remote backup.")
def backup_all(
    dry_run: bool,
    delete: bool,
    skip_apple_photos: bool,
    skip_sd_card: bool,
    skip_ssd: bool,
    skip_remote: bool,
) -> None:
    config = Config.from_env()
    summaries: list[BackupSummary] = []

    # Apple Photos
    if skip_apple_photos:
        summaries.append(BackupSummary(step_name="Apple Photos", skipped=True))
    else:
        try:
            summary = ApplePhotosExport(config=config, testing=dry_run).export()
            summaries.append(summary)
        except Exception as e:
            summaries.append(BackupSummary(step_name="Apple Photos", error=str(e)))

    # SD Card
    if skip_sd_card:
        summaries.append(BackupSummary(step_name="SD Card", skipped=True))
    else:
        try:
            summary = SdCardBackup(config=config, dry_run=dry_run).backup()
            summaries.append(summary)
        except Exception as e:
            summaries.append(BackupSummary(step_name="SD Card", error=str(e)))

    # SSD
    if skip_ssd:
        summaries.append(BackupSummary(step_name="SSD", skipped=True))
    else:
        try:
            ssd_summaries = SsdBackup(
                config=config, delete_at_destination=delete, dry_run=dry_run
            ).backup()
            summaries.extend(ssd_summaries)
        except Exception as e:
            summaries.append(BackupSummary(step_name="SSD", error=str(e)))

    # Remote (skip gracefully if not configured)
    if skip_remote or not config.rclone_remote:
        summaries.append(BackupSummary(step_name="Remote", skipped=True))
    else:
        try:
            summary = RemoteBackup(config=config, dry_run=dry_run).backup()
            summaries.append(summary)
        except Exception as e:
            summaries.append(BackupSummary(step_name="Remote", error=str(e)))

    print_pipeline_summary(summaries)
