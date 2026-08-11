from __future__ import annotations

import datetime
import json
import tempfile
import unittest
from pathlib import Path

from photos_backup.archive import (
    STATE_VERSION,
    ArchiveLocked,
    ArchivePaths,
    ArchiveState,
    ArchiveStateStore,
    ArchiveUnavailable,
    ArchiveUnsafe,
    SystemProbes,
    open_archive,
    resolve_archive,
    safe_name,
)
from photos_backup.config import ApplePhotosConfig

HOST = "Sebastiens MacBook.local"
FIXED_NOW = datetime.datetime(2026, 8, 11, 9, 30, tzinfo=datetime.UTC)


def make_config(volume: Path, archive: Path) -> ApplePhotosConfig:
    return ApplePhotosConfig(
        volume=volume,
        archive=archive,
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


class ArchiveSafetyTests(ArchiveTestCase):
    def test_unmounted_volume_fails_closed(self) -> None:
        probes = self.make_probes(mounted=False)
        with self.assertRaises(ArchiveUnavailable) as caught:
            resolve_archive(self.config, probes)
        self.assertIn("is not a mount point", str(caught.exception))
        self.assert_untouched()

    def test_missing_volume_is_never_created(self) -> None:
        missing = self.root / "Absent"
        config = make_config(missing, missing / "Media" / "Apple Photos")
        with self.assertRaises(ArchiveUnavailable):
            with open_archive(config, probes=self.probes):
                self.fail("archive must not open on a missing volume")
        self.assertFalse(missing.exists())

    def test_symlinked_volume_fails_closed(self) -> None:
        target = self.root / "elsewhere"
        target.mkdir()
        linked = self.root / "LinkedVolume"
        linked.symlink_to(target)
        config = make_config(linked, linked / "Media" / "Apple Photos")
        probes = SystemProbes(
            is_mount=lambda path: True,
            hostname=lambda: "test",
            now=lambda: FIXED_NOW,
        )
        with self.assertRaises(ArchiveUnsafe) as caught:
            resolve_archive(config, probes)
        self.assertIn("is a symlink", str(caught.exception))

    def test_symlinked_archive_component_fails_closed(self) -> None:
        escape = self.root / "escape"
        escape.mkdir()
        (self.volume / "Media").symlink_to(escape)
        with self.assertRaises(ArchiveUnsafe) as caught:
            resolve_archive(self.config, self.probes)
        self.assertIn("could escape volume", str(caught.exception))
        self.assertEqual(list(escape.iterdir()), [])

    def test_parent_traversal_fails_closed(self) -> None:
        config = make_config(self.volume, Path(f"{self.volume}/../escape"))
        with self.assertRaises(ArchiveUnsafe) as caught:
            resolve_archive(config, self.probes)
        self.assertIn("must not contain '..'", str(caught.exception))

    def test_archive_equal_to_volume_fails_closed(self) -> None:
        config = make_config(self.volume, self.volume)
        with self.assertRaises(ArchiveUnsafe):
            resolve_archive(config, self.probes)

    def test_archive_outside_volume_fails_closed(self) -> None:
        config = make_config(self.volume, self.root / "other" / "Apple Photos")
        with self.assertRaises(ArchiveUnsafe):
            resolve_archive(config, self.probes)

    def test_component_that_is_a_file_fails_closed(self) -> None:
        (self.volume / "Media").write_text("not a directory")
        with self.assertRaises(ArchiveUnsafe) as caught:
            resolve_archive(self.config, self.probes)
        self.assertIn("is not a directory", str(caught.exception))

    def test_relative_paths_fail_closed(self) -> None:
        config = make_config(Path("SanDisk"), Path("SanDisk/Media"))
        with self.assertRaises(ArchiveUnsafe) as caught:
            resolve_archive(config, self.probes)
        self.assertIn("must be absolute", str(caught.exception))

    def test_creates_only_the_archive_subtree(self) -> None:
        with open_archive(self.config, probes=self.probes) as archive:
            self.assertTrue(archive.paths.archive.is_dir())
            self.assertTrue(archive.paths.reports.is_dir())
            self.assertTrue(archive.paths.migrations.is_dir())
            self.assertTrue(archive.paths.cleanup.is_dir())
        self.assertEqual([p.name for p in self.volume.iterdir()], ["Media"])
        self.assertEqual([p.name for p in self.root.iterdir()], ["SanDisk"])


class ArchiveLockTests(ArchiveTestCase):
    def test_runs_cannot_overlap(self) -> None:
        with open_archive(self.config, probes=self.probes):
            with self.assertRaises(ArchiveLocked) as caught:
                with open_archive(self.config, probes=self.probes):
                    self.fail("a second run must not acquire the archive lock")
        self.assertIn("Sebastiens MacBook.local", str(caught.exception))

    def test_lock_is_released_when_the_block_ends(self) -> None:
        with open_archive(self.config, probes=self.probes):
            pass
        with open_archive(self.config, probes=self.probes) as archive:
            self.assertTrue(archive.paths.lock_file.is_file())

    def test_lock_is_released_after_a_failure(self) -> None:
        with self.assertRaises(RuntimeError):
            with open_archive(self.config, probes=self.probes):
                raise RuntimeError("export blew up")
        with open_archive(self.config, probes=self.probes):
            pass

    def test_dry_run_refuses_to_read_during_a_real_run(self) -> None:
        with open_archive(self.config, probes=self.probes):
            with self.assertRaises(ArchiveLocked):
                with open_archive(self.config, dry_run=True, probes=self.probes):
                    self.fail("a dry run must not read a locked archive")

    def test_dry_runs_do_not_block_each_other(self) -> None:
        with open_archive(self.config, probes=self.probes):
            pass
        with open_archive(self.config, dry_run=True, probes=self.probes):
            with open_archive(self.config, dry_run=True, probes=self.probes):
                pass


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
        self.assertEqual([p.name for p in store.path.parent.iterdir()], ["state.json"])

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


class ProbeInjectionTests(ArchiveTestCase):
    def test_hostname_and_clock_come_from_the_probes(self) -> None:
        with open_archive(self.config, probes=self.probes) as archive:
            self.assertEqual(archive.hostname, "Sebastiens MacBook.local")
            self.assertEqual(archive.now(), FIXED_NOW)
            self.assertIn(FIXED_NOW.isoformat(), archive.paths.lock_file.read_text())


if __name__ == "__main__":
    unittest.main()
