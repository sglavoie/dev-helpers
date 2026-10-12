from __future__ import annotations

import json
import shutil
import sqlite3
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

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
    SystemProbes,
    open_archive,
)
from photos_backup.cli.cli import cli
from photos_backup.progress import ExportProgress
from tests.test_export import HOSTNAME, make_config
from tests.verify_case import (
    MALFORMED,
    MONDAY,
    OTHER_HOST,
    THURSDAY,
    VerifyTestCase,
    write_export_db,
)


class HealthyArchiveTests(VerifyTestCase):
    def test_progress_counts_missing_files_without_changing_findings_or_state(self):
        self.exported.unlink()
        before = self.paths.state_file.read_bytes()
        expected = self.verify()
        lines = []
        with (
            open_archive(
                self.config, dry_run=True, probes=self.system_probes
            ) as archive,
            ExportProgress(
                sink=lines.append,
                terminal=False,
                item_label="files checked",
                show_downloads=False,
            ) as progress,
        ):
            actual = verify_archive(archive, progress=progress)
        self.assertEqual(actual, expected)
        self.assertEqual(progress.processed, 1)
        self.assertTrue(any("files checked 1/1" in line for line in lines))
        self.assertFalse(progress._thread.is_alive())
        self.assertEqual(self.paths.state_file.read_bytes(), before)
        self.assertFalse(self.paths.lock_file.exists())

    def test_moved_archive_verifies_and_status_finds_legacy_and_portable_reports(self):
        for legacy in (False, True):
            with self.subTest(legacy=legacy):
                volume = self.root / f"Moved-{legacy}"
                target = volume / "Archive"
                shutil.copytree(self.archive_root, target)
                paths = ArchivePaths(volume=volume, archive=target)
                if legacy:
                    document = json.loads(paths.state_file.read_text())
                    document["version"] = 1
                    document["last_report_path"] = str(self.report)
                    paths.state_file.write_text(json.dumps(document))
                before = paths.state_file.read_bytes()
                probes = SystemProbes(
                    is_mount=lambda path: path == volume,
                    hostname=lambda: HOSTNAME,
                    now=lambda: THURSDAY,
                )
                with open_archive(
                    make_config(volume, target), dry_run=True, probes=probes
                ) as archive:
                    report = verify_archive(archive)
                self.assertTrue(report.passed, report.failed)
                config = self.root / "moved.toml"
                config.write_text(
                    f'[apple_photos]\nvolume = "{volume}"\narchive = "{target}"\n'
                    'library = "/unused/Photos.photoslibrary"\n'
                )
                with mock.patch(
                    "photos_backup.archive.probes.os.path.ismount", return_value=True
                ):
                    result = CliRunner().invoke(
                        cli, ["--config", str(config), "status"]
                    )
                self.assertEqual(result.exit_code, 0, result.output)
                self.assertIn(str(paths.reports / self.report.name), result.output)
                self.assertNotIn(str(self.report), result.output)
                self.assertEqual(paths.state_file.read_bytes(), before)
                # The report at the old location must not conceal a missing copy.
                (paths.reports / self.report.name).unlink()
                with open_archive(
                    make_config(volume, target), dry_run=True, probes=probes
                ) as archive:
                    report = verify_archive(archive)
                self.assertFalse(self.check(report, STATE).passed)

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


class SignatureCoverageTests(VerifyTestCase):
    def test_missing_signature_fields_are_explicit_without_changing_exit_policy(self):
        for field in ("dest_size", "dest_mtime"):
            write_export_db(self.paths.export_db, [self.exported])
            with sqlite3.connect(self.paths.export_db) as connection:
                connection.execute(f"UPDATE export_data SET {field} = NULL")
            report = self.verify()
            check = self.check(report, SIGNATURES)
            self.assertTrue(report.passed)
            self.assertIn("Coverage incomplete", check.detail)
            self.assertIn("0 matched, 0 changed, 1 missing signatures", check.detail)
            self.assertIn("file contents were not checksummed", check.detail)

    def test_mixed_coverage_accounts_for_every_record(self):
        changed, unsigned, missing = (
            self.archive_root / name
            for name in ("changed.jpg", "unsigned.jpg", "missing.jpg")
        )
        for path in (changed, unsigned, missing):
            path.write_bytes(b"photo")
        write_export_db(
            self.paths.export_db, [self.exported, changed, unsigned, missing]
        )
        with sqlite3.connect(self.paths.export_db) as connection:
            connection.execute(
                "UPDATE export_data SET dest_size = NULL WHERE uuid = 'uuid-2'"
            )
        changed.write_bytes(b"changed photo")
        missing.unlink()
        check = self.check(self.verify(), SIGNATURES)
        self.assertFalse(check.passed)
        self.assertIn(
            "1 matched, 1 changed, 1 missing signatures, 1 unavailable", check.detail
        )
        self.assertEqual(check.paths, (changed,))


if __name__ == "__main__":
    unittest.main()
