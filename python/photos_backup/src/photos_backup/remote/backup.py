from __future__ import annotations

import re
import shutil
import time
from pathlib import Path
from typing import TYPE_CHECKING

import click

from photos_backup.copy_safety import check_copy_path
from photos_backup.process import stream_command
from photos_backup.summary import BackupSummary

if TYPE_CHECKING:
    from photos_backup.config import RcloneConfig


class Backup:
    def __init__(self, config: RcloneConfig, source: Path, dry_run: bool) -> None:
        self.remote = config.remote
        self.src_path = source
        self.dry_run = dry_run
        self._check_rclone_installed()

    def _check_rclone_installed(self) -> None:
        if not shutil.which("rclone"):
            raise click.UsageError(
                "rclone is not installed. Install it from https://rclone.org/install/"
            )

    def backup(self) -> BackupSummary:
        check_copy_path(self.src_path, workflow="Remote", source=True)
        cmd = [
            "rclone",
            "sync",
            str(self.src_path),
            self.remote,
            "--progress",
            "--stats-one-line",
            "--stats",
            "5s",
        ]
        if self.dry_run:
            cmd.append("--dry-run")

        start = time.monotonic()
        result = stream_command(cmd)
        elapsed = time.monotonic() - start

        if result.returncode != 0:
            return BackupSummary(
                step_name="Remote",
                elapsed_seconds=elapsed,
                error=f"rclone exited with code {result.returncode}",
            )

        stats = _parse_rclone_stats(result.stdout)
        return BackupSummary(
            step_name="Remote",
            files_transferred=stats["files_transferred"],
            total_size=stats["total_size"],
            elapsed_seconds=elapsed,
            dry_run=self.dry_run,
        )


def _parse_rclone_stats(output: str) -> dict[str, int | str]:
    result: dict[str, int | str] = {"files_transferred": 0, "total_size": ""}

    files_matches = re.findall(r"(?:Transferred:\s*|xfr#)(\d+)\s*/\s*\d+", output)
    if files_matches:
        result["files_transferred"] = int(files_matches[-1])

    size_matches = re.findall(r"Transferred:\s*([\d.]+ \S+)\s*/", output)
    if size_matches:
        result["total_size"] = size_matches[-1]

    return result
