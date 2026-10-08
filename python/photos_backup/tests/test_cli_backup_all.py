from __future__ import annotations

import unittest
from unittest import mock

import click
from click.testing import CliRunner

from photos_backup.apple_photos.downloads import DEFAULT_DOWNLOAD_TIMEOUT
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.takeover import TakeoverCheck
from photos_backup.archive import ArchiveStateStore
from photos_backup.cli.backup_all import _export_apple_photos, backup_all
from photos_backup.cli.cli import cli
from photos_backup.summary import BackupSummary
from tests.archive_case import ArchiveCommandTestCase
from tests.test_export import FakeRunner, row


class BackupAllProgressTests(ArchiveCommandTestCase):
    def invoke(self, *arguments):
        return self.runner.invoke(
            cli,
            [
                "--config",
                str(self.config_path),
                "backup-all",
                "--skip-sd-card",
                "--skip-ssd",
                "--skip-remote",
                *arguments,
            ],
        )

    def test_timeout_and_progress_reach_exporter_and_stop_after_export(self):
        self.initialized_archive()
        for arguments, timeout in (
            ([], DEFAULT_DOWNLOAD_TIMEOUT),
            (["--download-timeout", "300"], 300),
        ):
            with (
                self.subTest(timeout=timeout),
                self.mounted(),
                mock.patch(
                    "photos_backup.cli.exporting.ensure_writer",
                    return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
                ),
                mock.patch(
                    "photos_backup.cli.exporting.run_osxphotos_export",
                    side_effect=lambda arguments, **kwargs: FakeRunner()(arguments),
                ) as runner,
            ):
                result = self.invoke(*arguments)
            self.assertEqual(result.exit_code, 0, result.output)
            self.assertIn(f"{timeout}s per asset", result.output)
            self.assertIn("Checking archive", result.output)
            self.assertIn("Checking archive writer", result.output)
            self.assertIn("Generating reports", result.output)
            self.assertIn("Phase timings", result.output)
            self.assertEqual(runner.call_args.kwargs["download_timeout"], timeout)
            progress = runner.call_args.kwargs["progress"]
            self.assertIn("Generating reports", progress.timings)
            self.assertFalse(progress._thread.is_alive())

    def test_dry_run_does_not_invoke_exporter_or_change_state(self):
        paths = self.initialized_archive()
        before = paths.state_file.read_bytes()
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.exporting.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
            ),
            mock.patch("photos_backup.cli.exporting.run_osxphotos_export") as runner,
        ):
            result = self.invoke("--dry-run", "--download-timeout", "300")
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("PREVIEW COMPLETE", result.output)
        self.assertIn("300s per asset", result.output)
        runner.assert_not_called()
        self.assertEqual(paths.state_file.read_bytes(), before)
        self.assertFalse(paths.reports.exists())
        self.assertFalse(paths.lock_file.exists())

    def test_skipped_export_does_not_start_progress_or_open_archive(self):
        with (
            mock.patch("photos_backup.cli.backup_all.ExportProgress") as progress,
            mock.patch("photos_backup.cli.exporting.open_archive") as opened,
        ):
            result = self.invoke("--skip-apple-photos")
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("NOTHING TO DO — all steps skipped", result.output)
        progress.assert_not_called()
        opened.assert_not_called()

    def test_invalid_timeout_is_rejected_before_opening_archive(self):
        for timeout in ("0", "-1", "invalid"):
            with (
                self.subTest(timeout=timeout),
                mock.patch("photos_backup.cli.exporting.open_archive") as opened,
            ):
                result = self.invoke("--download-timeout", timeout)
            self.assertEqual(result.exit_code, 2, result.output)
            opened.assert_not_called()

    def test_incomplete_export_still_fails_and_preserves_baseline(self):
        paths = self.initialized_archive()
        before = ArchiveStateStore(paths).load()
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.exporting.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
            ),
            mock.patch(
                "photos_backup.cli.exporting.run_osxphotos_export",
                side_effect=lambda arguments, **kwargs: FakeRunner(
                    [row("a.jpg", missing=1)]
                )(arguments),
            ),
        ):
            result = self.invoke()
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("1 file(s) remain missing", result.output)
        self.assertIn("BACKUP PIPELINE SUMMARY", result.output)
        self.assertEqual(ArchiveStateStore(paths).load(), before)


