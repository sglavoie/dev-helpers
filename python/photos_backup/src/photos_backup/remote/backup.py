from __future__ import annotations

import re
import shutil
import time
from pathlib import Path
from typing import TYPE_CHECKING

import click

from photos_backup.archive.lock import copy_source_lock
from photos_backup.copy_safety import check_copy_path
from photos_backup.process import (
    interactive_transfers,
    stream_command,
    transfer_errors,
    transfer_failure,
)
from photos_backup.summary import BackupSummary

if TYPE_CHECKING:
    from photos_backup.config import RcloneConfig
    from photos_backup.transfers import TransferHistory

# Separate argv items: rclone receives the patterns verbatim, without a shell.
FINDER_CLUTTER_EXCLUDES = ("--exclude", ".DS_Store", "--exclude", "._*")


class Backup:
    def __init__(
        self,
        config: RcloneConfig,
        source: Path,
        dry_run: bool,
        *,
        delete_at_destination: bool = False,
        history: TransferHistory | None = None,
    ) -> None:
        self.remote = config.remote
        self.src_path = source
        self.dry_run = dry_run
        self.delete_at_destination = delete_at_destination
        self.history = history
        self._check_rclone_installed()

    def _check_rclone_installed(self) -> None:
        if not shutil.which("rclone"):
            raise click.ClickException(
                "Remote: rclone was not found on PATH. "
                "Install it from https://rclone.org/install/"
            )

    def backup(self) -> BackupSummary:
        if self.history is not None:
            return self.history.run(
                "Remote",
                self.src_path,
                self.remote,
                self._copy,
                dry_run=self.dry_run,
                delete_at_destination=self.delete_at_destination,
            )
        return self._copy()

    def _copy(self) -> BackupSummary:
        check_copy_path(self.src_path, workflow="Remote", source=True)
        cmd = [
            "rclone",
            "sync" if self.delete_at_destination else "copy",
            str(self.src_path),
            self.remote,
            "--stats-one-line",
            *FINDER_CLUTTER_EXCLUDES,
        ]
        if interactive_transfers():
            cmd.extend(["--progress", "--stats", "5s"])
        else:
            cmd.extend(["--stats", "1m", "--stats-log-level", "NOTICE"])
        if self.dry_run:
            cmd.append("--dry-run")

        start = time.monotonic()
        with (
            copy_source_lock(self.src_path, workflow="Remote"),
            transfer_errors("Remote", "rclone"),
        ):
            result = stream_command(cmd)
        elapsed = time.monotonic() - start

        if result.returncode != 0:
            return BackupSummary(
                step_name="Remote",
                elapsed_seconds=elapsed,
                error=transfer_failure("rclone", result.returncode, result.stdout),
                dry_run=self.dry_run,
            )

        stats = _parse_rclone_stats(result.stdout)
        return BackupSummary(
            step_name="Remote",
            files_transferred=stats["files_transferred"],
            total_size=stats["total_size"],
            elapsed_seconds=elapsed,
            dry_run=self.dry_run,
        )


def _parse_rclone_stats(output: str) -> dict[str, int | str | None]:
    result: dict[str, int | str | None] = {"files_transferred": None, "total_size": ""}

    files_matches = re.findall(r"(?:Transferred:\s*|xfr#)(\d+)\s*/\s*\d+", output)
    if files_matches:
        result["files_transferred"] = int(files_matches[-1])

    size_matches = re.findall(r"(?:Transferred:|NOTICE:)\s*([\d.]+ \S+)\s*/", output)
    if size_matches:
        result["total_size"] = size_matches[-1]

    return result
