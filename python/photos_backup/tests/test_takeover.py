from __future__ import annotations

import contextlib
import datetime
import json
import sqlite3
import tempfile
import unittest
from pathlib import Path

from photos_backup.apple_photos.adapter import (
    PhotosProbes,
    export_db_version_supported,
    read_export_db_assets,
    read_export_db_version,
)
from photos_backup.apple_photos.identity import (
    AssetIdentity,
    WriterStatus,
    assess_library,
    compare_library,
)
from photos_backup.apple_photos.takeover import (
    copy_database,
    ensure_writer,
)
from photos_backup.archive import (
    ArchivePaths,
    ArchiveUnsafe,
    ArchiveState,
    ArchiveStateStore,
    SystemProbes,
    open_archive,
)
from photos_backup.config import ApplePhotosConfig
from photos_backup.errors import ActionRequired

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


class ComparisonTests(unittest.TestCase):
    def test_records_matched_by_cloud_guid_with_a_new_uuid_are_migratable(self) -> None:
        comparison = compare_library(
            [asset("old-1", "guid-1"), asset("old-2", "guid-2")],
            [asset("new-1", "guid-1"), asset("new-2", "guid-2")],
        )

        self.assertEqual(comparison.recorded, 2)
        self.assertEqual(comparison.matched, 2)
        self.assertEqual(comparison.uuid_changed, 2)
        self.assertEqual(comparison.absent, ())

    def test_the_same_uuid_on_both_sides_needs_no_migration(self) -> None:
        comparison = compare_library(
            [asset("same-1", "guid-1")], [asset("same-1", "guid-1")]
        )

        self.assertEqual(comparison.uuid_changed, 0)

    def test_a_record_whose_cloud_guid_is_gone_is_absent(self) -> None:
        comparison = compare_library(
            [asset("old-1", "guid-1"), asset("old-2", "guid-2")],
            [asset("new-1", "guid-1")],
        )

        self.assertEqual([item.uuid for item in comparison.absent], ["old-2"])
        self.assertEqual(comparison.absent_fraction, 0.5)

    def test_a_matching_cloud_guid_under_another_filename_does_not_match(self) -> None:
        comparison = compare_library(
            [asset("old-1", "guid-1", "IMG_0001.HEIC")],
            [asset("new-1", "guid-1", "IMG_9999.HEIC")],
        )

        self.assertEqual(comparison.matched, 0)
        self.assertEqual(comparison.absent_count, 1)

    def test_records_without_a_cloud_guid_are_unidentifiable_not_absent(self) -> None:
        comparison = compare_library(
            [asset("old-1", None), asset("old-2", "guid-2")],
            [asset("new-2", "guid-2")],
        )

        self.assertEqual(comparison.unidentifiable, 1)
        self.assertEqual(comparison.comparable, 1)
        self.assertEqual(comparison.absent, ())


class VerdictTests(unittest.TestCase):
    def assess(self, recorded, library, **overrides):
        limits = {"max_absent_assets": 10, "max_absent_fraction": 0.001}
        limits.update(overrides)
        return assess_library(compare_library(recorded, library), **limits)

    def test_an_empty_export_database_has_nothing_to_protect(self) -> None:
        verdict = self.assess([], [asset("new-1", "guid-1")])

        self.assertFalse(verdict.blocked)
        self.assertFalse(verdict.migration_required)

    def test_a_library_without_one_shared_asset_is_the_wrong_library(self) -> None:
        verdict = self.assess([asset("old-1", "guid-1")], [asset("new-9", "guid-9")])

        self.assertIn("different library", str(verdict.blocked_reason))

    def test_records_without_cloud_guids_cannot_identify_a_library(self) -> None:
        verdict = self.assess([asset("old-1", None)], [asset("new-1", "guid-1")])

        self.assertIn("cannot be identified", str(verdict.blocked_reason))

    def test_absence_over_the_asset_cap_blocks_below_the_fraction(self) -> None:
        recorded = [asset(f"old-{index}", f"guid-{index}") for index in range(12000)]
        library = [asset(f"new-{index}", f"guid-{index}") for index in range(11989)]

        verdict = self.assess(recorded, library)

        self.assertIn("over the limit", str(verdict.blocked_reason))

    def test_absence_over_the_fraction_blocks_below_the_asset_cap(self) -> None:
        recorded = [asset(f"old-{index}", f"guid-{index}") for index in range(1000)]
        library = [asset(f"new-{index}", f"guid-{index}") for index in range(998)]

        verdict = self.assess(recorded, library)

        self.assertIn("over the limit", str(verdict.blocked_reason))

    def test_a_small_absent_set_becomes_deletion_candidates(self) -> None:
        recorded = [asset(f"old-{index}", f"guid-{index}") for index in range(5000)]
        library = [asset(f"new-{index}", f"guid-{index}") for index in range(4995)]

        verdict = self.assess(recorded, library)

        self.assertFalse(verdict.blocked)
        self.assertTrue(verdict.migration_required)
        self.assertEqual(len(verdict.deletion_candidates), 5)


class ExportDbReaderTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        self.root = Path(self._tmpdir.name).resolve()
        self.export_db = self.root / "export.db"

    def test_a_missing_database_reads_as_no_records(self) -> None:
        self.assertEqual(read_export_db_assets(self.export_db), ())
        self.assertIsNone(read_export_db_version(self.export_db))

    def test_current_records_are_read_with_their_cloud_guids(self) -> None:
        write_export_db(self.export_db, (asset("old-1", "guid-1"),))

        self.assertEqual(
            read_export_db_assets(self.export_db),
            (AssetIdentity("old-1", "guid-1", "guid-1.HEIC"),),
        )
        self.assertEqual(read_export_db_version(self.export_db), "11.0")

    def test_a_legacy_info_table_is_read_the_same_way(self) -> None:
        write_export_db(
            self.export_db,
            (asset("old-1", "guid-1"),),
            version="4.3",
            table="info",
            column="json_info",
        )

        self.assertEqual(
            read_export_db_assets(self.export_db),
            (AssetIdentity("old-1", "guid-1", "guid-1.HEIC"),),
        )

    def test_an_unreadable_record_is_unidentifiable_rather_than_fatal(self) -> None:
        write_export_db(self.export_db, ())
        connection = sqlite3.connect(str(self.export_db))
        with contextlib.closing(connection):
            connection.execute(
                "INSERT INTO photoinfo(uuid, photoinfo) VALUES (?, ?)",
                ("old-1", "{not json"),
            )
            connection.commit()

        self.assertEqual(
            read_export_db_assets(self.export_db), (AssetIdentity("old-1", None, ""),)
        )

    def test_a_file_without_asset_records_is_refused_not_read_as_empty(self) -> None:
        self.export_db.write_bytes(b"this is not a database")

        with self.assertRaises(ArchiveUnsafe) as raised:
            read_export_db_assets(self.export_db)

        self.assertIn("no readable asset records", str(raised.exception))

    def test_only_versions_this_osxphotos_understands_are_supported(self) -> None:
        self.assertTrue(export_db_version_supported("11.0"))
        self.assertTrue(export_db_version_supported("4.3"))
        self.assertFalse(export_db_version_supported("12.0"))
        self.assertFalse(export_db_version_supported(None))
        self.assertFalse(export_db_version_supported("not-a-version"))


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


