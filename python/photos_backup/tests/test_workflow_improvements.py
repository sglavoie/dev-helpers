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

from photos_backup.archive import ArchiveLocked, ArchiveUnavailable
from photos_backup.archive.state import ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.config import SdCardConfig, SsdConfig
from photos_backup.errors import ActionRequired
from photos_backup.ssd.backup import Backup as SsdBackup
from photos_backup.summary import BackupSummary
from photos_backup.transfers import TransferHistory, annotate_archive_freshness
from tests import isolate_transfer_history
from tests.test_cli import ArchiveCommandTestCase
from tests.timezones import pin_timezone
from tests.test_verify import MONDAY, THURSDAY, VerifyTestCase, write_export_db


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


class PipelinePreflightTests(ArchiveCommandTestCase):
    def test_missing_apple_photos_suggests_copy_only_without_starting_work(self):
        self.config_path.write_text("")
        with mock.patch("photos_backup.cli.backup_all._check_executables") as check:
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "backup-all"]
            )
        self.assertEqual(result.exit_code, 2, result.output)
        self.assertIn("backup-all --skip-apple-photos", result.output)
        self.assertIn(str(self.config_path), result.output)
        check.assert_not_called()

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
        with mock.patch("photos_backup.cli.exporting.open_archive") as opened:
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
    def setUp(self):
        super().setUp()
        # The full-export cadence follows local midnight; fixtures are in UTC.
        pin_timezone(self, "UTC")

    def test_unavailable_archive_still_shows_history_without_writes(self):
        history = TransferHistory(self.config_path)
        history.run(
            "Remote",
            self.root / "photos",
            "b2:photos",
            lambda: BackupSummary("Remote", files_transferred=2),
            dry_run=False,
        )
        before = {p: p.read_bytes() for p in history.directory.iterdir()}
        for error in (ArchiveUnavailable("Drive disconnected"), ArchiveLocked("Busy")):
            for as_json in (False, True):
                with (
                    self.subTest(error=error, as_json=as_json),
                    mock.patch(
                        "photos_backup.cli.status.open_archive", side_effect=error
                    ),
                ):
                    result = self.invoke_status(*(["--json"] if as_json else []))
                self.assertEqual(result.exit_code, error.exit_code, result.output)
                self.assertIn("b2:photos", result.stdout)
                if as_json:
                    document = json.loads(result.stdout)
                    self.assertIsNone(document["state"])
                    self.assertIsNone(document["next_export"])
                    self.assertEqual(document["archive_error"], str(error))
                    self.assertFalse(document["files_verified"])
                else:
                    self.assertIn("Archive unavailable", result.stdout)
        self.assertEqual(
            {p: p.read_bytes() for p in history.directory.iterdir()}, before
        )
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_json_status_is_one_document_with_iso_dates_and_no_writes(self):
        paths = self.initialized_archive(
            writer_hostname="test.local",
            pending_cleanup_run_id="run-7",
            last_full_export_at=MONDAY,
            last_successful_export_at=THURSDAY,
            last_report_path=self.root / "report.csv",
        )
        before = paths.state_file.read_bytes()
        with mock.patch("photos_backup.archive.Archive.now", return_value=THURSDAY):
            result = self.invoke_status("--json")
        self.assertEqual(result.exit_code, 0, result.output)
        document = json.loads(result.stdout)
        self.assertEqual(document["version"], 1)
        self.assertFalse(document["files_verified"])
        self.assertTrue(document["archive_configured"])
        self.assertEqual(
            document["state"]["last_successful_export_at"], THURSDAY.isoformat()
        )
        self.assertEqual(document["state"]["pending_cleanup_run_id"], "run-7")
        self.assertEqual(document["next_export"]["mode"], "incremental")
        self.assertEqual(document["transfers"], [])
        self.assertEqual(document["transfer_history_errors"], [])
        self.assertEqual(result.stderr, "")
        self.assertEqual(paths.state_file.read_bytes(), before)
        self.assertFalse(paths.lock_file.exists())
        self.assertFalse(self.history_root.exists())

    def test_json_status_of_uninitialized_archive_keeps_nulls(self):
        result = self.invoke_status("--json")
        self.assertEqual(result.exit_code, 0, result.output)
        document = json.loads(result.stdout)
        self.assertIsNone(document["state"]["initialized_at"])
        self.assertIsNone(document["state"]["last_report_path"])
        self.assertEqual(document["next_export"]["mode"], "full")
        self.assertFalse(self.history_root.exists())
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_status_shows_local_transfer_results_and_reports_corrupt_receipts(self):
        self.initialized_archive()
        history = TransferHistory(self.config_path)
        history.run(
            "Remote",
            self.root / "photos",
            "b2:photos",
            lambda: BackupSummary("Remote", files_transferred=2),
            dry_run=False,
        )
        result = self.invoke_status()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Last successful copy:", result.output)
        self.assertIn("b2:photos", result.output)
        document = json.loads(self.invoke_status("--json").stdout)
        self.assertEqual(
            document["transfers"][0]["last_success"]["files_transferred"], 2
        )
        receipt = next(history.directory.glob("*.json"))
        receipt.write_text("invalid")
        result = self.invoke_status("--json")
        self.assertEqual(result.exit_code, 0, result.output)
        document = json.loads(result.stdout)
        self.assertEqual(len(document["transfer_history_errors"]), 1)
        self.assertEqual(document["transfers"], [])
        self.assertEqual(receipt.read_text(), "invalid")

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

    def invoke_status(self, *args):
        with (
            self.mounted(),
            mock.patch("photos_backup.apple_photos.adapter.PhotosProbes") as photos,
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "status", *args]
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


