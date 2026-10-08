import tempfile
import unittest
from pathlib import Path
from unittest import mock

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


if __name__ == "__main__":
    unittest.main()
