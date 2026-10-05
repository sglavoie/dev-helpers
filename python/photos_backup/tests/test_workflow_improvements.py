from __future__ import annotations

import datetime
import json
import shlex
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.archive.state import ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.config import SdCardConfig, SsdConfig
from photos_backup.errors import ActionRequired
from photos_backup.ssd.backup import Backup as SsdBackup
from tests.test_cli import ArchiveCommandTestCase
from tests.test_verify import MONDAY, THURSDAY, VerifyTestCase, write_export_db


class SsdSafetyTests(unittest.TestCase):
    def setUp(self):
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


class PipelinePreflightTests(ArchiveCommandTestCase):
    def test_skip_reasons_distinguish_explicit_flags_from_missing_sections(self):
        self.config_path.write_text("")
        for explicit in (False, True):
            with self.subTest(explicit=explicit):
                args = [
                    "--config",
                    str(self.config_path),
                    "backup-all",
                    "--skip-apple-photos",
                ]
                if explicit:
                    args += ["--skip-sd-card", "--skip-ssd", "--skip-remote"]
                result = self.runner.invoke(cli, args)
                self.assertEqual(result.exit_code, 0, result.output)
                self.assertIn("NOTHING TO DO — all steps skipped", result.output)
                self.assertNotIn("ALL OK", result.output)
                self.assertEqual(
                    result.output.count("Skipped by request"), 4 if explicit else 1
                )
                for section in ("sd_card", "ssd", "rclone"):
                    if explicit:
                        self.assertNotIn(f"Not configured: [{section}]", result.output)
                    else:
                        self.assertIn(f"Not configured: [{section}]", result.output)

    def test_bad_later_configuration_stops_before_export(self):
        with self.config_path.open("a") as handle:
            handle.write("\n[rclone]\nremote = 42\n")
        with mock.patch("photos_backup.cli.backup_all.open_archive") as opened:
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "backup-all"]
            )
        self.assertEqual(result.exit_code, 2, result.output)
        self.assertIn("[rclone] remote", result.output)
        opened.assert_not_called()
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_remote_fallback_configuration_is_validated_even_when_ssd_skipped(self):
        self.config_path.write_text('[rclone]\nremote = "b2:photos"\n')
        with mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote:
            result = self.runner.invoke(
                cli,
                [
                    "--config",
                    str(self.config_path),
                    "backup-all",
                    "--skip-apple-photos",
                    "--skip-ssd",
                ],
            )
        self.assertEqual(result.exit_code, 2, result.output)
        self.assertIn("[ssd]", result.output)
        remote.assert_not_called()

    def test_skipped_invalid_sections_are_not_loaded(self):
        self.config_path.write_text("[rclone]\nremote = 42\n")
        result = self.runner.invoke(
            cli,
            [
                "--config",
                str(self.config_path),
                "backup-all",
                "--skip-apple-photos",
                "--skip-remote",
            ],
        )
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("SKIPPED", result.output)


