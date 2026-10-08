from __future__ import annotations

import datetime
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.plan import ExportMode, ExportPlan, ExportResult
from photos_backup.apple_photos.verify import Check, VerificationReport
from photos_backup.archive import ArchivePaths, ArchiveState, ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.errors import ACTION_REQUIRED_EXIT_CODE, ActionRequired
from photos_backup.progress import ExportProgress
from tests.archive_case import ArchiveCommandTestCase
from photos_backup.verification_history import VerificationHistory
from tests.test_export import FakeRunner, row

UNCHANGED_WRITER = mock.Mock(status=WriterStatus.UNCHANGED)


class DailyTests(ArchiveCommandTestCase):
    def test_export_summary_survives_a_cleanup_exception(self):
        self.initialized_archive()
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.exporting.ensure_writer",
                return_value=UNCHANGED_WRITER,
            ),
            mock.patch(
                "photos_backup.cli.exporting.run_osxphotos_export",
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

    def test_results_print_only_while_no_progress_line_is_live(self):
        self.initialized_archive()
        events: list[str] = []
        enter, exit_ = ExportProgress.__enter__, ExportProgress.__exit__

        def record(name):
            return lambda *args, **kwargs: events.append(name)

        def entering(progress):
            events.append("progress open")
            return enter(progress)

        def exiting(progress, *exc):
            exit_(progress, *exc)
            events.append("progress closed")

        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.exporting.ensure_writer",
                return_value=mock.Mock(status=WriterStatus.CLAIMED),
            ),
            mock.patch(
                "photos_backup.cli.exporting.run_osxphotos_export",
                side_effect=lambda arguments, **kwargs: FakeRunner()(arguments),
            ),
            mock.patch.object(ExportProgress, "__enter__", entering),
            mock.patch.object(ExportProgress, "__exit__", exiting),
            mock.patch(
                "photos_backup.cli.exporting.print_takeover_check",
                side_effect=record("takeover"),
            ),
            mock.patch(
                "photos_backup.cli.exporting.print_export_result",
                side_effect=record("export result"),
            ),
            mock.patch(
                "photos_backup.cli.daily.reconcile_mirror",
                side_effect=lambda *args: (
                    events.append("reconcile") or mock.Mock(pending=False)
                ),
            ),
            mock.patch("photos_backup.cli.daily.print_mirror_outcome"),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily"]
            )

        self.assertEqual(result.exit_code, 0, result.output)

        self.assertEqual(
            events,
            [
                "progress open",
                "progress closed",
                "takeover",
                "export result",
                "progress open",
                "reconcile",
                "progress closed",
            ],
        )

    def test_missing_downloads_fail_daily_without_advancing_or_cleaning(self):
        paths = self.initialized_archive()
        before = ArchiveStateStore(paths).load()
        runner = FakeRunner([row("missing.mov", missing=1)])
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.exporting.ensure_writer",
                return_value=UNCHANGED_WRITER,
            ),
            mock.patch(
                "photos_backup.cli.exporting.run_osxphotos_export",
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
                    "photos_backup.cli.exporting.ensure_writer",
                    return_value=UNCHANGED_WRITER,
                ),
                mock.patch(
                    "photos_backup.cli.exporting.run_osxphotos_export",
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
        with mock.patch("photos_backup.cli.exporting.open_archive") as opened:
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
                "photos_backup.cli.exporting.ensure_writer",
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
                "photos_backup.cli.exporting.ensure_writer",
                return_value=UNCHANGED_WRITER,
            ),
            mock.patch(
                "photos_backup.cli.exporting.ApplePhotosExport",
                return_value=mock.Mock(export=lambda: export),
            ),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE, result.output)
        self.assertIn("Mirror (pending)", result.output)
        self.assertIn("approve-cleanup run-7", result.output)


class DailyVerifyTests(ArchiveCommandTestCase):
    def run_daily(self, export: ExportResult, report: VerificationReport, *args):
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.exporting.ensure_writer",
                return_value=UNCHANGED_WRITER,
            ),
            mock.patch(
                "photos_backup.cli.exporting.ApplePhotosExport",
                return_value=mock.Mock(export=lambda: export),
            ),
            mock.patch(
                "photos_backup.cli.verify.verify_archive", return_value=report
            ) as scan,
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily", *args]
            )
        return result, scan

    def export(self, **changes) -> ExportResult:
        fields = dict(
            plan=ExportPlan(ExportMode.INCREMENTAL, "photos created since"),
            exit_code=0,
            counts={"new": 1},
            state_advanced=True,
        )
        return ExportResult(**{**fields, **changes})

    def test_verify_records_a_receipt_status_treats_as_fresh(self) -> None:
        paths = self.initialized_archive()
        passing = VerificationReport((Check("state", True, "ok"),))
        result, scan = self.run_daily(self.export(), passing, "--verify")

        self.assertEqual(result.exit_code, 0, result.output)
        scan.assert_called_once()
        self.assertIn("All 1 check(s) passed", result.output)
        receipt, error = VerificationHistory(self.config_path, paths.archive).read()
        self.assertIsNone(error)
        self.assertTrue(receipt["passed"])
        with self.mounted():
            status = self.runner.invoke(
                cli, ["--config", str(self.config_path), "status", "--short"]
            )
        self.assertNotIn("verify --record", status.stdout)

    def test_verify_is_opt_in_and_skipped_for_dry_runs_and_failed_exports(self):
        paths = self.initialized_archive()
        passing = VerificationReport((Check("state", True, "ok"),))
        for args, export, code in (
            ((), self.export(), 0),
            (("--verify", "--dry-run"), self.export(), 0),
            (("--verify",), self.export(exit_code=1), 1),
        ):
            with self.subTest(args=args):
                result, scan = self.run_daily(export, passing, *args)
                self.assertEqual(result.exit_code, code, result.output)
                scan.assert_not_called()
        history = VerificationHistory(self.config_path, paths.archive)
        self.assertEqual(history.read(), (None, None))

    def test_failed_checks_fail_daily_after_printing_the_export(self) -> None:
        self.initialized_archive()
        failing = VerificationReport((Check("missing assets", False, "1 gone"),))
        result, _ = self.run_daily(self.export(), failing, "--verify")

        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("Archive check(s) failed: missing assets", result.output)
        self.assertLess(
            result.output.index("Exported ("), result.output.index("1 gone")
        )

    def test_a_pending_cleanup_still_asks_for_approval(self) -> None:
        self.initialized_archive(pending_cleanup_run_id="run-7")
        pending = VerificationReport((Check("pending cleanup", False, "run-7"),))
        result, scan = self.run_daily(self.export(), pending, "--verify")

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE, result.output)
        scan.assert_called_once()
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
