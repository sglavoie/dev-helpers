from __future__ import annotations

import shlex
import subprocess
import time
from pathlib import Path
from typing import TYPE_CHECKING

import click

from photos_backup.summary import BackupSummary, parse_rsync_stats

if TYPE_CHECKING:
    from photos_backup.config import Config


class Backup:
    def __init__(
        self, config: Config, delete_at_destination: bool, dry_run: bool
    ) -> None:
        self.delete_at_destination = delete_at_destination
        self.dry_run = dry_run
        self.all_photos_path = config.all_photos_path
        self.all_photos_exclude_file = config.all_photos_exclude_file
        self.apple_photos_path = config.apple_photos_dst_path
        self.sd_card_path = config.sd_card_dst_path
        self.sd_card_exclude_file = config.sd_card_exclude_file
        self.ssd_dst_path = config.ssd_dst_path

    def _run_rsync(
        self, step_name: str, src_path: Path, exclude: str = ""
    ) -> BackupSummary:
        if not src_path.exists():
            click.echo(f"'{src_path}' does not exist: skipping")
            return BackupSummary(step_name=step_name, skipped=True)

        dry_run = "--dry-run" if self.dry_run else ""
        delete = "--delete" if self.delete_at_destination else ""
        cmd = f"""rsync -avh --progress --stats {delete} \
            {dry_run} \
            {exclude} \
            {src_path} {self.ssd_dst_path}"""

        start = time.monotonic()
        result = subprocess.run(
            shlex.split(cmd), check=True, capture_output=True, text=True
        )
        elapsed = time.monotonic() - start

        if result.stdout:
            print(result.stdout)

        stats = parse_rsync_stats(result.stdout)
        return BackupSummary(
            step_name=step_name,
            files_transferred=stats["files_transferred"],
            total_size=stats["total_size"],
            elapsed_seconds=elapsed,
        )

    def backup(self) -> list[BackupSummary]:
        exclude_all_photos = (
            ""
            if not self.all_photos_exclude_file
            else f"--exclude-from={self.all_photos_exclude_file}"
        )
        exclude_sd_card = (
            ""
            if not self.sd_card_exclude_file
            else f"--exclude-from={self.sd_card_exclude_file}"
        )

        summaries = [
            self._run_rsync(
                "SSD: All Photos", self.all_photos_path, exclude_all_photos
            ),
            self._run_rsync("SSD: Apple Photos", self.apple_photos_path),
            self._run_rsync("SSD: SD Card", self.sd_card_path, exclude_sd_card),
        ]
        return summaries