class PreviewSummaryTests(unittest.TestCase):
    def setUp(self):
        self.enterContext(
            mock.patch(
                "photos_backup.cli.backup_all.which", return_value="/test/bin/tool"
            )
        )

    def test_copy_previews_never_claim_files_were_transferred(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / "config.toml"
            config.write_text(
                f'[sd_card]\nsource = "{root / "card"}"\ndestination = "{root / "sd"}"\n'
                f'[ssd]\nsource = "{root / "photos"}"\ndestination = "{root / "ssd"}"\n'
                f'[rclone]\nsource = "{root}"\nremote = "b2:photos"\n'
            )
            (root / "card").mkdir()
            (root / "photos").mkdir()
            (root / "sd").mkdir()
            completed = subprocess.CompletedProcess(
                [],
                0,
                stdout="Number of regular files transferred: 3\n"
                "Total transferred file size: 42 bytes\n"
                "Transferred: 42 bytes / 42 bytes, (xfr#3/3)\n",
            )
            for command in ("sd-card", "ssd", "remote", "backup-all"):
                with (
                    self.subTest(command=command),
                    mock.patch(
                        "photos_backup.remote.backup.shutil.which",
                        return_value="rclone",
                    ),
                    mock.patch(
                        "photos_backup.remote.backup.stream_command",
                        return_value=completed,
                    ),
                    mock.patch(
                        "photos_backup.sd_card.backup.stream_command",
                        return_value=completed,
                    ),
                    mock.patch(
                        "photos_backup.ssd.backup.stream_command",
                        return_value=completed,
                    ),
                ):
                    args = ["--config", str(config), command, "--dry-run"]
                    if command == "backup-all":
                        args.append("--skip-apple-photos")
                    result = CliRunner().invoke(cli, args)
                    self.assertEqual(result.exit_code, 0, result.output)
                    self.assertIn("DRY RUN", result.output)
                    self.assertIn("proposed transfers", result.output.lower())
                    self.assertNotIn("Files transferred:", result.output)
                    self.assertNotIn("Status: OK", result.output)
                    self.assertNotIn("ALL OK", result.output)
            self.assertFalse((root / "ssd").exists())


class StatusTests(ArchiveCommandTestCase):
    def test_status_uses_existing_cadence_and_shows_latest_report(self):
        report = self.root / "export.csv"
        self.initialized_archive(
            last_full_export_at=MONDAY,
            last_successful_export_at=THURSDAY,
            last_report_path=report,
        )
        with mock.patch("photos_backup.archive.Archive.now", return_value=THURSDAY):
            result = self.invoke_status()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Next export: incremental", result.output)
        self.assertIn(str(report), result.output)
        self.assertIn(THURSDAY.isoformat(), result.output)
        self.assertIn(f"{THURSDAY.isoformat()} (just now)", result.output)
        self.assertIn(f"{MONDAY.isoformat()} (3 days ago)", result.output)
        self.assertIn("Last archive cleanup reconciliation:", result.output)

    def test_status_relative_ages_handle_units_and_future_timestamps(self):
        store = ArchiveStateStore(self.initialized_archive())
        for offset, expected in (
            (datetime.timedelta(seconds=59), "just now"),
            (datetime.timedelta(minutes=1), "1 minute ago"),
            (datetime.timedelta(minutes=2), "2 minutes ago"),
            (datetime.timedelta(hours=1), "1 hour ago"),
            (datetime.timedelta(days=1), "1 day ago"),
            (datetime.timedelta(seconds=-30), "in less than a minute"),
            (datetime.timedelta(hours=-2), "in 2 hours"),
        ):
            with self.subTest(expected=expected):
                timestamp = THURSDAY - offset
                store.update(last_successful_export_at=timestamp)
                with mock.patch(
                    "photos_backup.archive.Archive.now", return_value=THURSDAY
                ):
                    result = self.invoke_status()
                self.assertEqual(result.exit_code, 0, result.output)
                self.assertIn(
                    f"Last successful export: {timestamp.isoformat()} ({expected})",
                    result.output,
                )

    def invoke_status(self):
        with (
            self.mounted(),
            mock.patch("photos_backup.apple_photos.adapter.PhotosProbes") as photos,
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "status"]
            )
        photos.assert_not_called()
        return result

    def test_new_archive_status_creates_nothing_and_suggests_bootstrap(self):
        result = self.invoke_status()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("(unclaimed)", result.output)
        self.assertIn(
            shlex.join(
                ["photos-backup", "--config", str(self.config_path), "bootstrap"]
            ),
            result.output,
        )
        self.assertIn("Next export: full", result.output)
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_status_shows_pending_cleanup_and_does_not_modify_state(self):
        paths = self.initialized_archive(
            writer_hostname="another-mac", pending_cleanup_run_id="run-7"
        )
        before = paths.state_file.read_bytes()
        result = self.invoke_status()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("another-mac", result.output)
        self.assertIn("approve-cleanup run-7", result.output)
        self.assertIn("approve-cleanup run-7 --discard", result.output)
        self.assertIn("files have not been verified", result.output)
        self.assertEqual(paths.state_file.read_bytes(), before)
        self.assertFalse(paths.lock_file.exists())
        self.assertFalse(paths.export_db.exists())

    def test_corrupt_state_is_reported_without_repair(self):
        paths = self.initialized_archive()
        paths.state_file.write_text("invalid")
        result = self.invoke_status()
        self.assertNotEqual(result.exit_code, 0)
        self.assertIn("not valid JSON", result.output)
        self.assertEqual(paths.state_file.read_text(), "invalid")