class ArchiveFreshnessTests(ArchiveCommandTestCase):
    def test_status_compares_exact_archive_sources_without_probing_destinations(self):
        paths = self.initialized_archive(last_successful_export_at=THURSDAY)
        with self.config_path.open("a") as config:
            config.write(
                f'\n[ssd]\nsource = "{paths.archive}"\ndestination = "{self.root}/offline"\n'
                f'\n[rclone]\nsource = "{paths.archive.parent}"\nremote = "b2:photos"\n'
            )
        older = THURSDAY - datetime.timedelta(days=1)
        newer = THURSDAY + datetime.timedelta(days=1)
        history = TransferHistory(self.config_path)
        for copied_at, expected in ((older, True), (newer, False)):
            with mock.patch(
                "photos_backup.transfers._now", return_value=copied_at.isoformat()
            ):
                history.run(
                    "SSD: All Photos",
                    paths.archive,
                    self.root / "offline" / paths.archive.name,
                    lambda: BackupSummary(step_name="SSD: All Photos"),
                    dry_run=False,
                )
                history.run(
                    "Remote",
                    paths.archive.parent,
                    "b2:photos",
                    lambda: BackupSummary(step_name="Remote"),
                    dry_run=False,
                )
            with mock.patch(
                "pathlib.Path.resolve",
                side_effect=AssertionError("no filesystem resolution"),
            ):
                rows, errors = history.read()
                annotated = annotate_archive_freshness(rows, paths.archive, THURSDAY)
            self.assertEqual(errors, [])
            by_step = {row["step"]: row for row in annotated}
            self.assertIs(
                by_step["SSD: All Photos"]["archive_exported_since_copy"], expected
            )
            self.assertIsNone(by_step["Remote"]["archive_exported_since_copy"])
            with self.mounted():
                result = self.runner.invoke(
                    cli, ["--config", str(self.config_path), "status", "--json"]
                )
                text = self.runner.invoke(
                    cli, ["--config", str(self.config_path), "status"]
                )
            self.assertEqual(result.exit_code, 0, result.output)
            rows = {
                row["step"]: row
                for row in json.loads(result.stdout)["configured_transfers"]
            }
            self.assertIs(
                rows["SSD: All Photos"]["archive_exported_since_copy"], expected
            )
            self.assertEqual(
                "Archive exported since this copy" in text.stdout, expected
            )
            self.assertFalse((self.root / "offline").exists())