class ExecutablePreflightTests(ArchiveCommandTestCase):
    def test_destinations_are_printed_before_export_with_volume_override(self):
        self.configure_copies()
        override = self.root / "alternate"
        override.mkdir()

        def export(config, **kwargs):
            click.echo("EXPORT STARTED")
            return BackupSummary("Apple Photos")

        with (
            mock.patch(
                "photos_backup.cli.backup_all._export_apple_photos", side_effect=export
            ),
            mock.patch("photos_backup.cli.backup_all.SdCardBackup") as sd,
            mock.patch("photos_backup.cli.backup_all.SsdBackup") as ssd,
            mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote,
        ):
            sd.return_value.backup.return_value = BackupSummary("SD Card")
            ssd.return_value.backup.return_value = [BackupSummary("SSD")]
            remote.return_value.backup.return_value = BackupSummary("Remote")
            result = self.runner.invoke(
                cli,
                [
                    "--config",
                    str(self.config_path),
                    "--volume",
                    str(override),
                    "backup-all",
                    "--dry-run",
                    "--delete",
                ],
            )
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertLess(
            result.output.index("Backup destinations"),
            result.output.index("EXPORT STARTED"),
        )
        self.assertIn(str(override / "Media" / "Apple Photos"), result.output)
        self.assertIn(
            f"SSD: All Photos: {self.root}/photos → {self.root}/ssd/photos | deletions: ON",
            result.output,
        )
        self.assertIn(
            f"SD Card: {self.root}/card → {self.root}/raw/card | deletions: off",
            result.output,
        )
        self.assertIn(
            f"Remote: {self.root}/ssd → b2:photos | deletions: off", result.output
        )

    def test_skipped_steps_are_omitted_and_remote_deletion_is_independent(self):
        self.configure_copies()
        with mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote:
            remote.return_value.backup.return_value = BackupSummary("Remote")
            result = self.invoke(
                "--skip-apple-photos", "--skip-sd-card", "--skip-ssd", "--delete-remote"
            )
        self.assertEqual(result.exit_code, 0, result.output)
        overview = result.output.split("BACKUP PIPELINE SUMMARY")[0]
        self.assertNotIn("Apple Photos:", overview)
        self.assertNotIn("SD Card:", overview)
        self.assertNotIn("SSD:", overview)
        self.assertIn(f"Remote: {self.root}/ssd → b2:photos | deletions: ON", overview)

    def configure_copies(self):
        with self.config_path.open("a") as handle:
            handle.write(
                f'\n[sd_card]\nsource = "{self.root}/card"\n'
                f'destination = "{self.root}/raw"\n'
                f'[ssd]\nsource = "{self.root}/photos"\n'
                f'destination = "{self.root}/ssd"\n'
                '[rclone]\nremote = "b2:photos"\n'
            )

    def invoke(self, *arguments):
        return self.runner.invoke(
            cli, ["--config", str(self.config_path), "backup-all", *arguments]
        )

    def test_missing_tools_are_reported_together_before_any_backup_step(self):
        self.configure_copies()
        for missing in ({"exiftool", "rsync", "rclone"}, {"rclone"}):
            with (
                self.subTest(missing=missing),
                mock.patch(
                    "photos_backup.cli.backup_all.which",
                    side_effect=lambda name: (
                        None if name in missing else f"/bin/{name}"
                    ),
                ),
                mock.patch(
                    "photos_backup.cli.backup_all._export_apple_photos"
                ) as export,
                mock.patch("photos_backup.cli.backup_all.SdCardBackup") as sd,
                mock.patch("photos_backup.cli.backup_all.SsdBackup") as ssd,
                mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote,
            ):
                result = self.invoke()
            self.assertEqual(result.exit_code, 1, result.output)
            for name in missing:
                self.assertIn(name, result.output)
            self.assertIn("No backup steps were started", result.output)
            for effect in (export, sd, ssd, remote):
                effect.assert_not_called()
            self.assertEqual(list(self.volume.iterdir()), [])

    def test_only_tools_for_enabled_effects_are_checked(self):
        self.configure_copies()
        for flags, expected in (
            ([], ["exiftool", "rsync", "rclone"]),
            (["--dry-run"], ["rsync", "rclone"]),
            (["--skip-sd-card", "--skip-ssd", "--skip-remote"], ["exiftool"]),
            (["--skip-apple-photos", "--skip-sd-card", "--skip-remote"], ["rsync"]),
            (["--skip-apple-photos", "--skip-sd-card", "--skip-ssd"], ["rclone"]),
        ):
            with (
                self.subTest(flags=flags),
                mock.patch(
                    "photos_backup.cli.backup_all.which", return_value=None
                ) as which,
                mock.patch(
                    "photos_backup.cli.backup_all._export_apple_photos"
                ) as export,
            ):
                result = self.invoke(*flags)
            self.assertEqual(result.exit_code, 1, result.output)
            self.assertEqual(
                which.call_args_list, [mock.call(name) for name in expected]
            )
            export.assert_not_called()

    def test_unconfigured_transfers_do_not_require_tools(self):
        with (
            mock.patch(
                "photos_backup.cli.backup_all.which", return_value="/bin/exiftool"
            ) as which,
            mock.patch(
                "photos_backup.cli.backup_all._export_apple_photos",
                return_value=BackupSummary("Apple Photos"),
            ),
        ):
            result = self.invoke()
        self.assertEqual(result.exit_code, 0, result.output)
        which.assert_called_once_with("exiftool")

    def test_planning_only_export_needs_no_executable(self):
        self.initialized_archive()
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.backup_all.which", return_value=None
            ) as which,
            mock.patch(
                "photos_backup.cli.exporting.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
            ),
            mock.patch("photos_backup.cli.exporting.run_osxphotos_export") as runner,
        ):
            result = self.invoke("--dry-run")
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("PREVIEW COMPLETE", result.output)
        which.assert_not_called()
        runner.assert_not_called()

    def test_skipping_every_step_needs_no_executable_even_in_preview(self):
        self.configure_copies()
        with mock.patch(
            "photos_backup.cli.backup_all.which", return_value=None
        ) as which:
            result = self.invoke(
                "--skip-apple-photos",
                "--skip-sd-card",
                "--skip-ssd",
                "--skip-remote",
                "--dry-run",
            )
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("NOTHING TO DO — all steps skipped", result.output)
        self.assertNotIn("PREVIEW COMPLETE", result.output)
        which.assert_not_called()


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


