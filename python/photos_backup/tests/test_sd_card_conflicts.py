from __future__ import annotations

import os
import shutil
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.config import SdCardConfig
from photos_backup.sd_card.backup import Backup
from photos_backup.sd_card.conflicts import conflicting_files
from tests.transfer_case import RSYNC_OK, TransferTestCase


class ConflictingFilesTests(TransferTestCase):
    def setUp(self) -> None:
        super().setUp()
        self.card = self.root / "card" / "100MSDCF"
        self.card.mkdir(parents=True)
        self.target = self.destination / "100MSDCF"
        self.target.mkdir(parents=True)

    def pair(self, name: str, card: str, archived: str, *, offset: float = 0) -> None:
        (self.card / name).write_text(card)
        (self.target / name).write_text(archived)
        stat = (self.card / name).stat()
        os.utime(self.target / name, (stat.st_atime, stat.st_mtime + offset))

    def test_only_different_files_with_an_archived_name_conflict(self):
        self.pair("DSC00001.ARW", "same", "same", offset=1)
        self.pair("DSC00002.ARW", "new photo", "older photo")
        self.pair("DSC00003.ARW", "same", "same", offset=3600)
        (self.card / "DSC00004.ARW").write_text("not archived yet")

        self.assertEqual(
            conflicting_files(self.card, self.target),
            [Path("DSC00002.ARW"), Path("DSC00003.ARW")],
        )

    def test_nested_files_conflict_and_dot_files_are_ignored(self):
        (self.card / "CLIP").mkdir()
        (self.target / "CLIP").mkdir()
        self.pair("CLIP/C0001.MP4", "new clip", "old")
        self.pair(".DS_Store", "finder", "other finder")

        self.assertEqual(
            conflicting_files(self.card, self.target), [Path("CLIP/C0001.MP4")]
        )

    def test_a_first_copy_has_no_conflicts(self):
        (self.card / "DSC00001.ARW").write_text("photo")

        self.assertEqual(conflicting_files(self.card, self.root / "missing"), [])

    def test_a_real_copy_still_runs_then_needs_a_person(self):
        self.pair("DSC00001.ARW", "new photo", "older photo")
        with mock.patch(
            "photos_backup.sd_card.backup.stream_command", return_value=RSYNC_OK
        ) as run:
            summary = Backup(
                SdCardConfig(self.card, self.destination, None), False
            ).backup()

        run.assert_called_once()
        self.assertTrue(summary.action_required)
        self.assertIn("1 SD card file(s) were not copied", summary.error or "")
        self.assertIn("DSC00001.ARW", summary.error or "")

    def test_a_preview_warns_without_needing_a_person(self):
        self.pair("DSC00001.ARW", "new photo", "older photo")
        with (
            mock.patch(
                "photos_backup.sd_card.backup.stream_command", return_value=RSYNC_OK
            ),
            mock.patch("photos_backup.sd_card.backup.click.echo") as echo,
        ):
            summary = Backup(
                SdCardConfig(self.card, self.destination, None), True
            ).backup()

        self.assertIsNone(summary.error)
        warnings = [call.args[0] for call in echo.call_args_list]
        self.assertTrue(any("would not be copied" in line for line in warnings))

    def test_many_conflicts_are_counted_but_listed_briefly(self):
        for number in range(1, 8):
            self.pair(f"DSC0000{number}.ARW", "new", "older")
        with mock.patch(
            "photos_backup.sd_card.backup.stream_command", return_value=RSYNC_OK
        ):
            summary = Backup(
                SdCardConfig(self.card, self.destination, None), False
            ).backup()

        self.assertIn("7 SD card file(s)", summary.error or "")
        self.assertIn("and 2 more", summary.error or "")
        self.assertNotIn("DSC00006.ARW", summary.error or "")

    def test_sd_card_command_exits_three(self):
        self.pair("DSC00001.ARW", "new photo", "older photo")
        self.config.write_text(
            f'[sd_card]\nsource = "{self.card}"\ndestination = "{self.destination}"\n'
        )
        with mock.patch(
            "photos_backup.sd_card.backup.stream_command", return_value=RSYNC_OK
        ):
            result = self.invoke("sd-card")

        self.assertEqual(result.exit_code, 3, result.output)
        self.assertIn("camera file counter reset", result.output)


@unittest.skipIf(shutil.which("rsync") is None, "rsync is not installed")
class RealRsyncConflictTests(TransferTestCase):
    def setUp(self) -> None:
        super().setUp()
        self.enterContext(mock.patch("photos_backup.process.click.echo"))
        self.enterContext(mock.patch("photos_backup.space.click.echo"))

    def test_a_second_unchanged_copy_reports_no_conflicts(self):
        card = self.root / "card" / "100MSDCF"
        self.add_photo(card)
        config = SdCardConfig(card, self.destination, None)

        first = Backup(config, False).backup()
        second = Backup(config, False).backup()

        self.assertEqual([first.error, second.error], [None, None])


if __name__ == "__main__":
    unittest.main()
