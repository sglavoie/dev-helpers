from __future__ import annotations

import time
from contextlib import ExitStack
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.copy_safety import check_copy_paths
from photos_backup.archive.lock import copy_source_lock
from photos_backup.exclude import exclude_from_arg
from photos_backup.errors import ActionRequired
from photos_backup.process import stream_command, transfer_errors
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
        check_copy_paths((src_path,), self.destination, workflow="SSD")

        cmd = ["rsync", "-avh", "--progress", "--stats"]
        if self.delete_at_destination:
            cmd.append("--delete")
        if self.dry_run:
            cmd.append("--dry-run")
        if exclude:
            cmd.append(exclude)
        cmd.extend(["--", str(src_path), str(self.destination)])

        start = time.monotonic()
        with transfer_errors(step_name, "rsync"):
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
        sources = (self.source,)
        if self.sd_card is not None:
            sources += (self.sd_card.destination,)
        check_copy_paths(sources, self.destination, workflow="SSD")
        with ExitStack() as locks:
            for source in sources:
                locks.enter_context(copy_source_lock(source))
            return self._copy_sources()

    def _copy_sources(self) -> list[BackupSummary]:
        if not self.dry_run:
            self.destination.mkdir(parents=True, exist_ok=True)

        copies = [("SSD: All Photos", self.source, self.exclude_file)]
        if self.sd_card is not None:
            copies.append(
                ("SSD: SD Card", self.sd_card.destination, self.sd_card.exclude_file)
            )
        summaries: list[BackupSummary] = []
        for step_name, source, exclude_file in copies:
            if any(summary.error for summary in summaries):
                summaries.append(
                    BackupSummary(
                        step_name=step_name,
                        skipped=True,
                        skip_reason="Previous SSD copy did not complete",
                    )
                )
                continue
            started = time.monotonic()
            try:
                summaries.append(
                    self._run_rsync(step_name, source, exclude_from_arg(exclude_file))
                )
            except Exception as error:
                summaries.append(
                    BackupSummary(
                        step_name=step_name,
                        error=str(error),
                        action_required=isinstance(error, ActionRequired),
                        elapsed_seconds=time.monotonic() - started,
                        dry_run=self.dry_run,
                    )
                )
        if self.sd_card is None:
            summaries.append(
                BackupSummary(
                    step_name="SSD: SD Card",
                    skipped=True,
                    skip_reason="Not configured: [sd_card]",
                )
            )
        return summaries
