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

UNCHANGED_WRITER = mock.Mock(status=WriterStatus.UNCHANGED)


class DailyTests(ArchiveCommandTestCase):
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
