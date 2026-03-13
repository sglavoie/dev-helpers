from __future__ import annotations

import re
import shutil
import subprocess
import time

import click

from photos_backup.config import Config
from photos_backup.summary import BackupSummary


class Backup:
    def __init__(self, config: Config, dry_run: bool) -> None:
        self.remote = config.rclone_remote
        self.src_path = config.rclone_src_path or config.ssd_dst_path
        self.dry_run = dry_run
        self._check_rclone_installed()

    def _check_rclone_installed(self) -> None:
        if not shutil.which("rclone"):
            raise click.UsageError(
                "rclone is not installed. Install it from https://rclone.org/install/"
            )

    def backup(self) -> BackupSummary:
        if not self.remote:
            raise click.UsageError("RCLONE_REMOTE is not set in ~/.osxphotos.env")

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
        result = subprocess.run(cmd, capture_output=True, text=True, check=False)
        elapsed = time.monotonic() - start

        # Print rclone output
        if result.stdout:
            click.echo(result.stdout)
        if result.stderr:
            click.echo(result.stderr)

        if result.returncode != 0:
            return BackupSummary(
                step_name="Remote",
                elapsed_seconds=elapsed,
                error=f"rclone exited with code {result.returncode}",
            )

        stats = _parse_rclone_stats(result.stderr or result.stdout)
        return BackupSummary(
            step_name="Remote",
            files_transferred=stats["files_transferred"],
            total_size=stats["total_size"],
            elapsed_seconds=elapsed,
        )


def _parse_rclone_stats(output: str) -> dict[str, int | str]:
    result: dict[str, int | str] = {"files_transferred": 0, "total_size": ""}

    files_match = re.search(r"Transferred:\s*(\d+)\s*/\s*\d+", output)
    if files_match:
        result["files_transferred"] = int(files_match.group(1))

    size_match = re.search(r"Transferred:\s*([\d.]+ \S+)\s*/", output)
    if size_match:
        result["total_size"] = size_match.group(1)

    return result
