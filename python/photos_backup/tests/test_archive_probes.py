from __future__ import annotations

import subprocess
import unittest
from unittest.mock import patch

from photos_backup.archive.errors import ArchiveUnavailable
from photos_backup.archive.probes import real_hostname


class HostnameTests(unittest.TestCase):
    @patch("photos_backup.archive.probes.sys.platform", "darwin")
    def test_network_changes_do_not_change_the_mac_name(self) -> None:
        with (
            patch("photos_backup.archive.probes.subprocess.run") as run,
            patch("photos_backup.archive.probes.socket.gethostname") as network_name,
        ):
            run.return_value.stdout = "Sebastiens-Mac-Studio\n"
            for hostname in (
                "Sebastiens-Mac-Studio.local",
                "sbastiens-mac-studio.tailb5cfdf.ts.net",
                "dhcp-192-168-1-10.example.net",
            ):
                with self.subTest(hostname=hostname):
                    network_name.return_value = hostname
                    self.assertEqual(real_hostname(), "Sebastiens-Mac-Studio.local")
            network_name.assert_not_called()
            run.assert_called_with(
                ["/usr/sbin/scutil", "--get", "LocalHostName"],
                capture_output=True,
                text=True,
                check=True,
                timeout=5,
            )

    @patch("photos_backup.archive.probes.sys.platform", "darwin")
    def test_unavailable_local_name_never_falls_back_to_the_network(self) -> None:
        for error in (
            FileNotFoundError("scutil"),
            subprocess.CalledProcessError(1, "scutil"),
            subprocess.TimeoutExpired("scutil", 5),
        ):
            with (
                self.subTest(error=error),
                patch("photos_backup.archive.probes.subprocess.run", side_effect=error),
                patch(
                    "photos_backup.archive.probes.socket.gethostname"
                ) as network_name,
                self.assertRaisesRegex(ArchiveUnavailable, "LocalHostName"),
            ):
                real_hostname()
            network_name.assert_not_called()

    @patch("photos_backup.archive.probes.sys.platform", "darwin")
    def test_empty_local_name_is_refused(self) -> None:
        with patch("photos_backup.archive.probes.subprocess.run") as run:
            run.return_value.stdout = " \n"
            with self.assertRaisesRegex(ArchiveUnavailable, "empty LocalHostName"):
                real_hostname()

    @patch("photos_backup.archive.probes.sys.platform", "linux")
    def test_other_platforms_keep_their_hostname_probe(self) -> None:
        with (
            patch("photos_backup.archive.probes.subprocess.run") as run,
            patch(
                "photos_backup.archive.probes.socket.gethostname", return_value="tester"
            ),
        ):
            self.assertEqual(real_hostname(), "tester")
            run.assert_not_called()


if __name__ == "__main__":
    unittest.main()
