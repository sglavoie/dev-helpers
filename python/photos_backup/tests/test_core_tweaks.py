import contextlib
import io
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.adapter import run_osxphotos_export
from photos_backup.apple_photos.downloads import DownloadBudget
from photos_backup.apple_photos.files import prune_emptied_parents
from photos_backup.archive import ArchivePaths, SystemProbes
from photos_backup.archive.errors import ArchiveUnavailable
from photos_backup.archive.lock import archive_lock


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


class DownloadFailureRecordTests(unittest.TestCase):
    def setUp(self) -> None:
        self.root = Path(self.enterContext(tempfile.TemporaryDirectory()))
        self.missing = self.root / "gone" / "export.csv"

    def test_an_unwritable_failure_record_is_a_warning(self) -> None:
        budget = DownloadBudget(120)
        budget.failures["asset-1"] = {
            "uuid": "asset-1",
            "filename": "photo.jpg",
            "reason": "timed out",
        }
        stderr = io.StringIO()

        with contextlib.redirect_stderr(stderr):
            budget.write_failures(self.missing)

        self.assertIn("Warning: could not record 1 incomplete", stderr.getvalue())

    def test_an_unwritable_failure_record_does_not_mask_the_export_error(
        self,
    ) -> None:
        def failing_export(**_: object) -> int:
            raise RuntimeError("osxphotos crashed")

        def record_failure(budget: DownloadBudget) -> None:
            budget.failures["asset-1"] = {
                "uuid": "asset-1",
                "filename": "photo.jpg",
                "reason": "timed out",
            }

        original = DownloadBudget.__init__

        def init(budget: DownloadBudget, *args: object, **kwargs: object) -> None:
            original(budget, *args, **kwargs)  # type: ignore[arg-type]
            record_failure(budget)

        with (
            mock.patch.object(DownloadBudget, "__init__", init),
            mock.patch("photos_backup.apple_photos.adapter.export_cli", failing_export),
            contextlib.redirect_stderr(io.StringIO()),
            self.assertRaisesRegex(RuntimeError, "osxphotos crashed"),
        ):
            run_osxphotos_export({"report": str(self.missing)}, download_timeout=1)


class PruneEmptiedParentsTests(unittest.TestCase):
    def setUp(self) -> None:
        self.root = Path(self.enterContext(tempfile.TemporaryDirectory())) / "archive"

    def test_the_root_and_kept_subtrees_survive_even_when_emptied(self) -> None:
        kept = self.root / ".photos-backup" / "reports"
        kept.mkdir(parents=True)
        deleted = (self.root / "a.jpg", kept / "old.csv", self.root / "2019/07/b.jpg")
        (self.root / "2019" / "07").mkdir(parents=True)

        removed = prune_emptied_parents(deleted, self.root, keep=(kept.parent,))

        self.assertEqual(removed, (self.root / "2019/07", self.root / "2019"))
        self.assertTrue(kept.is_dir())
        self.assertTrue(self.root.is_dir())

    def test_a_symlinked_parent_is_never_followed(self) -> None:
        target = self.root.parent / "elsewhere"
        target.mkdir()
        self.root.mkdir()
        (self.root / "link").symlink_to(target)

        prune_emptied_parents((self.root / "link" / "a.jpg",), self.root)

        self.assertTrue((self.root / "link").is_symlink())
        self.assertTrue(target.is_dir())


if __name__ == "__main__":
    unittest.main()
