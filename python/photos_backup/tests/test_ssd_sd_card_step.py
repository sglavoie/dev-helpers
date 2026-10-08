from __future__ import annotations

import unittest
from pathlib import Path
from unittest import mock

from photos_backup.config import SdCardConfig, SsdConfig
from photos_backup.errors import ActionRequired
from photos_backup.ssd.backup import Backup as SsdBackup
from tests.transfer_case import RCLONE_OK, RSYNC_OK, TransferTestCase


class SsdSdCardNotCreatedTests(TransferTestCase):
    def setUp(self) -> None:
        super().setUp()
        self.add_photo(self.source)
        self.card_copy = self.root / "SD Card Copies"
        self.config.write_text(
            f'[ssd]\nsource = "{self.source}"\ndestination = "{self.destination}"\n'
            f'[sd_card]\nsource = "{self.root / "card"}"\n'
            f'destination = "{self.card_copy}"\n'
            f'[rclone]\nsource = "{self.destination}"\nremote = "b2:photos"\n'
        )

    def test_ssd_copies_the_archive_and_skips_the_missing_card_copy(self):
        for delete in (False, True):
            with (
                self.subTest(delete=delete),
                mock.patch(
                    "photos_backup.ssd.backup.stream_command", return_value=RSYNC_OK
                ) as rsync,
            ):
                result = self.invoke("ssd", *(["--delete"] if delete else []))
            self.assertEqual(result.exit_code, 0, result.output)
            rsync.assert_called_once()
            self.assertEqual(rsync.call_args.args[0][-2], str(self.source))
            self.assertIn("not created yet", result.output)
            self.assertIn("sd-card", result.output)

    def test_backup_all_still_reaches_the_remote_step(self):
        with (
            mock.patch(
                "photos_backup.ssd.backup.stream_command", return_value=RSYNC_OK
            ),
            mock.patch(
                "photos_backup.remote.backup.stream_command", return_value=RCLONE_OK
            ) as rclone,
        ):
            result = self.invoke("backup-all", "--skip-apple-photos", "--skip-sd-card")
        self.assertEqual(result.exit_code, 0, result.output)
        rclone.assert_called_once()
        self.assertIn("not created yet", result.output)

    def test_summaries_keep_step_order(self):
        with (
            mock.patch(
                "photos_backup.ssd.backup.stream_command", return_value=RSYNC_OK
            ),
            mock.patch("photos_backup.space.click.echo"),
        ):
            summaries = SsdBackup(
                SsdConfig(self.source, self.destination, None),
                False,
                False,
                SdCardConfig(self.root / "card", self.card_copy, None),
            ).backup()
        self.assertEqual(
            [(s.step_name, s.skipped) for s in summaries],
            [("SSD: All Photos", False), ("SSD: SD Card", True)],
        )

    def test_unmounted_card_copy_volume_still_requires_action(self):
        backup = SsdBackup(
            SsdConfig(self.source, self.destination, None),
            False,
            False,
            SdCardConfig(self.root / "card", Path("/Volumes/Offline/SD"), None),
        )
        with (
            mock.patch.object(Path, "is_mount", return_value=False),
            mock.patch("photos_backup.ssd.backup.stream_command") as rsync,
        ):
            with self.assertRaisesRegex(ActionRequired, "not a mounted drive"):
                backup.backup()
        rsync.assert_not_called()


if __name__ == "__main__":
    unittest.main()