class BackupAllTweaksTests(unittest.TestCase):
    def test_help_shows_only_the_current_ssd_deletion_flag(self):
        result = CliRunner().invoke(cli, ["backup-all", "--help"])
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("--delete-ssd", result.output)
        options = {param.name: param for param in backup_all.params}
        self.assertTrue(options["delete_alias"].hidden)
        self.assertIn("absent from the source", options["delete_remote"].help)
        self.assertNotIn("--delete ", result.output)
        self.assertNotIn("--delete,", result.output)

    def test_takeover_is_printed_after_the_live_status_line(self):
        events = []

        class Progress:
            def __enter__(self):
                events.append("progress started")
                return mock.MagicMock()

            def __exit__(self, *_):
                events.append("progress finished")

        config = mock.Mock(limit_export=25)
        with (
            mock.patch("photos_backup.cli.backup_all.ExportProgress", Progress),
            mock.patch("photos_backup.cli.exporting.open_archive"),
            mock.patch(
                "photos_backup.cli.exporting.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.CLAIMED, "this.local"),
            ),
            mock.patch("photos_backup.cli.exporting.ApplePhotosExport") as exporter,
            mock.patch(
                "photos_backup.cli.exporting.print_takeover_check",
                side_effect=lambda *_, **__: events.append("takeover"),
            ),
            mock.patch("photos_backup.cli.exporting.print_export_result"),
        ):
            _export_apple_photos(config, dry_run=True, download_timeout=5)
        self.assertEqual(events, ["progress started", "progress finished", "takeover"])
        self.assertNotIn("verbose", exporter.call_args.kwargs)
        self.assertNotIn("limit", exporter.call_args.kwargs)
        self.assertTrue(exporter.call_args.kwargs["plan_only"])
