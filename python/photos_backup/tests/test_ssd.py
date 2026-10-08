from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.config import SdCardConfig, SsdConfig
from photos_backup.errors import ActionRequired
from photos_backup.ssd.backup import Backup as SsdBackup
from tests import isolate_transfer_history


class SsdSafetyTests(unittest.TestCase):
    def setUp(self):
        isolate_transfer_history(self)
        self.enterContext(
            mock.patch(
                "photos_backup.cli.backup_all.which", return_value="/test/bin/tool"
            )
        )

    def test_copy_failures_preserve_completed_results_and_stop_remaining_copies(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ("photos", "sd"):
                (root / name).mkdir()
            config = root / "config.toml"
            config.write_text(
                f'[ssd]\nsource = "{root / "photos"}"\ndestination = "{root / "backup"}"\n'
                f'[sd_card]\nsource = "{root / "card"}"\ndestination = "{root / "sd"}"\n'
            )
            success = subprocess.CompletedProcess(
                [], 0, stdout="Number of regular files transferred: 3\n"
            )
            for command, dry_run in (
                ("ssd", False),
                ("ssd", True),
                ("backup-all", False),
                ("backup-all", True),
            ):
                for failed_copy in (0, 1):
                    for failure, exit_code in (
                        (subprocess.CalledProcessError(23, ["rsync"]), 1),
                        (ActionRequired("Drive disconnected"), 3),
                    ):
                        with (
                            self.subTest(
                                command=command,
                                dry_run=dry_run,
                                failed_copy=failed_copy,
                                code=exit_code,
                            ),
                            mock.patch(
                                "photos_backup.ssd.backup.stream_command",
                                side_effect=[success] * failed_copy + [failure],
                            ) as run,
                        ):
                            args = ["--config", str(config), command]
                            if dry_run:
                                args.append("--dry-run")
                            if command == "backup-all":
                                args += ["--skip-apple-photos", "--skip-sd-card"]
                            result = CliRunner().invoke(cli, args)
                        self.assertEqual(result.exit_code, exit_code, result.output)
                        self.assertEqual(run.call_count, failed_copy + 1)
                        self.assertIn("SSD: All Photos", result.output)
                        self.assertIn("SSD: SD Card", result.output)
                        if failed_copy:
                            if command == "ssd":
                                expected = (
                                    "Proposed transfers: 3"
                                    if dry_run
                                    else "Files transferred: 3"
                                )
                            else:
                                expected = (
                                    "3 proposed transfers" if dry_run else "3 files"
                                )
                            self.assertIn(expected, result.output)
                            self.assertNotIn("Previous SSD copy", result.output)
                        else:
                            self.assertIn(
                                "Previous SSD copy did not complete", result.output
                            )
                        if dry_run:
                            self.assertFalse((root / "backup").exists())
                        else:
                            (root / "backup").rmdir()

    def test_missing_sd_archive_stops_before_any_copy(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            destination = root / "backup"
            backup = SsdBackup(
                SsdConfig(root, destination, None),
                False,
                False,
                sd_card=SdCardConfig(root, root / "missing-sd-archive", None),
            )
            with mock.patch("photos_backup.ssd.backup.stream_command") as run:
                with self.assertRaises(ActionRequired):
                    backup.backup()
            run.assert_not_called()
            self.assertFalse(destination.exists())

    def test_missing_volume_never_creates_destination_or_runs_rsync(self):
        for dry_run in (False, True):
            with (
                self.subTest(dry_run=dry_run),
                tempfile.TemporaryDirectory() as source,
                mock.patch.object(Path, "is_mount", return_value=False),
                mock.patch.object(Path, "mkdir") as mkdir,
                mock.patch("photos_backup.ssd.backup.stream_command") as run,
            ):
                backup = SsdBackup(
                    SsdConfig(Path(source), Path("/Volumes/Offline/Pictures"), None),
                    False,
                    dry_run,
                )
                with self.assertRaisesRegex(ActionRequired, "mounted drive"):
                    backup.backup()
                mkdir.assert_not_called()
                run.assert_not_called()

    def test_mounted_volume_allows_preview_without_creating_directories(self):
        with (
            mock.patch.object(Path, "is_mount", return_value=True),
            mock.patch.object(Path, "is_dir", return_value=True),
            mock.patch.object(Path, "is_symlink", return_value=False),
            mock.patch.object(Path, "mkdir") as mkdir,
            mock.patch(
                "photos_backup.ssd.backup.stream_command",
                return_value=subprocess.CompletedProcess([], 0, stdout=""),
            ) as run,
        ):
            SsdBackup(
                SsdConfig(Path("/Volumes/A/Photos"), Path("/Volumes/B/Backup"), None),
                False,
                True,
            ).backup()
        run.assert_called_once()
        mkdir.assert_not_called()

    def test_missing_source_is_action_required_in_standalone_and_pipeline(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / "config.toml"
            destination = root / "destination"
            config.write_text(
                f'[ssd]\nsource = "{root / "missing"}"\ndestination = "{destination}"\n'
            )
            for command in ("ssd", "backup-all"):
                with self.subTest(command=command):
                    args = ["--config", str(config), command]
                    if command == "backup-all":
                        args.append("--skip-apple-photos")
                    result = CliRunner().invoke(cli, args)
                    self.assertEqual(result.exit_code, 3, result.output)
                    self.assertIn("SSD source", result.output)
                    self.assertFalse(destination.exists())
                    if command == "backup-all":
                        self.assertIn("BACKUP PIPELINE SUMMARY", result.output)
                        self.assertIn("ACTION REQUIRED", result.output)
                        self.assertNotIn("ALL OK", result.output)


if __name__ == "__main__":
    unittest.main()
