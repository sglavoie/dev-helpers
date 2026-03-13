from __future__ import annotations

import shlex
import subprocess
import time
from typing import TYPE_CHECKING

from photos_backup.summary import BackupSummary, parse_rsync_stats

if TYPE_CHECKING:
    from photos_backup.config import Config


class Backup:
    def __init__(self, config: Config, dry_run: bool) -> None:
        self.dry_run = dry_run
        self.src_path = config.sd_card_src_path
        self.dst_path = config.sd_card_dst_path
        self.exclude_file = config.sd_card_exclude_file

    def backup(self) -> BackupSummary:
        dry_run = "--dry-run" if self.dry_run else ""
        exclude = "" if not self.exclude_file else f"--exclude-from={self.exclude_file}"
        cmd = f"""rsync -a --progress --stats \
            {dry_run} \
            {exclude} \
            {self.src_path} {self.dst_path}"""

        start = time.monotonic()
        result = subprocess.run(
            shlex.split(cmd), check=True, capture_output=True, text=True
        )
        elapsed = time.monotonic() - start

        if result.stdout:
            print(result.stdout)

        stats = parse_rsync_stats(result.stdout)
        return BackupSummary(
            step_name="SD Card",
            files_transferred=stats["files_transferred"],
            total_size=stats["total_size"],
            elapsed_seconds=elapsed,
        )
