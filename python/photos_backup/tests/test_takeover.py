from __future__ import annotations

import contextlib
import datetime
import json
import sqlite3
import tempfile
import unittest
from pathlib import Path

from photos_backup.apple_photos.adapter import PhotosProbes
from photos_backup.apple_photos.identity import AssetIdentity
from photos_backup.apple_photos.takeover import copy_database
from photos_backup.archive import (
    ArchivePaths,
    ArchiveState,
    ArchiveStateStore,
    SystemProbes,
    open_archive,
)
from photos_backup.config import ApplePhotosConfig

FIXED_NOW = datetime.datetime(2026, 8, 11, 9, 30, tzinfo=datetime.UTC)
OLD_HOST = "Sebastiens MacBook.local"
NEW_HOST = "Sebastiens Mac mini.local"
LIBRARY = Path("/Users/tester/Pictures/Photos Library.photoslibrary")


def asset(uuid: str, cloud_guid: str | None, name: str = "") -> AssetIdentity:
    return AssetIdentity(
        uuid=uuid,
        cloud_guid=cloud_guid,
        original_filename=name or f"{cloud_guid or uuid}.HEIC",
    )


def write_export_db(
    path: Path,
    assets: tuple[AssetIdentity, ...],
    *,
    version: str = "11.0",
    table: str = "photoinfo",
    column: str = "photoinfo",
) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
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
            f"CREATE TABLE {table} (id INTEGER PRIMARY KEY, uuid TEXT, {column} JSON)"
        )
        for item in assets:
            connection.execute(
                f"INSERT INTO {table}(uuid, {column}) VALUES (?, ?)",
                (
                    item.uuid,
                    json.dumps(
                        {
                            "uuid": item.uuid,
                            "cloud_guid": item.cloud_guid,
                            "original_filename": item.original_filename,
                        }
                    ),
                ),
            )
        connection.commit()


def read_uuids(path: Path) -> set[str]:
    connection = sqlite3.connect(str(path))
    with contextlib.closing(connection):
        return {row[0] for row in connection.execute("SELECT uuid FROM photoinfo")}


class FakeMigrator:
    """Rewrites export-database UUIDs the way osxphotos would, or fails on demand."""

    def __init__(
        self,
        library: tuple[AssetIdentity, ...] = (),
        *,
        error: Exception | None = None,
        rewrite: bool = True,
    ) -> None:
        self.library = library
        self.error = error
        self.rewrite = rewrite
        self.calls: list[tuple[Path, Path, bool]] = []

    def __call__(
        self, export_db: Path, library: Path, *, dry_run: bool
    ) -> tuple[int, int]:
        self.calls.append((export_db, library, dry_run))
        if self.error is not None:
            raise self.error
        by_cloud_key = {item.cloud_key: item.uuid for item in self.library}
        matched = 0
        connection = sqlite3.connect(str(export_db))
        with contextlib.closing(connection):
            rows = connection.execute(
                "SELECT uuid, photoinfo FROM photoinfo"
            ).fetchall()
            for uuid, raw in rows:
                document = json.loads(raw)
                key = f"{document['original_filename']}:{document['cloud_guid']}"
                new_uuid = by_cloud_key.get(key)
                if new_uuid is None:
                    continue
                matched += 1
                if self.rewrite and not dry_run:
                    document["uuid"] = new_uuid
                    connection.execute(
                        "UPDATE photoinfo SET uuid = ?, photoinfo = ? WHERE uuid = ?",
                        (new_uuid, json.dumps(document), uuid),
                    )
            connection.commit()
        return matched, len(rows) - matched


class TakeoverTestCase(unittest.TestCase):
    """Gives every test a fake mounted volume holding a fixture export database."""

    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        # Resolved because macOS temporary directories live under symlinks.
        self.root = Path(self._tmpdir.name).resolve()
        self.volume = self.root / "SanDisk"
        self.volume.mkdir()
        self.archive_root = self.volume / "Media" / "Apple Photos"
        self.paths = ArchivePaths(volume=self.volume, archive=self.archive_root)
        self.paths.metadata.mkdir(parents=True)
        self.config = ApplePhotosConfig(
            volume=self.volume,
            archive=self.archive_root,
            library=LIBRARY,
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

    def set_writer(self, hostname: str | None) -> None:
        ArchiveStateStore(self.paths).save(
            ArchiveState(initialized_at=FIXED_NOW, writer_hostname=hostname)
        )

    def writer(self) -> str | None:
        return ArchiveStateStore(self.paths).load().writer_hostname

    @contextlib.contextmanager
    def archive(self, *, hostname: str = NEW_HOST, dry_run: bool = False):
        volume = self.volume
        probes = SystemProbes(
            is_mount=lambda path: path == volume,
            hostname=lambda: hostname,
            now=lambda: FIXED_NOW,
        )
        with open_archive(self.config, dry_run=dry_run, probes=probes) as opened:
            yield opened

    def probes(self, library: tuple[AssetIdentity, ...], **overrides) -> PhotosProbes:
        defaults = {
            "read_library": lambda _: library,
            "migrate": FakeMigrator(library),
        }
        defaults.update(overrides)
        return PhotosProbes(**defaults)


class CopyDatabaseTests(TakeoverTestCase):
    def test_a_copy_replaces_the_destination_and_drops_stale_sidecars(self) -> None:
        source = self.root / "source.db"
        destination = self.root / "destination.db"
        write_export_db(source, (asset("old-1", "guid-1"),))
        write_export_db(destination, (asset("other-1", "guid-9"),))
        stale = destination.with_name(destination.name + "-wal")
        stale.write_bytes(b"stale write-ahead log")

        copy_database(source, destination)

        self.assertEqual(read_uuids(destination), {"old-1"})
        self.assertFalse(stale.exists())

    def test_a_copy_carries_uncheckpointed_write_ahead_log_records(self) -> None:
        source = self.root / "source.db"
        write_export_db(source, (asset("old-1", "guid-1"),))
        connection = sqlite3.connect(str(source))
        connection.execute("PRAGMA journal_mode=WAL")
        connection.execute(
            "INSERT INTO photoinfo(uuid, photoinfo) VALUES (?, ?)",
            ("old-2", json.dumps({"cloud_guid": "guid-2", "original_filename": "b"})),
        )
        connection.commit()

        copy_database(source, self.root / "copy.db")
        connection.close()

        self.assertEqual(read_uuids(self.root / "copy.db"), {"old-1", "old-2"})


if __name__ == "__main__":
    unittest.main()
