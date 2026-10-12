from __future__ import annotations

import subprocess
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.config import RcloneConfig, SdCardConfig, SsdConfig
from photos_backup.errors import ActionRequired
from photos_backup.remote.backup import Backup as RemoteBackup, _parse_rclone_stats
from photos_backup.sd_card.backup import Backup as SdCardBackup
from photos_backup.ssd.backup import Backup as SsdBackup

from tests.copy_workflow_case import CopyWorkflowTestCase


class CopySafetyWorkflowTests(CopyWorkflowTestCase):
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

    def test_ssd_deletions_require_all_configured_exclusions_before_any_copy(self):
        card_backup = self.root / "card-backup"
        card_backup.mkdir()
        for invalid in (self.root / "missing", card_backup):
            for section in ("ssd", "sd_card"):
                for command in ("ssd", "backup-all"):
                    for dry_run in (False, True):
                        with self.subTest(
                            invalid=invalid,
                            section=section,
                            command=command,
                            dry_run=dry_run,
                        ):
                            config = self.root / "config.toml"
                            config.write_text(
                                f'[ssd]\nsource = "{self.source}"\n'
                                f'destination = "{self.destination}"\n'
                                f'exclude_file = "{invalid if section == "ssd" else self.exclude}"\n'
                                f'[sd_card]\nsource = "{self.root / "card"}"\n'
                                f'destination = "{card_backup}"\n'
                                f'exclude_file = "{invalid if section == "sd_card" else self.exclude}"\n'
                            )
                            args = ["--config", str(config), command, "--delete"]
                            if dry_run:
                                args.append("--dry-run")
                            if command == "backup-all":
                                args += [
                                    "--skip-apple-photos",
                                    "--skip-sd-card",
                                    "--skip-remote",
                                ]
                            with mock.patch(
                                "photos_backup.ssd.backup.stream_command"
                            ) as run:
                                result = CliRunner().invoke(cli, args)
                            self.assertEqual(result.exit_code, 3, result.output)
                            self.assertIn("refusing SSD deletions", result.output)
                            run.assert_not_called()
                            self.assertFalse(self.destination.exists())

    def test_transfer_progress_adapts_to_terminal_and_preserves_statistics(self):
        for terminal in (False, True):
            for workflow in ("ssd", "sd_card", "remote"):
                with (
                    self.subTest(terminal=terminal, workflow=workflow),
                    mock.patch(
                        f"photos_backup.{workflow}.backup.interactive_transfers",
                        return_value=terminal,
                    ),
                    mock.patch(
                        "photos_backup.remote.backup.shutil.which",
                        return_value="rclone",
                    ),
                    mock.patch(
                        f"photos_backup.{workflow}.backup.stream_command",
                        return_value=subprocess.CompletedProcess([], 0, stdout=""),
                    ) as run,
                ):
                    if workflow == "ssd":
                        SsdBackup(
                            SsdConfig(self.source, self.destination, None), False, True
                        ).backup()
                    elif workflow == "sd_card":
                        SdCardBackup(
                            SdCardConfig(self.source, self.destination, None), True
                        ).backup()
                    else:
                        RemoteBackup(
                            RcloneConfig("b2:photos", self.source), self.source, True
                        ).backup()
                    args = run.call_args.args[0]
                    self.assertEqual("--progress" in args, terminal)
                    self.assertIn("--stats", args)
                    if workflow != "remote":
                        self.assertIn("--itemize-changes", args)
                    if workflow == "ssd":
                        self.assertEqual("--verbose" in args, terminal)
                    if workflow == "remote" and not terminal:
                        self.assertIn("NOTICE", args)
                        self.assertIn("1m", args)

    def test_rclone_logged_one_line_statistics_keep_count_and_size(self):
        output = "2026/10/04 12:00:00 NOTICE: 9 MiB / 9 MiB, 100%, 1 MiB/s, ETA 0s (xfr#3/3)\n"
        self.assertEqual(
            _parse_rclone_stats(output), {"files_transferred": 3, "total_size": "9 MiB"}
        )
