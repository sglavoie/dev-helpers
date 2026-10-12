from __future__ import annotations

import contextlib
import dataclasses
import datetime
import json
import os
import sqlite3
import tempfile
import unittest
from pathlib import Path


from photos_backup.apple_photos.adapter import PhotosProbes
from photos_backup.apple_photos.verify import (
    verify_archive,
)
from photos_backup.archive import (
    ArchivePaths,
    ArchiveState,
    ArchiveStateStore,
    SystemProbes,
    create_archive_tree,
    open_archive,
)
from tests.test_export import HOSTNAME, THURSDAY, make_config

MONDAY = datetime.datetime(2026, 8, 10, 6, 0, tzinfo=datetime.UTC)
OTHER_HOST = "Sebastiens Mac mini.local"
MALFORMED = "*** in database main ***\nPage 4 is never used"


def write_export_db(
    path: Path,
    files: list[Path],
    *,
    version: str = "11.0",
    relative_to: Path | None = None,
) -> None:
    """Write an osxphotos-shaped export database recording `files` as exported.

    `relative_to` stores the current osxphotos format, where every filepath is
    relative to the export root; without it the legacy absolute format is used.
    """
    path.parent.mkdir(parents=True, exist_ok=True)
    path.unlink(missing_ok=True)
    connection = sqlite3.connect(str(path))
    with contextlib.closing(connection):
        connection.execute(
            "CREATE TABLE version (id INTEGER PRIMARY KEY, "
            "osxphotos TEXT, exportdb TEXT)"
        )
        connection.execute(
            "INSERT INTO version(osxphotos, exportdb) VALUES (?, ?)",
            ("0.76.1", version),
        )
        connection.execute(
            "CREATE TABLE photoinfo (id INTEGER PRIMARY KEY, uuid TEXT, photoinfo JSON)"
        )
        connection.execute(
            "CREATE TABLE export_data (id INTEGER PRIMARY KEY, filepath TEXT, "
            "uuid TEXT, dest_size INTEGER, dest_mtime REAL)"
        )
        for index, exported in enumerate(files):
            status = exported.stat()
            uuid = f"uuid-{index}"
            connection.execute(
                "INSERT INTO photoinfo(uuid, photoinfo) VALUES (?, ?)",
                (
                    uuid,
                    json.dumps(
                        {"cloud_guid": f"guid-{index}", "original_filename": "a.jpg"}
                    ),
                ),
            )
            filepath = (
                os.path.relpath(exported, relative_to)
                if relative_to is not None
                else str(exported)
            )
            connection.execute(
                "INSERT INTO export_data(filepath, uuid, dest_size, dest_mtime) "
                "VALUES (?, ?, ?, ?)",
                (filepath, uuid, status.st_size, status.st_mtime),
            )
        connection.commit()


class VerifyTestCase(unittest.TestCase):
    """Gives every test a healthy archive that each test then breaks one way."""

    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        # Resolved because macOS temporary directories live under symlinks.
        self.root = Path(self._tmpdir.name).resolve()
        self.volume = self.root / "SanDisk"
        self.volume.mkdir()
        self.archive_root = self.volume / "Media" / "Apple Photos"
        self.config = make_config(self.volume, self.archive_root)
        volume = self.volume
        self.system_probes = SystemProbes(
            is_mount=lambda path: path == volume,
            hostname=lambda: HOSTNAME,
            now=lambda: THURSDAY,
        )
        self.paths = ArchivePaths(volume=self.volume, archive=self.archive_root)
        create_archive_tree(self.paths)

        self.report = self.paths.export_report(HOSTNAME, MONDAY.date())
        self.report.write_text("filename\n")
        self.exported = self.archive_root / "2026" / "08" / "a.jpg"
        self.exported.parent.mkdir(parents=True)
        self.exported.write_bytes(b"an exported photo")
        write_export_db(
            self.paths.export_db, [self.exported], relative_to=self.archive_root
        )
        self.save_state()

    def save_state(self, **changes) -> None:
        healthy = ArchiveState(
            initialized_at=MONDAY,
            last_successful_export_at=THURSDAY,
            last_full_export_at=MONDAY,
            writer_hostname=HOSTNAME,
            last_report_path=self.report,
        )
        ArchiveStateStore(self.paths).save(dataclasses.replace(healthy, **changes))

    def verify(self, **probe_overrides):
        probes = PhotosProbes(**probe_overrides) if probe_overrides else None
        with open_archive(
            self.config, dry_run=True, probes=self.system_probes
        ) as opened:
            return verify_archive(opened, probes=probes)

    def check(self, report, name):
        for check in report.checks:
            if check.name == name:
                return check
        raise AssertionError(f"no check named {name!r} in {report}")
