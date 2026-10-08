from __future__ import annotations

import unittest

from photos_backup.config import load_rclone_config
from tests.transfer_case import TransferTestCase


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


if __name__ == "__main__":
    unittest.main()
