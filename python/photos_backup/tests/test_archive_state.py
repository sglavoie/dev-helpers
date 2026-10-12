from __future__ import annotations

import datetime
import json

from photos_backup.archive import (
    STATE_VERSION,
    ArchivePaths,
    ArchiveState,
    ArchiveStateStore,
    ArchiveUnavailable,
    ArchiveUnsafe,
    open_archive,
    safe_name,
)

HOST = "Sebastiens MacBook.local"
FIXED_NOW = datetime.datetime(2026, 8, 11, 9, 30, tzinfo=datetime.UTC)

from tests.archive_store_case import FIXED_NOW, HOST, ArchiveTestCase


class ArchiveStateTests(ArchiveTestCase):
    def store(self, *, dry_run: bool = False) -> ArchiveStateStore:
        self.archive.mkdir(parents=True)
        paths = ArchivePaths(volume=self.volume, archive=self.archive)
        paths.metadata.mkdir()
        return ArchiveStateStore(paths, dry_run=dry_run)

    def test_missing_state_reads_as_uninitialized(self) -> None:
        state = self.store().load()
        self.assertEqual(state, ArchiveState())
        self.assertFalse(state.initialized)

    def test_atomic_round_trip_keeps_every_field(self) -> None:
        store = self.store()
        report = self.archive / ".photos-backup" / "reports" / "run.csv"
        saved = ArchiveState(
            initialized_at=FIXED_NOW,
            last_successful_export_at=FIXED_NOW,
            last_full_export_at=FIXED_NOW - datetime.timedelta(days=3),
            last_mirror_completed_at=FIXED_NOW - datetime.timedelta(days=1),
            writer_hostname="Sebastiens MacBook.local",
            last_report_path=report,
            pending_cleanup_run_id="2026-08-11T09:30:00",
        )
        store.save(saved)
        self.assertEqual(store.load(), saved)
        self.assertEqual(
            json.loads(store.path.read_text())["last_report_path"],
            ".photos-backup/reports/run.csv",
        )
        self.assertEqual([p.name for p in store.path.parent.iterdir()], ["state.json"])

    def test_legacy_state_is_upgraded_only_on_write(self) -> None:
        store = self.store()
        store.path.write_text(
            json.dumps(
                {
                    "version": 1,
                    "last_report_path": "/Volumes/Old/Archive/.photos-backup/reports/run.csv",
                    "writer_hostname": "first-mac",
                }
            )
        )
        before = store.path.read_bytes()
        state = store.load()
        self.assertEqual(state.version, STATE_VERSION)
        self.assertEqual(
            state.last_report_path, self.archive / ".photos-backup/reports/run.csv"
        )
        self.assertEqual(store.path.read_bytes(), before)
        store.update(writer_hostname="second-mac")
        document = json.loads(store.path.read_text())
        self.assertEqual(document["version"], STATE_VERSION)
        self.assertEqual(document["last_report_path"], ".photos-backup/reports/run.csv")

    def test_unrelated_absolute_report_paths_keep_their_meaning(self) -> None:
        store = self.store()
        report = self.root / "external.csv"
        for version in (1, STATE_VERSION):
            with self.subTest(version=version):
                store.path.write_text(
                    json.dumps({"version": version, "last_report_path": str(report)})
                )
                self.assertEqual(store.load().last_report_path, report)

    def test_unsafe_relative_report_paths_are_rejected_without_writes(self) -> None:
        store = self.store()
        for report in ("../outside.csv", ".photos-backup/../../outside.csv", "."):
            with self.subTest(report=report):
                store.path.write_text(
                    json.dumps({"version": STATE_VERSION, "last_report_path": report})
                )
                before = store.path.read_bytes()
                with self.assertRaises(ArchiveUnsafe):
                    store.load()
                self.assertEqual(store.path.read_bytes(), before)

    def test_update_advances_only_named_fields(self) -> None:
        store = self.store()
        store.save(ArchiveState(writer_hostname="first-mac"))
        updated = store.update(last_successful_export_at=FIXED_NOW)
        self.assertEqual(updated.writer_hostname, "first-mac")
        self.assertEqual(store.load().last_successful_export_at, FIXED_NOW)

    def test_state_is_versioned_on_disk(self) -> None:
        store = self.store()
        store.save(ArchiveState())
        document = json.loads(store.path.read_text())
        self.assertEqual(document["version"], STATE_VERSION)

    def test_unknown_version_fails_closed(self) -> None:
        store = self.store()
        store.path.write_text(json.dumps({"version": STATE_VERSION + 1}))
        with self.assertRaises(ArchiveUnsafe) as caught:
            store.load()
        self.assertIn("only understands version", str(caught.exception))

    def test_corrupt_state_fails_closed(self) -> None:
        store = self.store()
        store.path.write_text("{not json")
        with self.assertRaises(ArchiveUnsafe):
            store.load()

    def test_non_integer_versions_fail_closed(self) -> None:
        store = self.store()
        for version in (True, False, 1.0, "1", None):
            with self.subTest(version=version):
                store.path.write_text(json.dumps({"version": version}))
                before = store.path.read_bytes()
                with self.assertRaises(ArchiveUnsafe) as caught:
                    store.load()
                self.assertIn("only understands version", str(caught.exception))
                self.assertEqual(store.path.read_bytes(), before)

    def test_invalid_utf8_state_fails_with_an_archive_error(self) -> None:
        store = self.store()
        store.path.write_bytes(b'{"version": 1, "writer_hostname": "\xff"}')
        before = store.path.read_bytes()
        with self.assertRaises(ArchiveUnsafe) as caught:
            store.load()
        self.assertIn("not valid UTF-8", str(caught.exception))
        self.assertIn(str(store.path), str(caught.exception))
        self.assertEqual(store.path.read_bytes(), before)

    def test_unknown_key_fails_closed(self) -> None:
        store = self.store()
        store.path.write_text(json.dumps({"version": STATE_VERSION, "wat": 1}))
        with self.assertRaises(ArchiveUnsafe) as caught:
            store.load()
        self.assertIn("unknown key(s) wat", str(caught.exception))

    def test_naive_timestamp_fails_closed(self) -> None:
        store = self.store()
        store.path.write_text(
            json.dumps({"version": STATE_VERSION, "initialized_at": "2026-08-11"})
        )
        with self.assertRaises(ArchiveUnsafe) as caught:
            store.load()
        self.assertIn("must carry a timezone", str(caught.exception))

    def test_unwritable_state_directory_fails_closed(self) -> None:
        store = self.store()
        store.save(ArchiveState())
        store.path.parent.chmod(0o500)
        self.addCleanup(store.path.parent.chmod, 0o700)
        with self.assertRaises(ArchiveUnavailable):
            store.save(ArchiveState(writer_hostname="second-mac"))


