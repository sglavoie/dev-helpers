from __future__ import annotations

import datetime
import time
from contextlib import ExitStack
from functools import partial
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.copy_safety import (
    check_copy_path,
    check_copy_paths,
    check_mirror_source,
)
from photos_backup.archive.lock import copy_source_lock
from photos_backup.cli.context import suggested_command
from photos_backup.exclude import exclude_from_arg
from photos_backup.errors import ActionRequired
from photos_backup.process import interactive_transfers, stream_command, transfer_errors
from photos_backup.summary import BackupSummary, parse_rsync_stats
from photos_backup.space import print_destination_space

if TYPE_CHECKING:
    from photos_backup.config import SdCardConfig, SsdConfig
    from photos_backup.transfers import TransferHistory


class Backup:
    def __init__(
        self,
        config: SsdConfig,
        delete_at_destination: bool,
        dry_run: bool,
        sd_card: SdCardConfig | None = None,
        *,
        history: TransferHistory | None = None,
    ) -> None:
        self.delete_at_destination = delete_at_destination
        self.dry_run = dry_run
        self.source = config.source
        self.exclude_file = config.exclude_file
        self.destination = config.destination
        self.sd_card = sd_card
        self.history = history

    def _run_rsync(
        self, step_name: str, src_path: Path, exclude: str = ""
    ) -> BackupSummary:
        check_copy_paths((src_path,), self.destination, workflow="SSD")
        if self.delete_at_destination:
            check_mirror_source(src_path, workflow="SSD")

        cmd = ["rsync", "-ah", "--stats"]
        if interactive_transfers():
            cmd.extend(["--verbose", "--progress"])
        if self.delete_at_destination:
            cmd.append("--delete")
        if self.dry_run:
            cmd.extend(["--dry-run", "--itemize-changes"])
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
        copies = [("SSD: All Photos", self.source, self.exclude_file)]
        if self.sd_card is not None:
            copies.append(
                ("SSD: SD Card", self.sd_card.destination, self.sd_card.exclude_file)
            )
        sd_card_skip: BackupSummary | None = None
        started_at = datetime.datetime.now(datetime.UTC).isoformat()
        with ExitStack() as locks:
            try:
                if self.sd_card is not None and self._sd_card_copy_missing(
                    self.sd_card
                ):
                    copies = copies[:1]
                    sd_card_skip = BackupSummary(
                        step_name="SSD: SD Card",
                        skipped=True,
                        skip_reason=(
                            f"'{self.sd_card.destination}' not created yet; run "
                            f"`{suggested_command('sd-card')}` before copying it "
                            "to the SSD"
                        ),
                    )
                # Validate every input and exclusion before creating destinations
                # or copying, and keep all archive locks through the transfers.
                sources = tuple(source for _, source, _ in copies)
                check_copy_paths(sources, self.destination, workflow="SSD")
                for source in sources:
                    locks.enter_context(copy_source_lock(source))
                prepared = [
                    (
                        step_name,
                        source,
                        exclude_from_arg(
                            exclude_file,
                            delete_at_destination=self.delete_at_destination,
                        ),
                    )
                    for step_name, source, exclude_file in copies
                ]
                if self.delete_at_destination:
                    for source in sources:
                        check_mirror_source(source, workflow="SSD")
                print_destination_space(self.destination)
                if not self.dry_run:
                    self.destination.mkdir(parents=True, exist_ok=True)
            except BaseException as error:
                if self.history is not None:
                    for step_name, source, _ in copies:
                        self.history.record_preflight_failure(
                            step_name,
                            source,
                            self.destination / source.name,
                            error,
                            started_at=started_at,
                            dry_run=self.dry_run,
                            delete_at_destination=self.delete_at_destination,
                        )
                raise
            summaries = self._copy_sources(prepared)
        if sd_card_skip is not None:
            summaries.append(sd_card_skip)
        return summaries

    @staticmethod
    def _sd_card_copy_missing(sd_card: SdCardConfig) -> bool:
        """The SD card copy only exists after `sd-card` first runs.

        An unmounted volume still requires action, so check it before treating
        the directory as not created yet.
        """
        if sd_card.destination.exists():
            return False
        check_copy_path(sd_card.destination, workflow="SSD")
        return True

    def _copy_sources(
        self, prepared: list[tuple[str, Path, str]]
    ) -> list[BackupSummary]:
        summaries: list[BackupSummary] = []
        for step_name, source, exclude in prepared:
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
                operation = partial(self._run_rsync, step_name, source, exclude)
                summaries.append(
                    self.history.run(
                        step_name,
                        source,
                        self.destination / source.name,
                        operation,
                        dry_run=self.dry_run,
                        delete_at_destination=self.delete_at_destination,
                    )
                    if self.history is not None
                    else operation()
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
