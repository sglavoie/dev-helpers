from __future__ import annotations

import time
from typing import TYPE_CHECKING

from photos_backup.exclude import exclude_from_arg
from photos_backup.copy_safety import check_copy_path
from photos_backup.process import stream_command
from photos_backup.summary import BackupSummary, parse_rsync_stats

if TYPE_CHECKING:
    from photos_backup.config import SdCardConfig


class Backup:
    def __init__(self, config: SdCardConfig, dry_run: bool) -> None:
        self.dry_run = dry_run
        self.src_path = config.source
        self.dst_path = config.destination
        self.exclude_file = config.exclude_file

    def backup(self) -> BackupSummary:
        check_copy_path(self.src_path, workflow="SD Card", source=True)
        check_copy_path(self.dst_path, workflow="SD Card")
        if not self.dry_run:
            self.dst_path.mkdir(parents=True, exist_ok=True)
        exclude = exclude_from_arg(self.exclude_file)
        cmd = ["rsync", "-a", "--progress", "--stats"]
        if self.dry_run:
            cmd.append("--dry-run")
        if exclude:
            cmd.append(exclude)
        cmd.extend(["--", str(self.src_path), str(self.dst_path)])

        start = time.monotonic()
        result = stream_command(cmd, check=True)
        elapsed = time.monotonic() - start

        stats = parse_rsync_stats(result.stdout)
        return BackupSummary(
            step_name="SD Card",
            files_transferred=stats["files_transferred"],
            total_size=stats["total_size"],
            elapsed_seconds=elapsed,
            dry_run=self.dry_run,
        )
