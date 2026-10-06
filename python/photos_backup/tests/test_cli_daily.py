from __future__ import annotations

import datetime
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.plan import ExportMode, ExportPlan, ExportResult
from photos_backup.archive import ArchivePaths, ArchiveState, ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.errors import ACTION_REQUIRED_EXIT_CODE, ActionRequired
from tests.test_cli import ArchiveCommandTestCase
from tests.test_export import FakeRunner, row

UNCHANGED_WRITER = mock.Mock(status=WriterStatus.UNCHANGED)


class DailyTests(ArchiveCommandTestCase):
    def test_export_summary_survives_a_cleanup_exception(self):
        self.initialized_archive()
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.daily.ensure_writer", return_value=UNCHANGED_WRITER
            ),
            mock.patch(
                "photos_backup.cli.daily.run_osxphotos_export",
                side_effect=lambda arguments, **kwargs: FakeRunner()(arguments),
            ),
            mock.patch(
                "photos_backup.cli.daily.reconcile_mirror",
                side_effect=ActionRequired("Cleanup needs attention"),
            ),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily"]
            )
        self.assertEqual(result.exit_code, 3, result.output)
        self.assertEqual(result.output.count("Exported ("), 1)
        self.assertIn("Status: OK", result.output)
        self.assertLess(
            result.output.index("Export report:"),
            result.output.index("Cleanup needs attention"),
        )

    def test_missing_downloads_fail_daily_without_advancing_or_cleaning(self):
        paths = self.initialized_archive()
        before = ArchiveStateStore(paths).load()
        runner = FakeRunner([row("missing.mov", missing=1)])
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.daily.ensure_writer", return_value=UNCHANGED_WRITER
            ),
            mock.patch(
                "photos_backup.cli.daily.run_osxphotos_export",
                side_effect=lambda arguments, **kwargs: runner(arguments),
            ),
            mock.patch("photos_backup.apple_photos.cleanup._reconcile") as reconcile,
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily"]
            )
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("1 file(s) remain missing", result.output)
        self.assertEqual(ArchiveStateStore(paths).load(), before)
        reconcile.assert_not_called()

    def test_timeout_and_progress_reach_exporter_and_dry_run_preserves_state(self):
        paths = self.initialized_archive(pending_cleanup_run_id="run-7")
        for dry_run in (True, False):
            with (
                self.subTest(dry_run=dry_run),
                self.mounted(),
                mock.patch(
                    "photos_backup.cli.daily.ensure_writer",
                    return_value=UNCHANGED_WRITER,
                ),
                mock.patch(
                    "photos_backup.cli.daily.run_osxphotos_export",
                    side_effect=lambda arguments, **kwargs: FakeRunner()(arguments),
                ) as runner,
            ):
                before = paths.state_file.read_bytes()
                reports = list(paths.reports.glob("*"))
                result = self.runner.invoke(
                    cli,
                    [
                        "--config",
                        str(self.config_path),
                        "daily",
                        "--download-timeout",
                        "300",
                        *(["--dry-run"] if dry_run else []),
                    ],
                )
            self.assertEqual(result.exit_code, 3, result.output)
            self.assertIn("300s per asset", result.output)
            self.assertIn("Total command time:", result.output)
            self.assertIn("Reconciling archive cleanup", result.output)
            if dry_run:
                runner.assert_not_called()
                self.assertEqual(paths.state_file.read_bytes(), before)
                self.assertEqual(list(paths.reports.glob("*")), reports)
            else:
                self.assertEqual(runner.call_args.kwargs["download_timeout"], 300)
                self.assertIn(
                    "Generating reports", runner.call_args.kwargs["progress"].timings
                )

    def test_invalid_timeout_is_rejected_before_opening_archive(self):
        with mock.patch("photos_backup.cli.daily.open_archive") as opened:
            result = self.runner.invoke(cli, ["daily", "--download-timeout", "0"])
        self.assertEqual(result.exit_code, 2, result.output)
        opened.assert_not_called()

    def test_daily_refuses_an_uninitialized_archive(self) -> None:
        with mock.patch(
            "photos_backup.archive.probes.os.path.ismount",
            lambda path: Path(path) == self.volume,
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily", "--dry-run"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("not initialized", result.output)
        self.assertIn("bootstrap", result.output)

    def test_daily_stops_before_exporting_when_a_takeover_is_blocked(self) -> None:
        paths = ArchivePaths(
            volume=self.volume, archive=self.volume / "Media" / "Apple Photos"
        )
        paths.metadata.mkdir(parents=True)
        ArchiveStateStore(paths).save(
            ArchiveState(
                initialized_at=datetime.datetime(
                    2026, 8, 11, 9, 30, tzinfo=datetime.UTC
                ),
                writer_hostname="another Mac.local",
            )
        )

        with (
            mock.patch(
                "photos_backup.archive.probes.os.path.ismount",
                lambda path: Path(path) == self.volume,
            ),
            mock.patch(
                "photos_backup.cli.daily.ensure_writer",
                side_effect=ActionRequired("that is a different library"),
            ),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("different library", result.output)
        self.assertEqual(list(paths.reports.glob("*.csv")), [])

    def test_daily_leaves_an_unmounted_volume_untouched(self) -> None:
        result = self.runner.invoke(
            cli, ["--config", str(self.config_path), "daily", "--dry-run"]
        )

        self.assertEqual(list(self.volume.iterdir()), [])
        self.assertNotEqual(result.exit_code, 0)

    def test_daily_asks_for_approval_while_a_cleanup_is_pending(self) -> None:
        self.initialized_archive(pending_cleanup_run_id="run-7")
        export = ExportResult(
            plan=ExportPlan(ExportMode.FULL, "a weekly full export"),
            exit_code=0,
            counts={"new": 1},
            state_advanced=True,
        )

        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.daily.ensure_writer",
                return_value=UNCHANGED_WRITER,
            ),
            mock.patch(
                "photos_backup.cli.daily.ApplePhotosExport",
                return_value=mock.Mock(export=lambda: export),
            ),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE, result.output)
        self.assertIn("Mirror (pending)", result.output)
        self.assertIn("approve-cleanup run-7", result.output)


