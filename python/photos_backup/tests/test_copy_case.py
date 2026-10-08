from __future__ import annotations

import unittest
from pathlib import Path
from unittest import mock

from photos_backup.copy_safety import check_copy_path, check_copy_paths
from photos_backup.errors import ActionRequired
from tests.transfer_case import TransferTestCase


class CaseInsensitiveVolumeTests(TransferTestCase):
    def test_lowercase_volumes_path_still_requires_a_mounted_drive(self):
        for path in (Path("/volumes/Offline/DCIM"), Path("/VOLUMES/Offline/DCIM")):
            with (
                self.subTest(path=path),
                mock.patch.object(Path, "is_mount", return_value=False),
            ):
                with self.assertRaisesRegex(ActionRequired, "not a mounted drive"):
                    check_copy_path(path, workflow="SD Card")
        with self.assertRaisesRegex(ActionRequired, "beneath a mounted drive"):
            check_copy_path(Path("/volumes"), workflow="SSD")

    def test_overlap_checks_ignore_letter_case(self):
        nested = self.source / "Backup"
        nested.mkdir()
        differently_cased = Path(str(self.root / "PHOTOS" / "backup"))
        with self.assertRaisesRegex(ActionRequired, "inside or equal to source"):
            check_copy_paths((self.source,), differently_cased, workflow="SSD")
        cards = self.root / "Cards"
        cards.mkdir()
        other = self.root / "elsewhere" / "PHOTOS"
        other.mkdir(parents=True)
        with self.assertRaisesRegex(ActionRequired, "overlapping copy targets"):
            check_copy_paths((self.source, other), cards, workflow="SSD")


if __name__ == "__main__":
    unittest.main()
