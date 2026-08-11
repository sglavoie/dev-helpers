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
    EXPORT_DATABASE,
    MISSING_ASSETS,
    OWNERSHIP,
    PENDING_CLEANUP,
    SIGNATURES,
    STATE,
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


class HealthyArchiveTests(VerifyTestCase):
    def test_a_healthy_archive_passes_every_check(self) -> None:
        report = self.verify()

        self.assertTrue(report.passed, [check.detail for check in report.failed])
        self.assertEqual(len(report.checks), 6)

    def test_verification_writes_nothing(self) -> None:
        before = self.snapshot()

        self.verify()

        self.assertEqual(self.snapshot(), before)
        self.assertFalse(self.paths.lock_file.exists())

    def snapshot(self) -> dict[Path, tuple[int, int]]:
        return {
            path: (path.stat().st_size, path.stat().st_mtime_ns)
            for path in sorted(self.archive_root.rglob("*"))
        }


class ExportDatabaseTests(VerifyTestCase):
    def test_a_missing_export_database_fails(self) -> None:
        self.paths.export_db.unlink()

        check = self.check(self.verify(), EXPORT_DATABASE)

        self.assertFalse(check.passed)
        self.assertIn("does not exist", check.detail)

    def test_a_file_that_is_not_a_database_fails(self) -> None:
        self.paths.export_db.write_bytes(b"this is not a database")

        check = self.check(self.verify(), EXPORT_DATABASE)

        self.assertFalse(check.passed)
        self.assertIn("no readable exported file records", check.detail)

    def test_a_failed_sqlite_integrity_check_fails(self) -> None:
        check = self.check(
            self.verify(check_integrity=lambda _: MALFORMED), EXPORT_DATABASE
        )

        self.assertFalse(check.passed)
        self.assertIn("integrity_check reported", check.detail)
        self.assertIn("Page 4 is never used", check.detail)

    def test_an_unsupported_schema_version_fails(self) -> None:
        write_export_db(self.paths.export_db, [self.exported], version="99.0")

        check = self.check(self.verify(), EXPORT_DATABASE)

        self.assertFalse(check.passed)
        self.assertIn("99.0", check.detail)

    def test_relative_records_are_resolved_beneath_the_archive(self) -> None:
        report = self.verify()

        export_db = self.check(report, EXPORT_DATABASE)
        self.assertTrue(export_db.passed, export_db.detail)
        self.assertTrue(self.check(report, MISSING_ASSETS).passed)
        self.assertTrue(self.check(report, SIGNATURES).passed)

    def test_absolute_records_inside_the_archive_still_pass(self) -> None:
        write_export_db(self.paths.export_db, [self.exported])

        check = self.check(self.verify(), EXPORT_DATABASE)

        self.assertTrue(check.passed, check.detail)

    def test_records_pointing_outside_the_archive_fail(self) -> None:
        stray = self.root / "elsewhere.jpg"
        stray.write_bytes(b"not in the archive")
        write_export_db(self.paths.export_db, [self.exported, stray])

        check = self.check(self.verify(), EXPORT_DATABASE)

        self.assertFalse(check.passed)
        self.assertIn("outside the archive", check.detail)
        self.assertIn(str(stray), check.detail)

    def test_a_relative_record_that_traverses_out_of_the_archive_fails(self) -> None:
        stray = self.root / "elsewhere.jpg"
        stray.write_bytes(b"not in the archive")
        write_export_db(
            self.paths.export_db,
            [self.exported, stray],
            relative_to=self.archive_root,
        )

        check = self.check(self.verify(), EXPORT_DATABASE)

        self.assertFalse(check.passed)
        self.assertIn("outside the archive", check.detail)
        self.assertIn(str(stray), check.detail)


class ExportedFileTests(VerifyTestCase):
    def test_a_deleted_exported_file_is_reported_as_missing(self) -> None:
        self.exported.unlink()

        report = self.verify()

        missing = self.check(report, MISSING_ASSETS)
        self.assertFalse(missing.passed)
        self.assertIn(str(self.exported), missing.detail)
        self.assertTrue(self.check(report, SIGNATURES).passed)

    def test_a_changed_exported_file_fails_its_signature(self) -> None:
        self.exported.write_bytes(b"a different photo entirely")

        report = self.verify()

        signatures = self.check(report, SIGNATURES)
        self.assertFalse(signatures.passed)
        self.assertIn(str(self.exported), signatures.detail)
        self.assertTrue(self.check(report, MISSING_ASSETS).passed)


class StateTests(VerifyTestCase):
    def test_an_uninitialized_archive_fails_the_state_check(self) -> None:
        self.save_state(initialized_at=None)

        check = self.check(self.verify(), STATE)

        self.assertFalse(check.passed)
        self.assertIn("bootstrap", check.detail)

    def test_a_full_export_newer_than_the_last_export_fails(self) -> None:
        self.save_state(last_full_export_at=THURSDAY, last_successful_export_at=MONDAY)

        check = self.check(self.verify(), STATE)

        self.assertFalse(check.passed)
        self.assertIn("newer than", check.detail)

    def test_a_vanished_report_fails_the_state_check(self) -> None:
        self.report.unlink()

        check = self.check(self.verify(), STATE)

        self.assertFalse(check.passed)
        self.assertIn(str(self.report), check.detail)

    def test_unreadable_state_is_reported_instead_of_raised(self) -> None:
        self.paths.state_file.write_text("{not json")

        report = self.verify()

        self.assertFalse(self.check(report, STATE).passed)
        self.assertFalse(self.check(report, OWNERSHIP).passed)
        self.assertFalse(self.check(report, PENDING_CLEANUP).passed)
        self.assertTrue(self.check(report, EXPORT_DATABASE).passed)


class OwnershipTests(VerifyTestCase):
    def test_an_unclaimed_archive_fails_ownership(self) -> None:
        self.save_state(writer_hostname=None)

        check = self.check(self.verify(), OWNERSHIP)

        self.assertFalse(check.passed)
        self.assertIn("no Mac has claimed", check.detail)

    def test_another_mac_owning_the_archive_passes_and_names_it(self) -> None:
        self.save_state(writer_hostname=OTHER_HOST)

        check = self.check(self.verify(), OWNERSHIP)

        self.assertTrue(check.passed)
        self.assertIn(OTHER_HOST, check.detail)
        self.assertIn(HOSTNAME, check.detail)


class PendingCleanupTests(VerifyTestCase):
    def test_a_pending_cleanup_fails_until_it_is_approved(self) -> None:
        self.save_state(pending_cleanup_run_id="2026-08-13T09:30")

        check = self.check(self.verify(), PENDING_CLEANUP)

        self.assertFalse(check.passed)
        self.assertIn("2026-08-13T09:30", check.detail)
        self.assertIn("approve-cleanup", check.detail)


if __name__ == "__main__":
    unittest.main()
