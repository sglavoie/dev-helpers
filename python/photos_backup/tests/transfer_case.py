"""Shared fixtures for the copy workflow test modules."""

from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from tests import isolate_transfer_history

RSYNC_OK = subprocess.CompletedProcess(
    [], 0, stdout="Number of regular files transferred: 1\n"
)
RCLONE_OK = subprocess.CompletedProcess([], 0, stdout="Transferred: 1 / 1\n")


class TransferTestCase(unittest.TestCase):
    def setUp(self) -> None:
        isolate_transfer_history(self)
        self.root = Path(self.enterContext(tempfile.TemporaryDirectory())).resolve()
        self.source = self.root / "photos"
        self.source.mkdir()
        self.destination = self.root / "ssd"
        self.config = self.root / "config.toml"
        self.enterContext(
            mock.patch("photos_backup.cli.backup_all.which", return_value="tool")
        )
        self.enterContext(
            mock.patch(
                "photos_backup.remote.backup.shutil.which", return_value="rclone"
            )
        )

    def add_photo(self, directory: Path) -> None:
        directory.mkdir(parents=True, exist_ok=True)
        (directory / "DSC00001.ARW").write_text("photo")

    def add_finder_clutter(self, directory: Path) -> None:
        (directory / ".DS_Store").write_text("finder")
        (directory / "._DSC00001.ARW").write_text("resource fork")

    def invoke(self, *args: str):
        return CliRunner().invoke(cli, ["--config", str(self.config), *args])
