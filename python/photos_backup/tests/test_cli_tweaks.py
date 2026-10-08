import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.summary import parse_rsync_stats

# Captured from macOS /usr/bin/rsync (openrsync, protocol 29) with -ah --stats.
OPENRSYNC_STATS = """\
Number of files: 6
Number of files transferred: 4
Total file size: 1500 kB
Total transferred file size: 1500 kB
Unmatched data: 1500 kB
Matched data: 0 B
File list size: 190 B
Total sent: 1501 kB
Total received: 120 B

sent 1501k bytes  received 120 bytes  185M bytes/sec
total size is 1500k  speedup is 1.00
"""

GNU_RSYNC_STATS = """\
Number of files: 1,206 (reg: 1,204, dir: 2)
Number of created files: 6 (reg: 4, dir: 2)
Number of deleted files: 0
Number of regular files transferred: 1,204
Total file size: 1.50G bytes
Total transferred file size: 1.50G bytes
"""


class RsyncStatsTests(unittest.TestCase):
    def test_openrsync_statistics_are_parsed(self):
        self.assertEqual(
            parse_rsync_stats(OPENRSYNC_STATS),
            {"files_transferred": 4, "total_size": "1500 kB"},
        )

    def test_openrsync_without_human_readable_sizes(self):
        stats = parse_rsync_stats(
            "Number of files transferred: 0\nTotal transferred file size: 0 B\n"
        )
        self.assertEqual(stats, {"files_transferred": 0, "total_size": "0 B"})

    def test_gnu_rsync_plain_sizes(self):
        stats = parse_rsync_stats(
            "Number of regular files transferred: 2\n"
            "Total transferred file size: 1,500,006 bytes\n"
        )
        self.assertEqual(
            stats, {"files_transferred": 2, "total_size": "1,500,006 bytes"}
        )

    def test_gnu_rsync_human_readable_statistics(self):
        self.assertEqual(
            parse_rsync_stats(GNU_RSYNC_STATS),
            {"files_transferred": 1204, "total_size": "1.50G bytes"},
        )


def _completed(stdout):
    return mock.Mock(stdout=stdout)


class DoctorTests(unittest.TestCase):
    def setUp(self):
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        self.root = Path(self._tmpdir.name)
        (self.root / "ssd").mkdir()
        self.config = self.root / "config.toml"

    def doctor(self, remote, listremotes, *arguments):
        self.config.write_text(
            f'[rclone]\nsource = "{self.root}/ssd"\nremote = "{remote}"\n'
        )

        def run(command, **kwargs):
            self.assertEqual(kwargs["timeout"], 5)
            self.assertIs(kwargs["stdin"], subprocess.DEVNULL)
            if command[1] == "--version":
                return _completed("rclone v1.70.0\n")
            self.assertEqual(command[1:], ["listremotes", "--ask-password=false"])
            if isinstance(listremotes, Exception):
                raise listremotes
            return _completed(listremotes)

        with (
            mock.patch(
                "photos_backup.cli.doctor.shutil.which",
                return_value="/opt/homebrew/bin/rclone",
            ),
            mock.patch(
                "photos_backup.cli.doctor.subprocess.run", side_effect=run
            ) as runner,
        ):
            result = CliRunner().invoke(
                cli, ["--config", str(self.config), "doctor", *arguments]
            )
        return result, runner

    def test_configured_rclone_remote_passes(self):
        result, _ = self.doctor("b2:photos", "local:\nb2:\n")
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("PASS rclone remote: 'b2:' is configured", result.output)

    def test_missing_rclone_remote_needs_action(self):
        result, _ = self.doctor("b2,hard_delete=true:photos", "local:\n")
        self.assertEqual(result.exit_code, 3, result.output)
        self.assertIn(
            "ACTION rclone remote: 'b2:' is not in the rclone configuration",
            result.output,
        )

    def test_unreadable_rclone_configuration_needs_action(self):
        error = subprocess.CalledProcessError(1, ["rclone", "listremotes"])
        result, _ = self.doctor("b2:photos", error)
        self.assertEqual(result.exit_code, 3, result.output)
        self.assertIn("set RCLONE_CONFIG_PASS", result.output)

    def test_on_the_fly_and_local_remotes_need_no_configuration(self):
        for remote in (":b2:photos", "/Volumes/Backup/photos"):
            with self.subTest(remote=remote):
                result, runner = self.doctor(remote, AssertionError("not called"))
                self.assertEqual(result.exit_code, 0, result.output)
                self.assertIn("SKIP rclone remote", result.output)
                self.assertEqual(runner.call_count, 1)

    def test_missing_rclone_skips_remote_lookup(self):
        self.config.write_text('[rclone]\nremote = "b2:photos"\n')
        with (
            mock.patch("photos_backup.cli.doctor.shutil.which", return_value=None),
            mock.patch("photos_backup.cli.doctor.subprocess.run") as run,
        ):
            result = CliRunner().invoke(cli, ["--config", str(self.config), "doctor"])
        self.assertIn("SKIP rclone remote: needs rclone above", result.output)
        run.assert_not_called()

    def test_json_reports_versions_checks_and_exit_code(self):
        result, _ = self.doctor("b2:photos", "local:\n", "--json")
        self.assertEqual(result.exit_code, 3, result.output)
        document = json.loads(result.output)
        self.assertEqual(
            set(document["versions"]), {"python", "photos_backup", "click", "osxphotos"}
        )
        self.assertEqual(document["exit_code"], 3)
        self.assertIn(
            {"status": "PASS", "check": "[rclone] configuration", "detail": ""},
            document["checks"],
        )
        self.assertIn(
            "ACTION",
            {check["status"] for check in document["checks"]},
        )

    def test_openrsync_first_on_path_is_named(self):
        (self.root / "card").mkdir()
        self.config.write_text(
            f'[sd_card]\nsource = "{self.root}/card"\n'
            f'destination = "{self.root}/copy"\n'
        )
        with (
            mock.patch(
                "photos_backup.cli.doctor.shutil.which", return_value="/usr/bin/rsync"
            ),
            mock.patch(
                "photos_backup.cli.doctor.subprocess.run",
                return_value=_completed("openrsync: protocol version 29\n"),
            ),
        ):
            result = CliRunner().invoke(cli, ["--config", str(self.config), "doctor"])
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("macOS built-in openrsync, not GNU rsync", result.output)


if __name__ == "__main__":
    unittest.main()
