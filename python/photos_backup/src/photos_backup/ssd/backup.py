from __future__ import annotations

import shlex
import subprocess
import time
from pathlib import Path
from typing import TYPE_CHECKING

import click

from photos_backup.exclude import exclude_from_arg
from photos_backup.summary import BackupSummary, parse_rsync_stats

if TYPE_CHECKING:
    from photos_backup.config import SdCardConfig, SsdConfig


class Backup:
    def __init__(
        self,
        config: SsdConfig,
        delete_at_destination: bool,
        dry_run: bool,
        sd_card: SdCardConfig | None = None,
    ) -> None:
        self.delete_at_destination = delete_at_destination
        self.dry_run = dry_run
        self.source = config.source
        self.exclude_file = config.exclude_file
        self.destination = config.destination
        self.sd_card = sd_card

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
            {src_path} {self.destination}"""

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
        self.destination.mkdir(parents=True, exist_ok=True)

        summaries = [
            self._run_rsync(
                "SSD: All Photos", self.source, exclude_from_arg(self.exclude_file)
            )
        ]
        if self.sd_card is None:
            summaries.append(BackupSummary(step_name="SSD: SD Card", skipped=True))
        else:
            summaries.append(
                self._run_rsync(
                    "SSD: SD Card",
                    self.sd_card.destination,
                    exclude_from_arg(self.sd_card.exclude_file),
                )
            )
        return summaries
