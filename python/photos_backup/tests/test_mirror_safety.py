from __future__ import annotations

import subprocess
import unittest
from unittest import mock

from photos_backup.config import DEFAULT_RCLONE_MAX_DELETE, RcloneConfig
from photos_backup.copy_safety import check_mirror_source
from photos_backup.errors import ActionRequired
from photos_backup.remote.backup import Backup as RemoteBackup
from tests.transfer_case import RCLONE_OK, RSYNC_OK, TransferTestCase


class MirrorEmptySourceTests(TransferTestCase):
    def setUp(self) -> None:
        super().setUp()
        self.config.write_text(
            f'[ssd]\nsource = "{self.source}"\ndestination = "{self.destination}"\n'
            f'[rclone]\nsource = "{self.source}"\nremote = "b2:photos"\n'
        )

    def test_finder_clutter_alone_counts_as_empty(self):
        self.add_finder_clutter(self.source)
        with self.assertRaisesRegex(ActionRequired, "refusing to mirror deletions"):
            check_mirror_source(self.source, workflow="SSD")
        (self.source / "DCIM").mkdir()
        check_mirror_source(self.source, workflow="SSD")

    def test_ssd_and_remote_deletions_refuse_an_empty_source(self):
        self.add_finder_clutter(self.source)
        for args in (
            ["ssd", "--delete"],
            ["remote", "--delete"],
            ["backup-all", "--skip-apple-photos", "--skip-remote", "--delete-ssd"],
            ["backup-all", "--skip-apple-photos", "--skip-ssd", "--delete-remote"],
        ):
            for dry_run in (False, True):
                with (
                    self.subTest(args=args, dry_run=dry_run),
                    mock.patch("photos_backup.ssd.backup.stream_command") as rsync,
                    mock.patch("photos_backup.remote.backup.stream_command") as rclone,
                ):
                    result = self.invoke(*args, *(["--dry-run"] if dry_run else []))
                self.assertEqual(result.exit_code, 3, result.output)
                self.assertIn("is empty; refusing to mirror deletions", result.output)
                rsync.assert_not_called()
                rclone.assert_not_called()
                self.assertFalse(self.destination.exists())

    def test_copies_without_deletions_still_run_from_an_empty_source(self):
        with (
            mock.patch(
                "photos_backup.ssd.backup.stream_command", return_value=RSYNC_OK
            ) as rsync,
            mock.patch(
                "photos_backup.remote.backup.stream_command", return_value=RCLONE_OK
            ) as rclone,
        ):
            self.assertEqual(self.invoke("ssd").exit_code, 0)
            self.assertEqual(self.invoke("remote").exit_code, 0)
        rsync.assert_called_once()
        rclone.assert_called_once()

    def test_remote_sync_caps_deletions(self):
        self.add_photo(self.source)
        for delete, max_delete in ((False, None), (True, DEFAULT_RCLONE_MAX_DELETE)):
            with (
                self.subTest(delete=delete),
                mock.patch(
                    "photos_backup.remote.backup.stream_command",
                    return_value=RCLONE_OK,
                ) as rclone,
            ):
                RemoteBackup(
                    RcloneConfig("b2:photos", None),
                    self.source,
                    False,
                    delete_at_destination=delete,
                ).backup()
            argv = rclone.call_args.args[0]
            self.assertEqual(argv[1], "sync" if delete else "copy")
            if max_delete is None:
                self.assertNotIn("--max-delete", argv)
            else:
                index = argv.index("--max-delete")
                self.assertEqual(argv[index + 1], str(max_delete))

    def test_configured_max_delete_reaches_rclone_and_explains_a_stop(self):
        self.add_photo(self.source)
        self.config.write_text(
            f'[rclone]\nsource = "{self.source}"\nremote = "b2:photos"\n'
            "max_delete = 25\n"
        )
        stopped = subprocess.CompletedProcess(
            [],
            7,
            stdout="ERROR : Fatal error received - --max-delete threshold reached\n",
        )
        with mock.patch(
            "photos_backup.remote.backup.stream_command", return_value=stopped
        ) as rclone:
            result = self.invoke("remote", "--delete")
        self.assertEqual(result.exit_code, 1, result.output)
        argv = rclone.call_args.args[0]
        self.assertEqual(argv[argv.index("--max-delete") + 1], "25")
        self.assertIn("Remote deletions stop after 25 files", result.output)

    def test_max_delete_must_be_a_non_negative_integer(self):
        for value in ("-1", '"10"', "true"):
            with self.subTest(value=value):
                self.config.write_text(
                    f'[rclone]\nremote = "b2:x"\nmax_delete = {value}\n'
                )
                result = self.invoke("remote")
                self.assertEqual(result.exit_code, 2, result.output)
                self.assertIn("max_delete", result.output)


if __name__ == "__main__":
    unittest.main()
