from __future__ import annotations

import csv
import dataclasses
import datetime
import tempfile
import unittest
from pathlib import Path
from typing import Any

from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.archive import ArchiveState, SystemProbes, open_archive
from photos_backup.config import ApplePhotosConfig
from tests.timezones import pin_timezone

HOSTNAME = "Sebastiens MacBook.local"
SAFE_HOSTNAME = "Sebastiens-MacBook-local"
# 2026-08-13 is a Thursday; the most recent Monday is 2026-08-10.
THURSDAY = datetime.datetime(2026, 8, 13, 9, 30, tzinfo=datetime.UTC)
LAST_MONDAY = datetime.datetime(2026, 8, 10, 0, 0, tzinfo=datetime.UTC)

CSV_COLUMNS = ("filename", "new", "updated", "skipped", "missing", "error")


def make_config(volume: Path, archive: Path, **overrides: Any) -> ApplePhotosConfig:
    config = ApplePhotosConfig(
        volume=volume,
        archive=archive,
        library=Path("/Users/tester/Pictures/Photos Library.photoslibrary"),
        legacy_export=None,
        limit_export=25,
        spouse_device_models=("iPhone SE (2nd generation)",),
        incremental_overlap_days=14,
        full_export_weekday=0,
        full_export_max_age_days=7,
        mirror=True,
        cleanup_max_assets=10,
        cleanup_max_fraction=0.001,
    )
    return dataclasses.replace(config, **overrides)


class FakeRunner:
    """Stands in for osxphotos, recording arguments and writing a report."""

    def __init__(self, rows: list[dict[str, str]] | None = None, exit_code: int = 0):
        self.rows = rows if rows is not None else []
        self.exit_code = exit_code
        self.arguments: dict[str, Any] | None = None

    def __call__(self, arguments: dict[str, Any]) -> int:
        self.arguments = dict(arguments)
        report = Path(arguments["report"])
        report.parent.mkdir(parents=True, exist_ok=True)
        with report.open("w", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=CSV_COLUMNS)
            writer.writeheader()
            writer.writerows(self.rows)
        return self.exit_code


class ReportlessRunner:
    """Stands in for osxphotos exiting 0 without writing a usable report."""

    def __init__(self, contents: str | None = None, exit_code: int = 0):
        self.contents = contents
        self.exit_code = exit_code
        self.arguments: dict[str, Any] | None = None

    def __call__(self, arguments: dict[str, Any]) -> int:
        self.arguments = dict(arguments)
        if self.contents is not None:
            report = Path(arguments["report"])
            report.parent.mkdir(parents=True, exist_ok=True)
            report.write_text(self.contents)
        return self.exit_code


def row(filename: str, **flags: int) -> dict[str, str]:
    values = dict.fromkeys(CSV_COLUMNS, "0")
    values["filename"] = filename
    for name, flag in flags.items():
        values[name] = str(flag)
    return values


class ExportTestCase(unittest.TestCase):
    def setUp(self) -> None:
        pin_timezone(self, "UTC")
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        # Resolved because macOS temporary directories live under symlinks.
        self.root = Path(self._tmpdir.name).resolve()
        self.volume = self.root / "SanDisk"
        self.volume.mkdir()
        self.archive = self.volume / "Media" / "Apple Photos"
        self.config = make_config(self.volume, self.archive)
        volume = self.volume
        self.probes = SystemProbes(
            is_mount=lambda path: path == volume,
            hostname=lambda: HOSTNAME,
            now=lambda: THURSDAY,
        )

    def run_export(
        self,
        runner: FakeRunner,
        *,
        state: ArchiveState | None = None,
        dry_run: bool = False,
        **kwargs: Any,
    ):
        with open_archive(self.config, dry_run=dry_run, probes=self.probes) as archive:
            if state is not None:
                archive.state_store.save(state)
            export = ApplePhotosExport(
                config=self.config,
                archive=archive,
                runner=runner,
                metadata_reader=lambda path: {},
                **kwargs,
            )
            result = export.export()
            self.state = archive.state_store.load()
            self.paths = archive.paths
        return result
