from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.config import RcloneConfig, SdCardConfig, SsdConfig
from photos_backup.errors import ActionRequired
from photos_backup.remote.backup import Backup as RemoteBackup, _parse_rclone_stats
from photos_backup.sd_card.backup import Backup as SdCardBackup
from photos_backup.ssd.backup import Backup as SsdBackup


class CopyWorkflowTests(unittest.TestCase):
    def setUp(self) -> None:
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.source = self.root / "Apple Photos' originals"
        self.source.mkdir()
        self.destination = self.root / "new parent" / "Backup Photos"
        self.exclude = self.root / "exclude photos.txt"
        self.exclude.write_text("*.tmp\n")

    def test_sd_card_missing_inputs_stop_before_destination_creation(self):
        for dry_run in (False, True):
            for source, destination in (
                (self.root / "missing", self.destination),
                (Path("/Volumes/Offline/DCIM"), self.destination),
                (self.source, Path("/Volumes/Offline/Photos")),
            ):
                with (
                    self.subTest(
                        dry_run=dry_run, source=source, destination=destination
                    ),
                    mock.patch.object(Path, "is_mount", return_value=False),
                    mock.patch.object(Path, "mkdir") as mkdir,
                    mock.patch("photos_backup.sd_card.backup.stream_command") as run,
                ):
                    with self.assertRaises(ActionRequired):
                        SdCardBackup(
                            SdCardConfig(source, destination, None), dry_run
                        ).backup()
                mkdir.assert_not_called()
                run.assert_not_called()

    def test_remote_missing_or_unmounted_source_never_starts_sync(self):
        link = self.root / "linked-drive"
        link.symlink_to("/Volumes/Offline/Photos")
        for dry_run in (False, True):
            for source in (
                self.root / "missing",
                Path("/Volumes/Offline/Photos"),
                link,
            ):
                with (
                    self.subTest(dry_run=dry_run, source=source),
                    mock.patch.object(Path, "is_mount", return_value=False),
                    mock.patch(
                        "photos_backup.remote.backup.shutil.which",
                        return_value="rclone",
                    ),
                    mock.patch("photos_backup.remote.backup.stream_command") as run,
                ):
                    with self.assertRaises(ActionRequired):
                        RemoteBackup(
                            RcloneConfig("b2:photos", source), source, dry_run
                        ).backup()
                run.assert_not_called()

    def test_missing_source_returns_action_required_for_sd_card_and_remote(self):
        for workflow in ("sd-card", "remote"):
            config = self.root / "config.toml"
            section = (
                f'[sd_card]\nsource = "{self.root / "missing"}"\ndestination = "{self.destination}"\n'
                if workflow == "sd-card"
                else f'[rclone]\nsource = "{self.root / "missing"}"\nremote = "b2:photos"\n'
            )
            config.write_text(section)
            with mock.patch(
                "photos_backup.remote.backup.shutil.which", return_value="rclone"
            ):
                result = CliRunner().invoke(cli, ["--config", str(config), workflow])
            self.assertEqual(result.exit_code, 3, result.output)
            self.assertIn("source", result.output)
            self.assertFalse(self.destination.exists())

    def test_rsync_paths_remain_single_arguments_and_previews_create_nothing(self):
        for workflow in ("ssd", "sd_card"):
            with self.subTest(workflow=workflow):
                if workflow == "ssd":
                    backup = SsdBackup(
                        SsdConfig(self.source, self.destination, self.exclude),
                        delete_at_destination=True,
                        dry_run=True,
                    )
                else:
                    backup = SdCardBackup(
                        SdCardConfig(self.source, self.destination, self.exclude),
                        dry_run=True,
                    )
                with mock.patch(
                    f"photos_backup.{workflow}.backup.stream_command",
                    return_value=subprocess.CompletedProcess([], 0, stdout=""),
                ) as run:
                    backup.backup()
                arguments = run.call_args.args[0]
                self.assertEqual(
                    arguments[-3:], ["--", str(self.source), str(self.destination)]
                )
                self.assertIn(f"--exclude-from={self.exclude}", arguments)
                self.assertIn("--dry-run", arguments)
                self.assertEqual("--delete" in arguments, workflow == "ssd")
                self.assertFalse(self.destination.parent.exists())

    def test_real_copy_prepares_destination_and_preserves_transfer_statistics(self):
        for workflow in ("ssd", "sd_card"):
            with self.subTest(workflow=workflow):
                destination = self.destination / workflow
                if workflow == "ssd":
                    backup = SsdBackup(
                        SsdConfig(self.source, destination, None), False, False
                    )
                else:
                    backup = SdCardBackup(
                        SdCardConfig(self.source, destination, None), False
                    )

                def run(arguments, *, check):
                    self.assertTrue(destination.is_dir())
                    self.assertTrue(check)
                    self.assertNotIn("--dry-run", arguments)
                    self.assertFalse(
                        any(arg.startswith("--exclude-from") for arg in arguments)
                    )
                    return subprocess.CompletedProcess(
                        arguments,
                        0,
                        stdout=(
                            "Number of regular files transferred: 12\n"
                            "Total transferred file size: 42 bytes\n"
                        ),
                    )

                with mock.patch(
                    f"photos_backup.{workflow}.backup.stream_command", side_effect=run
                ):
                    result = backup.backup()
                summary = result[0] if isinstance(result, list) else result
                self.assertEqual(summary.files_transferred, 12)
                self.assertEqual(summary.total_size, "42 bytes")

    def test_remote_failure_reaches_cli_exit_status(self):
        config = self.root / "config.toml"
        config.write_text(f'[rclone]\nremote = "b2:photos"\nsource = "{self.source}"\n')
        with (
            mock.patch(
                "photos_backup.remote.backup.shutil.which", return_value="rclone"
            ),
            mock.patch(
                "photos_backup.remote.backup.stream_command",
                return_value=(
                    subprocess.CompletedProcess([], 5, stdout="network unavailable\n")
                ),
            ),
        ):
            result = CliRunner().invoke(cli, ["--config", str(config), "remote"])
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("rclone exited with code 5", result.output)

    def test_remote_preview_keeps_arguments_and_final_statistics(self):
        with (
            mock.patch(
                "photos_backup.remote.backup.shutil.which", return_value="rclone"
            ),
            mock.patch(
                "photos_backup.remote.backup.stream_command",
                return_value=(
                    subprocess.CompletedProcess(
                        [],
                        0,
                        stdout=(
                            "Transferred: 1 MiB / 9 MiB, 11%, (xfr#1/3)\n"
                            "Transferred: 9 MiB / 9 MiB, 100%, (xfr#3/3)\n"
                        ),
                    )
                ),
            ) as run,
        ):
            summary = RemoteBackup(
                RcloneConfig("b2:photos", self.source), self.source, True
            ).backup()
        self.assertIn("--dry-run", run.call_args.args[0])
        self.assertIn(str(self.source), run.call_args.args[0])
        self.assertEqual(summary.files_transferred, 3)
        self.assertEqual(summary.total_size, "9 MiB")

    def test_multiline_rclone_statistics_use_last_sample(self):
        stats = _parse_rclone_stats(
            "Transferred: 1 KiB / 2 KiB\nTransferred: 1 / 2, 50%\n"
            "Transferred: 2 KiB / 2 KiB\nTransferred: 2 / 2, 100%\n"
        )
        self.assertEqual(stats, {"files_transferred": 2, "total_size": "2 KiB"})

    def test_overlapping_copy_paths_stop_before_any_effects(self):
        alias = self.root / "source-alias"
        alias.symlink_to(self.source, target_is_directory=True)
        target_alias_parent = self.root / "target-alias-parent"
        target_alias_parent.mkdir()
        (target_alias_parent / self.source.name).symlink_to(self.source)
        destinations = (
            self.source,
            self.source / "nested" / "backup",
            alias / "backup",
            self.source.parent,  # rsync's effective target is the source itself.
            target_alias_parent,
        )
        for workflow in ("ssd", "sd_card"):
            for dry_run in (False, True):
                for destination in destinations:
                    with (
                        self.subTest(
                            workflow=workflow, dry_run=dry_run, destination=destination
                        ),
                        mock.patch.object(Path, "mkdir") as mkdir,
                        mock.patch(
                            f"photos_backup.{workflow}.backup.stream_command"
                        ) as run,
                    ):
                        backup = (
                            SsdBackup(
                                SsdConfig(self.source, destination, None), True, dry_run
                            )
                            if workflow == "ssd"
                            else SdCardBackup(
                                SdCardConfig(self.source, destination, None), dry_run
                            )
                        )
                        with self.assertRaises(ActionRequired):
                            backup.backup()
                        mkdir.assert_not_called()
                        run.assert_not_called()

    def test_ssd_sources_cannot_share_a_target(self):
        other_source = self.root / "other" / self.source.name
        other_source.mkdir(parents=True)
        for dry_run in (False, True):
            with (
                self.subTest(dry_run=dry_run),
                mock.patch.object(Path, "mkdir") as mkdir,
                mock.patch("photos_backup.ssd.backup.stream_command") as run,
            ):
                backup = SsdBackup(
                    SsdConfig(self.source, self.destination, None),
                    True,
                    dry_run,
                    sd_card=SdCardConfig(self.root, other_source, None),
                )
                with self.assertRaisesRegex(ActionRequired, "overlapping copy targets"):
                    backup.backup()
                mkdir.assert_not_called()
                run.assert_not_called()

    def test_ssd_target_cannot_overwrite_its_other_source(self):
        other_source = self.root / "stored-card"
        other_source.mkdir()
        self.destination.mkdir(parents=True)
        (self.destination / self.source.name).symlink_to(other_source)
        with mock.patch("photos_backup.ssd.backup.stream_command") as run:
            with self.assertRaisesRegex(ActionRequired, "overlaps source"):
                SsdBackup(
                    SsdConfig(self.source, self.destination, None),
                    True,
                    True,
                    sd_card=SdCardConfig(self.root, other_source, None),
                ).backup()
        run.assert_not_called()

    def test_exclusion_warning_is_visible_but_copy_continues(self):
        for workflow, section in (("sd_card", "sd_card"), ("ssd", "ssd")):
            for dry_run in (False, True):
                for exclusion in (None, self.exclude, self.root / "missing-exclusions"):
                    with self.subTest(
                        workflow=workflow, dry_run=dry_run, exclusion=exclusion
                    ):
                        config = self.root / "config.toml"
                        config.write_text(
                            f'[{section}]\nsource = "{self.source}"\n'
                            f'destination = "{self.destination}"\n'
                            + (f'exclude_file = "{exclusion}"\n' if exclusion else "")
                        )
                        with mock.patch(
                            f"photos_backup.{workflow}.backup.stream_command",
                            return_value=subprocess.CompletedProcess([], 0, stdout=""),
                        ) as run:
                            result = CliRunner().invoke(
                                cli,
                                ["--config", str(config), workflow.replace("_", "-")]
                                + (["--dry-run"] if dry_run else []),
                            )
                        self.assertEqual(result.exit_code, 0, result.output)
                        run.assert_called_once()
                        if exclusion is not None and not exclusion.exists():
                            self.assertIn(str(exclusion), result.stderr)
                            self.assertIn(
                                "continuing without these exclusions", result.stderr
                            )
                            self.assertFalse(
                                any(
                                    arg.startswith("--exclude-from")
                                    for arg in run.call_args.args[0]
                                )
                            )
                        else:
                            self.assertNotIn("Warning:", result.stderr)
