from __future__ import annotations

import time
from typing import TYPE_CHECKING

from photos_backup.exclude import exclude_from_arg
from photos_backup.copy_safety import check_copy_paths
from photos_backup.process import interactive_transfers, stream_command, transfer_errors
from photos_backup.summary import BackupSummary, parse_rsync_stats
from photos_backup.space import print_destination_space

if TYPE_CHECKING:
    from photos_backup.config import SdCardConfig
    from photos_backup.transfers import TransferHistory


class Backup:
    def __init__(
        self,
        config: SdCardConfig,
        dry_run: bool,
        *,
        history: TransferHistory | None = None,
    ) -> None:
        self.history = history
        self.dry_run = dry_run
        self.src_path = config.source
        self.dst_path = config.destination
        self.exclude_file = config.exclude_file

    def backup(self) -> BackupSummary:
        if self.history is not None:
            return self.history.run(
                "SD Card",
                self.src_path,
                self.dst_path / self.src_path.name,
                self._copy,
                dry_run=self.dry_run,
                delete_at_destination=False,
            )
        return self._copy()

    def _copy(self) -> BackupSummary:
        check_copy_paths((self.src_path,), self.dst_path, workflow="SD Card")
        print_destination_space(self.dst_path)
        if not self.dry_run:
            self.dst_path.mkdir(parents=True, exist_ok=True)
        exclude = exclude_from_arg(self.exclude_file)
        cmd = ["rsync", "-a", "--stats"]
        if interactive_transfers():
            cmd.append("--progress")
        if self.dry_run:
            cmd.extend(["--dry-run", "--itemize-changes"])
        if exclude:
            cmd.append(exclude)
        cmd.extend(["--", str(self.src_path), str(self.dst_path)])

        start = time.monotonic()
        with transfer_errors("SD Card", "rsync"):
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