class ManualExportTests(ArchiveCommandTestCase):
    def run_manual(self, result: ExportResult, *args: str):
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.apple_photos.ensure_writer",
                return_value=UNCHANGED_WRITER,
            ),
            mock.patch(
                "photos_backup.cli.apple_photos.ApplePhotosExport",
                return_value=mock.Mock(export=lambda: result),
            ),
        ):
            return self.runner.invoke(
                cli, ["--config", str(self.config_path), "apple-photos", *args]
            )

    def test_a_clean_manual_export_succeeds(self) -> None:
        result = self.run_manual(
            ExportResult(
                plan=ExportPlan(ExportMode.FULL, "a weekly full export"),
                exit_code=0,
                counts={"new": 1},
                state_advanced=True,
            )
        )

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Exported (full)", result.output)

    def test_a_failed_manual_export_reports_the_reason_and_fails(self) -> None:
        result = self.run_manual(
            ExportResult(
                plan=ExportPlan(
                    ExportMode.INCREMENTAL, "photos created since 2026-08-01"
                ),
                exit_code=1,
            )
        )

        self.assertEqual(result.exit_code, 1)
        self.assertIn("osxphotos exited with status 1", result.output)

    def test_a_manual_export_with_error_rows_fails(self) -> None:
        result = self.run_manual(
            ExportResult(
                plan=ExportPlan(ExportMode.FULL, "a weekly full export"),
                exit_code=0,
                counts={"new": 1, "error": 2},
            )
        )

        self.assertEqual(result.exit_code, 1)
        self.assertIn("2 export error(s)", result.output)


if __name__ == "__main__":
    unittest.main()