class VerificationReportTests(VerifyTestCase):
    def test_pending_cleanup_is_action_required_unless_integrity_also_fails(self):
        self.save_state(pending_cleanup_run_id="run-7")
        before = self.paths.state_file.read_bytes()
        for damaged in (False, True):
            with self.subTest(damaged=damaged):
                if damaged:
                    self.exported.unlink()
                destination = self.root / f"cleanup-{damaged}.json"
                result = self.invoke_verify(destination)
                self.assertEqual(result.exit_code, 1 if damaged else 3, result.output)
                self.assertIn("approve-cleanup run-7", result.output)
                document = json.loads(destination.read_text())
                self.assertFalse(document["passed"])
                failed = [
                    check["name"] for check in document["checks"] if not check["passed"]
                ]
                self.assertIn("pending cleanup", failed)
                if damaged:
                    self.assertIn("missing assets", failed)
                else:
                    self.assertEqual(failed, ["pending cleanup"])
                self.assertEqual(self.paths.state_file.read_bytes(), before)

    def invoke_verify(self, destination):
        config = self.root / "config.toml"
        config.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\narchive = "{self.archive_root}"\n'
            'library = "/unused.photoslibrary"\n'
        )
        return CliRunner().invoke(
            cli,
            [
                "--config",
                str(config),
                "--volume",
                str(self.volume),
                "verify",
                "--report",
                str(destination),
            ],
        )

    def test_report_keeps_all_missing_and_changed_paths_on_failure(self):
        missing = [self.archive_root / f"missing-{index}.jpg" for index in range(5)]
        for path in missing:
            path.write_bytes(b"photo")
        write_export_db(self.paths.export_db, [self.exported, *missing])
        for path in missing:
            path.unlink()
        self.exported.write_bytes(b"changed")
        before = self.paths.state_file.read_bytes()
        destination = self.root / "findings.json"
        result = self.invoke_verify(destination)
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("and 2 more", result.output)
        document = json.loads(destination.read_text())
        self.assertFalse(document["passed"])
        checks = {check["name"]: check for check in document["checks"]}
        self.assertEqual(
            checks["missing assets"]["paths"], [str(path) for path in missing]
        )
        self.assertEqual(checks["signatures"]["paths"], [str(self.exported)])
        self.assertEqual(self.paths.state_file.read_bytes(), before)

    def test_report_can_record_a_healthy_archive(self):
        destination = self.root / "healthy.json"
        result = self.invoke_verify(destination)
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertTrue(json.loads(destination.read_text())["passed"])
        self.assertIn("Reading export database", result.stderr)
        self.assertIn("files checked 1/1", result.stderr)
        self.assertNotIn("unresolved downloads", result.stderr)
        self.assertNotIn("files checked", result.stdout)

    def test_report_never_overwrites_an_existing_file_or_writes_inside_archive(self):
        existing = self.root / "existing.json"
        existing.write_text("keep me")
        for destination in (
            existing,
            self.archive_root / "new.json",
            self.paths.state_file,
        ):
            with self.subTest(destination=destination):
                result = self.invoke_verify(destination)
                self.assertEqual(result.exit_code, 2, result.output)
        self.assertEqual(existing.read_text(), "keep me")
        self.assertFalse((self.archive_root / "new.json").exists())

    def test_report_write_error_is_a_readable_cli_failure(self):
        result = self.invoke_verify(self.root / "absent" / "report.json")
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("Could not write report", result.output)
