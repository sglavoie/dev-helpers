from __future__ import annotations

import shutil
import unittest
from unittest import mock

from photos_backup.config import SdCardConfig, SsdConfig
from photos_backup.sd_card.backup import Backup as SdCardBackup
from photos_backup.ssd.backup import Backup as SsdBackup
from tests.transfer_case import TransferTestCase


@unittest.skipIf(shutil.which("rsync") is None, "rsync is not installed")
class RealRsyncLayoutTests(TransferTestCase):
    """Run the rsync found on PATH, which may be GNU rsync or macOS openrsync."""

    def setUp(self) -> None:
        super().setUp()
        self.enterContext(mock.patch("photos_backup.process.click.echo"))
        self.enterContext(mock.patch("photos_backup.space.click.echo"))

    def test_sd_card_copy_lands_under_source_name_and_keeps_existing_files(self):
        card = self.root / "card" / "DCIM"
        self.add_photo(card / "100MSDCF")
        (card / "100MSDCF" / "DSC00002.ARW").write_text("second")
        archived = self.destination / "DCIM" / "100MSDCF" / "DSC00001.ARW"
        archived.parent.mkdir(parents=True)
        archived.write_text("original from before the counter reset")

        summary = SdCardBackup(
            SdCardConfig(card, self.destination, None), False
        ).backup()

        self.assertTrue(summary.action_required)
        self.assertIn("100MSDCF/DSC00001.ARW", summary.error or "")
        self.assertEqual(archived.read_text(), "original from before the counter reset")
        self.assertEqual((archived.parent / "DSC00002.ARW").read_text(), "second")
        self.assertEqual(sorted(p.name for p in self.destination.iterdir()), ["DCIM"])

    def test_ssd_copy_lands_under_each_source_name(self):
        self.add_photo(self.source)
        card_copy = self.root / "card-copies"
        self.add_photo(card_copy / "DCIM")

        summaries = SsdBackup(
            SsdConfig(self.source, self.destination, None),
            True,
            False,
            SdCardConfig(self.root / "card", card_copy, None),
        ).backup()

        self.assertEqual([s.error for s in summaries], [None, None])
        self.assertTrue((self.destination / "photos" / "DSC00001.ARW").is_file())
        self.assertTrue(
            (self.destination / "card-copies" / "DCIM" / "DSC00001.ARW").is_file()
        )

    def test_ssd_mirror_deletes_only_stale_files_under_the_source_name(self):
        self.add_photo(self.source)
        stale = self.destination / "photos" / "stale.ARW"
        unrelated = self.destination / "unrelated" / "keep.ARW"
        for path in (stale, unrelated):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("old")

        summaries = SsdBackup(
            SsdConfig(self.source, self.destination, None), True, False
        ).backup()

        self.assertIsNone(summaries[0].error)
        self.assertFalse(stale.exists())
        self.assertTrue(unrelated.is_file())
        self.assertTrue((self.destination / "photos" / "DSC00001.ARW").is_file())

    def test_ssd_mirror_stops_at_max_delete_with_a_hint(self):
        self.add_photo(self.source)
        stale = self.destination / "photos"
        stale.mkdir(parents=True)
        for index in range(3):
            (stale / f"stale{index}.ARW").write_text("old")

        summaries = SsdBackup(
            SsdConfig(self.source, self.destination, None, max_delete=1),
            True,
            False,
        ).backup()

        self.assertIn("raise [ssd] max_delete", summaries[0].error or "")
        self.assertGreaterEqual(len(list(stale.glob("stale*.ARW"))), 2)


if __name__ == "__main__":
    unittest.main()