class VerificationReportTests(VerifyTestCase):
    def assert_timing(self, document):
        started = datetime.datetime.fromisoformat(document["started_at"])
        completed = datetime.datetime.fromisoformat(document["completed_at"])
        self.assertEqual(started.utcoffset(), datetime.timedelta(0))
        self.assertEqual(completed.utcoffset(), datetime.timedelta(0))
        self.assertGreaterEqual(completed, started)
        self.assertGreaterEqual(document["elapsed_seconds"], 0)

    def test_json_archive_open_errors_preserve_exit_codes_without_scanning(self):
        before = self.paths.state_file.read_bytes()
        for error in (
            ArchiveUnavailable("archive drive is not mounted"),
            ArchiveLocked("archive is busy"),
            PermissionError("archive cannot be read"),
        ):
            with (
                self.subTest(error=error),
                mock.patch("photos_backup.cli.verify.open_archive", side_effect=error),
                mock.patch("photos_backup.cli.verify.verify_archive") as scan,
            ):
                result = self.invoke_verify(None, "--json")
            self.assertEqual(result.exit_code, getattr(error, "exit_code", 1))
            document = json.loads(result.stdout)
            self.assert_timing(document)
            self.assertEqual(document["archive"], str(self.archive_root))
            self.assertFalse(document["passed"])
            self.assertEqual(document["checks"], [])
            self.assertEqual(document["archive_error"], str(error))
            self.assertIn(str(error), result.stderr)
            scan.assert_not_called()
        self.assertEqual(self.paths.state_file.read_bytes(), before)
        self.assertFalse(self.paths.lock_file.exists())

    def test_json_stdout_matches_report_and_preserves_outcome_codes(self):
        before = self.paths.state_file.read_bytes()
        for condition, expected_code in (
            ("healthy", 0),
            ("cleanup", 3),
            ("missing", 1),
        ):
            if condition == "cleanup":
                self.save_state(pending_cleanup_run_id="run-7")
            elif condition == "missing":
                self.exported.unlink()
            for save_report in (False, True):
                with self.subTest(condition=condition, save_report=save_report):
                    destination = (
                        self.root / f"{condition}.json" if save_report else None
                    )
                    result = self.invoke_verify(destination, "--json")
                    self.assertEqual(result.exit_code, expected_code, result.output)
                    document = json.loads(result.stdout)
                    self.assert_timing(document)
                    self.assertEqual(document["passed"], expected_code == 0)
                    self.assertIsNone(document["archive_error"])
                    self.assertEqual(len(document["checks"]), 6)
                    self.assertIn("files checked", result.stderr)
                    if destination:
                        self.assertEqual(document, json.loads(destination.read_text()))
                        self.assertIn("Verification report:", result.stderr)
            if condition == "healthy":
                self.assertEqual(self.paths.state_file.read_bytes(), before)

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

    def invoke_verify(self, destination, *extra):
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
                *(["--report", str(destination)] if destination else []),
                *extra,
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

    def test_invalid_report_parent_stops_before_scanning_or_creating_anything(self):
        not_directory = self.root / "file"
        not_directory.write_text("keep")
        for parent in (self.root / "absent", not_directory):
            with (
                self.subTest(parent=parent),
                mock.patch("photos_backup.cli.verify.verify_archive") as scan,
            ):
                result = self.invoke_verify(parent / "report.json")
            self.assertEqual(result.exit_code, 2, result.output)
            self.assertIn("must be an existing directory", result.output)
            scan.assert_not_called()
        self.assertFalse((self.root / "absent").exists())
        self.assertEqual(not_directory.read_text(), "keep")

    def test_report_write_error_after_scan_is_a_readable_cli_failure(self):
        destination = self.root / "report.json"
        original_open = Path.open

        def fail_report(path, mode="r", *args, **kwargs):
            if path == destination and mode == "x":
                raise PermissionError("permission changed during scan")
            return original_open(path, mode, *args, **kwargs)

        with mock.patch.object(Path, "open", fail_report):
            result = self.invoke_verify(destination)
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("Could not write report", result.output)
