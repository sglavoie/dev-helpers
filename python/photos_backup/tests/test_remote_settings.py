from __future__ import annotations

import unittest
from unittest import mock

from photos_backup.config import RcloneConfig, load_rclone_config
from photos_backup.remote.backup import Backup as RemoteBackup
from tests.transfer_case import RCLONE_OK, TransferTestCase


class RemoteSettingsTests(TransferTestCase):
    def test_remote_must_name_an_rclone_remote(self):
        for remote in ("my-photos-bucket", "/Volumes/Backup/photos", "/tmp/a:b"):
            with self.subTest(remote=remote):
                self.config.write_text(f'[rclone]\nremote = "{remote}"\n')
                result = self.invoke("remote")
                self.assertEqual(result.exit_code, 2, result.output)
                self.assertIn("must name an rclone remote", result.output)
                self.assertIn("rclone listremotes", result.output)

    def test_remote_names_and_on_the_fly_backends_are_accepted(self):
        for remote in ("b2:bucket", "gdrive:", ":s3,provider=AWS:bucket/dir"):
            with self.subTest(remote=remote):
                self.config.write_text(f'[rclone]\nremote = "{remote}"\n')
                config = load_rclone_config(self.config)
                self.assertEqual(config.remote, remote)

    def test_uploads_skip_finder_clutter(self):
        self.add_photo(self.source)
        for delete in (False, True):
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
            self.assertIn("--exclude .DS_Store --exclude ._*", " ".join(argv))


if __name__ == "__main__":
    unittest.main()
