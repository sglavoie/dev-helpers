from unittest import mock

from photos_backup.apple_photos.downloads import DEFAULT_DOWNLOAD_TIMEOUT
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.takeover import TakeoverCheck
from photos_backup.archive import ArchiveStateStore
from photos_backup.cli.cli import cli
from tests.test_cli import ArchiveCommandTestCase
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
                    "photos_backup.cli.backup_all.ensure_writer",
                    return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
                ),
                mock.patch(
                    "photos_backup.cli.backup_all.run_osxphotos_export",
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
                "photos_backup.cli.backup_all.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
            ),
            mock.patch("photos_backup.cli.backup_all.run_osxphotos_export") as runner,
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
            mock.patch("photos_backup.cli.backup_all.open_archive") as opened,
        ):
            result = self.invoke("--skip-apple-photos")
        self.assertEqual(result.exit_code, 0, result.output)
        progress.assert_not_called()
        opened.assert_not_called()

    def test_invalid_timeout_is_rejected_before_opening_archive(self):
        for timeout in ("0", "-1", "invalid"):
            with (
                self.subTest(timeout=timeout),
                mock.patch("photos_backup.cli.backup_all.open_archive") as opened,
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
                "photos_backup.cli.backup_all.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
            ),
            mock.patch(
                "photos_backup.cli.backup_all.run_osxphotos_export",
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
