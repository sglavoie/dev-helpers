from __future__ import annotations

import datetime
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.archive import ArchivePaths, ArchiveState, ArchiveStateStore
from tests import isolate_transfer_history


class ArchiveCommandTestCase(unittest.TestCase):
    """Gives every test a fake mounted volume and a matching configuration file."""

    def setUp(self) -> None:
        self.history_root = isolate_transfer_history(self)
        self.enterContext(
            mock.patch(
                "photos_backup.cli.backup_all.which", return_value="/test/bin/tool"
            )
        )
        self.runner = CliRunner()
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        # Resolved because macOS temporary directories live under symlinks.
        self.root = Path(self._tmpdir.name).resolve()
        self.volume = self.root / "SanDisk"
        self.volume.mkdir()
        self.config_path = self.root / "photos-backup.toml"
        self.config_path.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\n'
            f'archive = "{self.volume}/Media/Apple Photos"\n'
            'library = "/Users/tester/Pictures/Photos Library.photoslibrary"\n'
        )

    def mounted(self):
        return mock.patch(
            "photos_backup.archive.probes.os.path.ismount",
            lambda path: Path(path) == self.volume,
        )

    def initialized_archive(self, **changes) -> ArchivePaths:
        paths = ArchivePaths(
            volume=self.volume, archive=self.volume / "Media" / "Apple Photos"
        )
        paths.metadata.mkdir(parents=True)
        ArchiveStateStore(paths).save(
            ArchiveState(
                initialized_at=datetime.datetime(
                    2026, 8, 11, 9, 30, tzinfo=datetime.UTC
                ),
                **changes,
            )
        )
        return paths
