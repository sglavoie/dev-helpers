from __future__ import annotations

import unittest
from pathlib import Path

from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.takeover import ensure_writer
from photos_backup.errors import ActionRequired
from tests.test_takeover import (
    NEW_HOST,
    OLD_HOST,
    FakeMigrator,
    TakeoverTestCase,
    asset,
    read_uuids,
    write_export_db,
)


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


if __name__ == "__main__":
    unittest.main()
