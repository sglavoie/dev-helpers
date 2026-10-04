from __future__ import annotations

import time
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.copy_safety import check_copy_path
from photos_backup.exclude import exclude_from_arg
from photos_backup.process import stream_command
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
        check_copy_path(src_path, workflow="SSD", source=True)
        check_copy_path(self.destination, workflow="SSD")

        cmd = ["rsync", "-avh", "--progress", "--stats"]
        if self.delete_at_destination:
            cmd.append("--delete")
        if self.dry_run:
            cmd.append("--dry-run")
        if exclude:
            cmd.append(exclude)
        cmd.extend(["--", str(src_path), str(self.destination)])

        start = time.monotonic()
        result = stream_command(cmd, check=True)
        elapsed = time.monotonic() - start

        stats = parse_rsync_stats(result.stdout)
        return BackupSummary(
            step_name=step_name,
            files_transferred=stats["files_transferred"],
            total_size=stats["total_size"],
            elapsed_seconds=elapsed,
            dry_run=self.dry_run,
        )

    def backup(self) -> list[BackupSummary]:
        # Validate every configured input before creating directories or copying.
        check_copy_path(self.source, workflow="SSD", source=True)
        if self.sd_card is not None:
            check_copy_path(self.sd_card.destination, workflow="SSD", source=True)
        check_copy_path(self.destination, workflow="SSD")
        if not self.dry_run:
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