class EnsureWriterTests(TakeoverTestCase):
    def test_an_unchanged_writer_never_opens_the_photos_library(self) -> None:
        self.set_writer(NEW_HOST)

        def refuse(_: Path):
            raise AssertionError("the Photos library must not be read")

        with self.archive() as opened:
            check = ensure_writer(
                self.config, opened, probes=self.probes((), read_library=refuse)
            )

        self.assertIs(check.status, WriterStatus.UNCHANGED)
        self.assertIsNone(check.verdict)

    def test_an_archive_without_records_is_claimed_without_a_comparison(self) -> None:
        self.set_writer(None)

        with self.archive() as opened:
            check = ensure_writer(self.config, opened, probes=self.probes(()))

        self.assertIs(check.status, WriterStatus.CLAIMED)
        self.assertEqual(self.writer(), NEW_HOST)

    def test_an_unsupported_export_database_blocks_the_takeover(self) -> None:
        self.set_writer(OLD_HOST)
        write_export_db(
            self.paths.export_db, (asset("old-1", "guid-1"),), version="12.0"
        )
        library = (asset("new-1", "guid-1"),)

        with self.archive() as opened, self.assertRaises(ActionRequired) as raised:
            ensure_writer(self.config, opened, probes=self.probes(library))

        self.assertIn("12.0", str(raised.exception))
        self.assertEqual(self.writer(), OLD_HOST)

    def test_a_takeover_migrates_changed_uuids_and_records_the_writer(self) -> None:
        self.set_writer(OLD_HOST)
        write_export_db(
            self.paths.export_db,
            (asset("old-1", "guid-1"), asset("old-2", "guid-2")),
        )
        library = (asset("new-1", "guid-1"), asset("new-2", "guid-2"))
        migrator = FakeMigrator(library)

        with self.archive() as opened:
            check = ensure_writer(
                self.config, opened, probes=self.probes(library, migrate=migrator)
            )

        self.assertIs(check.status, WriterStatus.TAKEOVER)
        self.assertEqual(self.writer(), NEW_HOST)
        self.assertTrue(check.migration.performed)
        self.assertEqual(check.migration.matched, 2)
        self.assertEqual(read_uuids(self.paths.export_db), {"new-1", "new-2"})
        self.assertTrue(self.paths.export_db_backup.exists())
        self.assertEqual(read_uuids(self.paths.export_db_backup), {"old-1", "old-2"})

    def test_an_unchanged_library_takes_over_without_migrating(self) -> None:
        self.set_writer(OLD_HOST)
        write_export_db(self.paths.export_db, (asset("same-1", "guid-1"),))
        library = (asset("same-1", "guid-1"),)
        migrator = FakeMigrator(library)

        with self.archive() as opened:
            check = ensure_writer(
                self.config, opened, probes=self.probes(library, migrate=migrator)
            )

        self.assertIs(check.status, WriterStatus.TAKEOVER)
        self.assertEqual(migrator.calls, [])
        self.assertIsNone(check.migration)
        self.assertFalse(self.paths.export_db_backup.exists())
        self.assertEqual(self.writer(), NEW_HOST)

    def test_a_mismatched_library_blocks_and_changes_nothing(self) -> None:
        self.set_writer(OLD_HOST)
        write_export_db(self.paths.export_db, (asset("old-1", "guid-1"),))
        library = (asset("new-9", "guid-9"),)
        migrator = FakeMigrator(library)

        with self.archive() as opened, self.assertRaises(ActionRequired) as raised:
            ensure_writer(
                self.config, opened, probes=self.probes(library, migrate=migrator)
            )

        self.assertIn("different library", str(raised.exception))
        self.assertEqual(migrator.calls, [])
        self.assertEqual(self.writer(), OLD_HOST)
        self.assertEqual(read_uuids(self.paths.export_db), {"old-1"})

    def test_an_incomplete_library_blocks_before_any_migration(self) -> None:
        self.set_writer(OLD_HOST)
        recorded = tuple(asset(f"old-{i}", f"guid-{i}") for i in range(20))
        write_export_db(self.paths.export_db, recorded)
        library = tuple(asset(f"new-{i}", f"guid-{i}") for i in range(5))

        with self.archive() as opened, self.assertRaises(ActionRequired) as raised:
            ensure_writer(self.config, opened, probes=self.probes(library))

        self.assertIn("absent from this library", str(raised.exception))
        self.assertEqual(self.writer(), OLD_HOST)

    def test_a_failed_migration_is_restored_and_leaves_the_writer_alone(self) -> None:
        self.set_writer(OLD_HOST)
        write_export_db(self.paths.export_db, (asset("old-1", "guid-1"),))
        library = (asset("new-1", "guid-1"),)
        migrator = FakeMigrator(library, error=RuntimeError("library disappeared"))

        with self.archive() as opened, self.assertRaises(ActionRequired) as raised:
            ensure_writer(
                self.config, opened, probes=self.probes(library, migrate=migrator)
            )

        self.assertIn("restored from", str(raised.exception))
        self.assertEqual(read_uuids(self.paths.export_db), {"old-1"})
        self.assertEqual(self.writer(), OLD_HOST)

    def test_a_migration_that_leaves_stale_uuids_is_rolled_back(self) -> None:
        self.set_writer(OLD_HOST)
        write_export_db(self.paths.export_db, (asset("old-1", "guid-1"),))
        library = (asset("new-1", "guid-1"),)
        migrator = FakeMigrator(library, rewrite=False)

        with self.archive() as opened, self.assertRaises(ActionRequired) as raised:
            ensure_writer(
                self.config, opened, probes=self.probes(library, migrate=migrator)
            )

        self.assertIn("still point at the previous library", str(raised.exception))
        self.assertEqual(read_uuids(self.paths.export_db), {"old-1"})
        self.assertEqual(self.writer(), OLD_HOST)

    def test_a_dry_run_reports_the_migration_without_performing_it(self) -> None:
        self.set_writer(OLD_HOST)
        write_export_db(self.paths.export_db, (asset("old-1", "guid-1"),))
        library = (asset("new-1", "guid-1"),)
        migrator = FakeMigrator(library)

        with self.archive(dry_run=True) as opened:
            check = ensure_writer(
                self.config, opened, probes=self.probes(library, migrate=migrator)
            )

        self.assertIs(check.status, WriterStatus.TAKEOVER)
        self.assertFalse(check.migration.performed)
        self.assertEqual(migrator.calls[0][2], True)
        self.assertFalse(self.paths.export_db_backup.exists())
        self.assertEqual(read_uuids(self.paths.export_db), {"old-1"})
        self.assertEqual(self.writer(), OLD_HOST)

    def test_absent_assets_within_the_limits_travel_as_deletion_candidates(
        self,
    ) -> None:
        self.set_writer(OLD_HOST)
        recorded = tuple(asset(f"old-{i}", f"guid-{i}") for i in range(5000))
        write_export_db(self.paths.export_db, recorded)
        library = tuple(asset(f"new-{i}", f"guid-{i}") for i in range(4998))

        with self.archive() as opened:
            check = ensure_writer(
                self.config,
                opened,
                probes=self.probes(library, migrate=FakeMigrator(library)),
            )

        self.assertEqual(
            [item.uuid for item in check.deletion_candidates], ["old-4998", "old-4999"]
        )


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