class DryRunTests(ArchiveTestCase):
    def test_dry_run_persists_nothing(self) -> None:
        with open_archive(self.config, dry_run=True, probes=self.probes) as archive:
            self.assertTrue(archive.dry_run)
            self.assertFalse(archive.state_store.load().initialized)
            archive.state_store.save(ArchiveState(initialized_at=FIXED_NOW))
            archive.state_store.update(writer_hostname="ghost")
        self.assertFalse(self.archive.exists())
        self.assert_untouched()

    def test_dry_run_reads_existing_state(self) -> None:
        with open_archive(self.config, probes=self.probes) as archive:
            archive.state_store.update(
                initialized_at=FIXED_NOW, writer_hostname="first-mac"
            )
        with open_archive(self.config, dry_run=True, probes=self.probes) as archive:
            state = archive.state_store.load()
            self.assertTrue(state.initialized)
            self.assertEqual(state.writer_hostname, "first-mac")
            archive.state_store.update(writer_hostname="ghost")
        with open_archive(self.config, dry_run=True, probes=self.probes) as archive:
            self.assertEqual(archive.state_store.load().writer_hostname, "first-mac")


class ArchivePathLayoutTests(ArchiveTestCase):
    def paths(self) -> ArchivePaths:
        return ArchivePaths(volume=self.volume, archive=self.archive)

    def test_every_owned_path_lives_under_one_hidden_directory(self) -> None:
        paths = self.paths()
        metadata = self.archive / ".photos-backup"
        self.assertEqual(paths.metadata, metadata)
        self.assertEqual(paths.export_db, metadata / "export.db")
        self.assertEqual(paths.state_file, metadata / "state.json")
        self.assertEqual(paths.lock_file, metadata / "archive.lock")
        self.assertEqual(
            paths.export_db_backup,
            metadata / "migrations" / "export.db.last-known-good",
        )
        self.assertEqual(
            paths.cleanup_manifest("2026-08-11T09:30:00"),
            metadata / "cleanup" / "2026-08-11T09-30-00.json",
        )

    def test_reports_are_named_for_host_and_day(self) -> None:
        paths = self.paths()
        day = datetime.date(2026, 8, 11)
        self.assertEqual(
            paths.export_report("Sebastiens MacBook.local", day),
            paths.reports / "photos_export_Sebastiens-MacBook-local_2026-08-11.csv",
        )
        self.assertEqual(
            paths.late_additions_report("Sebastiens MacBook.local", day),
            paths.reports
            / "late_photo_additions_Sebastiens-MacBook-local_2026-08-11.csv",
        )

    def test_a_later_run_the_same_day_is_given_unused_report_paths(self) -> None:
        paths = self.paths()
        day = datetime.date(2026, 8, 11)
        paths.reports.mkdir(parents=True)

        self.assertEqual(paths.next_report_sequence(HOST, day), 1)
        paths.export_report(HOST, day).write_text("filename\n")
        self.assertEqual(paths.next_report_sequence(HOST, day), 2)
        self.assertEqual(
            paths.export_report(HOST, day, 2),
            paths.reports / "photos_export_Sebastiens-MacBook-local_2026-08-11_2.csv",
        )
        self.assertEqual(
            paths.late_additions_report(HOST, day, 2),
            paths.reports
            / "late_photo_additions_Sebastiens-MacBook-local_2026-08-11_2.csv",
        )

    def test_a_late_additions_report_alone_also_consumes_a_sequence(self) -> None:
        paths = self.paths()
        day = datetime.date(2026, 8, 11)
        paths.reports.mkdir(parents=True)
        paths.late_additions_report(HOST, day).write_text("filename\n")

        self.assertEqual(paths.next_report_sequence(HOST, day), 2)

    def test_unusable_host_names_still_produce_one_path_part(self) -> None:
        self.assertEqual(safe_name("../../etc"), "etc")
        self.assertEqual(safe_name("///"), "unknown")
