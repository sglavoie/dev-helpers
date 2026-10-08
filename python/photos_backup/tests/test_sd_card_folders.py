from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.config import SdCardConfig
from photos_backup.sd_card.backup import Backup
from photos_backup.sd_card.folders import uncovered_camera_folders
from tests import isolate_transfer_history


class CameraFolderTests(unittest.TestCase):
    def setUp(self) -> None:
        isolate_transfer_history(self)
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.dcim = self.root / "DCIM"
        self.source = self.dcim / "100MSDCF"
        self.source.mkdir(parents=True)

    def test_only_sibling_dcf_directories_are_reported(self):
        (self.dcim / "102MSDCF").mkdir()
        (self.dcim / "101MSDCF").mkdir()
        (self.dcim / "099MSDCF").mkdir()
        (self.dcim / "MISC").mkdir()
        (self.dcim / "103MSDCF").write_text("not a folder")

        self.assertEqual(
            [folder.name for folder in uncovered_camera_folders(self.source)],
            ["101MSDCF", "102MSDCF"],
        )

    def test_a_dcim_root_or_missing_card_reports_nothing(self):
        (self.dcim / "101MSDCF").mkdir()

        self.assertEqual(uncovered_camera_folders(self.dcim), [])
        self.assertEqual(uncovered_camera_folders(self.root / "gone" / "100ABCDE"), [])

    def test_copy_warns_and_still_copies_the_configured_folder(self):
        (self.dcim / "101MSDCF").mkdir()
        with (
            mock.patch("photos_backup.sd_card.backup.click.echo") as echo,
            mock.patch(
                "photos_backup.sd_card.backup.stream_command",
                return_value=subprocess.CompletedProcess([], 0, stdout=""),
            ) as run,
        ):
            result = Backup(
                SdCardConfig(self.source, self.root / "copy", None), True
            ).backup()

        self.assertIsNone(result.error)
        self.assertIn(str(self.source), run.call_args.args[0])
        (warning,) = [
            call for call in echo.call_args_list if "101MSDCF" in call.args[0]
        ]
        self.assertIn(f"source to '{self.dcim}'", warning.args[0])
        self.assertTrue(warning.kwargs["err"])

    def test_doctor_asks_for_action_on_uncovered_folders(self):
        (self.dcim / "101MSDCF").mkdir()
        config = self.root / "config.toml"
        config.write_text(
            f'[sd_card]\nsource = "{self.source}"\ndestination = "{self.root}/copy"\n'
        )
        with mock.patch("photos_backup.cli.doctor._tool_version", return_value="test"):
            result = CliRunner().invoke(cli, ["--config", str(config), "doctor"])

        self.assertEqual(result.exit_code, 3, result.output)
        self.assertIn(
            "ACTION SD Card camera folders: SD card folder(s) 101MSDCF", result.output
        )
