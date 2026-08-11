from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from photos_backup.apple_photos.local_export import (
    NOT_CONFIGURED,
    confirmation_matches,
    contents_refusal,
    delete_local_export,
    plan_local_export_cleanup,
    scan_local_export,
    target_refusal,
)
from photos_backup.archive.errors import ArchiveUnsafe
from tests.test_export import make_config

HOME = Path("/Users/tester")
VOLUME = Path("/Volumes/SanDisk")
ARCHIVE = VOLUME / "Media" / "Apple Photos"
LIBRARY = HOME / "Pictures" / "Photos Library.photoslibrary"
EXPORT = HOME / "Pictures" / "export"


def refusal(target: Path) -> str | None:
    return target_refusal(
        target, archive=ARCHIVE, volume=VOLUME, library=LIBRARY, home=HOME
    )


class TargetTests(unittest.TestCase):
    def test_the_configured_export_is_accepted(self) -> None:
        self.assertIsNone(refusal(EXPORT))

    def test_the_filesystem_root_is_refused(self) -> None:
        self.assertIn("filesystem root", str(refusal(Path("/"))))

    def test_the_home_directory_is_refused(self) -> None:
        self.assertIn("home directory", str(refusal(HOME)))

    def test_a_parent_of_the_home_directory_is_refused(self) -> None:
        self.assertIn("'/Users' contains the", str(refusal(Path("/Users"))))

    def test_a_parent_of_the_photos_library_is_refused(self) -> None:
        self.assertIn("contains the Photos library", str(refusal(HOME / "Pictures")))

    def test_the_archive_itself_is_refused(self) -> None:
        self.assertIn("is the archive", str(refusal(ARCHIVE)))

    def test_a_directory_inside_the_archive_is_refused(self) -> None:
        self.assertIn("is the archive", str(refusal(ARCHIVE / "2026")))

    def test_a_directory_on_the_archive_volume_is_refused(self) -> None:
        self.assertIn("archive volume", str(refusal(VOLUME / "Scratch")))

    def test_a_directory_inside_a_photos_library_is_refused(self) -> None:
        self.assertIn("inside a Photos library", str(refusal(LIBRARY / "originals")))


class ScanTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        self.root = Path(self._tmpdir.name).resolve()

    def write(self, relative: str, content: str = "photo") -> Path:
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        return path

    def test_an_export_tree_is_entirely_recognized(self) -> None:
        self.write("2022/03/2022-03-01-101010_IMG_1.JPG")
        self.write("2022/03/2022-03-01-101010_IMG_1.JPG.json", "{}")
        self.write("2026/08/clip.mov")
        self.write(".DS_Store", "")
        self.write(".osxphotos_export.db", "sqlite")
        self.write(".osxphotos_export.db-wal", "wal")

        scan = scan_local_export(self.root)

        self.assertEqual(scan.file_count, 6)
        self.assertEqual(scan.unrecognized, ())
        self.assertIsNone(contents_refusal(scan))

    def test_total_size_is_the_sum_of_every_file(self) -> None:
        self.write("2022/03/a.JPG", "12345")
        self.write("2022/03/b.JPG", "678")

        self.assertEqual(scan_local_export(self.root).total_bytes, 8)

    def test_a_file_no_export_writes_is_unrecognized(self) -> None:
        self.write("2022/03/taxes.pdf")

        reason = contents_refusal(scan_local_export(self.root))

        self.assertIn("not something an Apple Photos export writes", str(reason))
        self.assertIn("taxes.pdf", str(reason))

    def test_a_symlink_is_refused_rather_than_followed(self) -> None:
        self.write("2022/03/a.JPG")
        (self.root / "elsewhere.JPG").symlink_to(Path("/etc/hosts"))

        scan = scan_local_export(self.root)

        self.assertEqual(len(scan.symlinks), 1)
        self.assertIn("symlinks", str(contents_refusal(scan)))

    def test_a_photos_library_inside_the_export_is_refused(self) -> None:
        (self.root / "Old Photos.photoslibrary").mkdir()

        scan = scan_local_export(self.root)

        self.assertEqual(len(scan.libraries), 1)
        self.assertIn("Photos library", str(contents_refusal(scan)))


class PlanTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        self.root = Path(self._tmpdir.name).resolve()
        self.export = self.root / "export"
        self.export.mkdir()

    def config(self, **overrides):
        overrides.setdefault("legacy_export", self.export)
        return make_config(
            self.root / "SanDisk",
            self.root / "SanDisk" / "Media" / "Apple Photos",
            **overrides,
        )

    def write(self, relative: str, content: str = "photo") -> Path:
        path = self.export / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        return path

    def test_an_unconfigured_legacy_export_leaves_nothing_to_delete(self) -> None:
        plan = plan_local_export_cleanup(
            self.config(legacy_export=None), home=self.root
        )

        self.assertEqual(plan.refusal, NOT_CONFIGURED)
        self.assertIsNone(plan.target)

    def test_a_missing_directory_is_refused_rather_than_deleted(self) -> None:
        config = self.config(legacy_export=self.root / "gone")

        plan = plan_local_export_cleanup(config, home=self.root)

        self.assertIn("not a real directory", str(plan.refusal))

    def test_a_clean_export_is_deletable_with_its_count_and_size(self) -> None:
        self.write("2022/03/a.JPG", "12345")

        plan = plan_local_export_cleanup(self.config(), home=self.root)

        self.assertTrue(plan.deletable)
        self.assertEqual(plan.scan.file_count, 1)
        self.assertEqual(plan.scan.total_bytes, 5)

    def test_deletion_empties_the_export_but_keeps_the_directory(self) -> None:
        self.write("2022/03/a.JPG", "12345")
        self.write(".DS_Store", "")

        cleanup = delete_local_export(
            plan_local_export_cleanup(self.config(), home=self.root)
        )

        self.assertEqual(len(cleanup.deleted), 2)
        self.assertEqual(cleanup.freed_bytes, 5)
        self.assertTrue(self.export.is_dir())
        self.assertEqual(list(self.export.iterdir()), [])
        self.assertIn(self.export / "2022", cleanup.pruned)

    def test_a_refused_plan_cannot_be_deleted_even_when_asked_directly(self) -> None:
        self.write("2022/03/taxes.pdf")
        plan = plan_local_export_cleanup(self.config(), home=self.root)

        with self.assertRaises(ArchiveUnsafe):
            delete_local_export(plan)

        self.assertTrue((self.export / "2022/03/taxes.pdf").exists())


class ConfirmationTests(unittest.TestCase):
    def test_the_exact_directory_confirms(self) -> None:
        self.assertTrue(confirmation_matches(f"  {EXPORT}  ", EXPORT))

    def test_anything_else_cancels(self) -> None:
        self.assertFalse(confirmation_matches("yes", EXPORT))
        self.assertFalse(confirmation_matches("", EXPORT))
        self.assertFalse(confirmation_matches(str(EXPORT / "2022"), EXPORT))


if __name__ == "__main__":
    unittest.main()
