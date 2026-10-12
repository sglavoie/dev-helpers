from __future__ import annotations

import datetime
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.archive import (
    ArchiveLocked,
    ArchivePaths,
    ArchiveUnavailable,
    ArchiveUnsafe,
    SystemProbes,
    open_archive,
    resolve_archive,
)
from photos_backup.archive.lock import archive_lock

HOST = "Sebastiens MacBook.local"
FIXED_NOW = datetime.datetime(2026, 8, 11, 9, 30, tzinfo=datetime.UTC)

from tests.archive_store_case import FIXED_NOW, ArchiveTestCase, make_config


class ArchiveSafetyTests(ArchiveTestCase):
    def test_unmounted_volume_fails_closed(self) -> None:
        probes = self.make_probes(mounted=False)
        with self.assertRaises(ArchiveUnavailable) as caught:
            resolve_archive(self.config, probes)
        self.assertIn("is not a mount point", str(caught.exception))
        self.assert_untouched()

    def test_overridden_volume_need_not_be_a_mount_point(self) -> None:
        local = self.root / "some" / "path"
        local.mkdir(parents=True)
        config = make_config(
            local, local / "Media" / "Apple Photos", require_mounted_volume=False
        )

        paths = resolve_archive(config, self.make_probes(mounted=False))

        self.assertEqual(paths.volume, local)
        self.assertEqual(paths.archive, local / "Media" / "Apple Photos")

    def test_missing_overridden_volume_is_never_created(self) -> None:
        missing = self.root / "some" / "path"
        config = make_config(
            missing, missing / "Media" / "Apple Photos", require_mounted_volume=False
        )
        with self.assertRaises(ArchiveUnavailable) as caught:
            with open_archive(config, probes=self.make_probes(mounted=False)):
                self.fail("archive must not open on a missing directory")
        self.assertIn("create it and retry", str(caught.exception))
        self.assertFalse(missing.exists())

    def test_symlinked_overridden_volume_fails_closed(self) -> None:
        target = self.root / "elsewhere"
        target.mkdir()
        linked = self.root / "linked"
        linked.symlink_to(target)
        config = make_config(
            linked, linked / "Media" / "Apple Photos", require_mounted_volume=False
        )
        with self.assertRaises(ArchiveUnsafe) as caught:
            resolve_archive(config, self.make_probes(mounted=False))
        self.assertIn("is a symlink", str(caught.exception))

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

    def test_a_reader_blocking_a_writer_is_not_blamed_on_a_finished_run(
        self,
    ) -> None:
        with open_archive(self.config, probes=self.probes) as archive:
            lock_file = archive.paths.lock_file
        self.assertEqual(lock_file.read_text(), "")

        with open_archive(self.config, dry_run=True, probes=self.probes):
            with self.assertRaises(ArchiveLocked) as caught:
                with open_archive(self.config, probes=self.probes):
                    self.fail("a writer must wait for the reader")
        self.assertIn("a reader such as verify", str(caught.exception))
        self.assertNotIn("pid", str(caught.exception))

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


class ProbeInjectionTests(ArchiveTestCase):
    def test_hostname_and_clock_come_from_the_probes(self) -> None:
        with open_archive(self.config, probes=self.probes) as archive:
            self.assertEqual(archive.hostname, "Sebastiens MacBook.local")
            self.assertEqual(archive.now(), FIXED_NOW)
            self.assertIn(FIXED_NOW.isoformat(), archive.paths.lock_file.read_text())


class ExclusiveLockTests(unittest.TestCase):
    def setUp(self) -> None:
        self.root = Path(self.enterContext(tempfile.TemporaryDirectory())).resolve()
        self.paths = ArchivePaths(self.root, self.root / "photos")
        self.paths.metadata.mkdir(parents=True)
        self.probes = SystemProbes(hostname=lambda: "test.local")

    def test_a_symlinked_lock_is_refused_and_its_target_left_alone(self) -> None:
        target = self.root / "precious.txt"
        target.write_text("keep")
        self.paths.lock_file.symlink_to(target)

        with self.assertRaises(ArchiveUnavailable):
            with archive_lock(self.paths, self.probes):
                self.fail("lock must not be acquired")

        self.assertEqual(target.read_text(), "keep")

    def test_a_failed_owner_note_does_not_fail_the_run(self) -> None:
        entered = False
        with mock.patch(
            "photos_backup.archive.lock.os.write", side_effect=OSError(28, "full")
        ):
            with archive_lock(self.paths, self.probes):
                entered = True

        self.assertTrue(entered)


if __name__ == "__main__":
    unittest.main()
