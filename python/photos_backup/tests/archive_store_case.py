from __future__ import annotations

import datetime
import tempfile
import unittest
from pathlib import Path

from photos_backup.archive import (
    SystemProbes,
)
from photos_backup.config import ApplePhotosConfig

HOST = "Sebastiens MacBook.local"
FIXED_NOW = datetime.datetime(2026, 8, 11, 9, 30, tzinfo=datetime.UTC)


def make_config(
    volume: Path, archive: Path, *, require_mounted_volume: bool = True
) -> ApplePhotosConfig:
    return ApplePhotosConfig(
        volume=volume,
        archive=archive,
        require_mounted_volume=require_mounted_volume,
        library=Path("/Users/tester/Pictures/Photos Library.photoslibrary"),
        legacy_export=None,
        limit_export=0,
        spouse_device_models=(),
        incremental_overlap_days=14,
        full_export_weekday=0,
        full_export_max_age_days=7,
        mirror=True,
        cleanup_max_assets=10,
        cleanup_max_fraction=0.001,
    )


class ArchiveTestCase(unittest.TestCase):
    """Base class giving every test a fake mounted volume in a temporary tree."""

    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        # Resolved because macOS temporary directories live under symlinks.
        self.root = Path(self._tmpdir.name).resolve()
        self.volume = self.root / "SanDisk"
        self.volume.mkdir()
        self.archive = self.volume / "Media" / "Apple Photos"
        self.config = make_config(self.volume, self.archive)
        self.probes = self.make_probes()

    def make_probes(self, *, mounted: bool = True) -> SystemProbes:
        volume = self.volume
        return SystemProbes(
            is_mount=lambda path: mounted and path == volume,
            hostname=lambda: "Sebastiens MacBook.local",
            now=lambda: FIXED_NOW,
        )

    def assert_untouched(self) -> None:
        self.assertEqual(list(self.volume.iterdir()), [])
