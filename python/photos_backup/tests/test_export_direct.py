from __future__ import annotations

import csv
import dataclasses
import datetime
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.plan import (
    ExportMode,
)
from photos_backup.archive import ArchiveState
from photos_backup.progress import ExportProgress
from tests.export_case import (
    HOSTNAME,
    LAST_MONDAY,
    SAFE_HOSTNAME,
    THURSDAY,
    ExportTestCase,
    FakeRunner,
    ReportlessRunner,
    row,
)
from tests.timezones import pin_timezone


class DirectExportTests(ExportTestCase):
    def test_elapsed_time_includes_report_generation(self):
        now = [0.0]
        runner = FakeRunner([row("a.jpg", new=1)])

        def run(arguments):
            now[0] += 2
            return runner(arguments)

        def report(**kwargs):
            now[0] += 5
            return 1

        with (
            mock.patch(
                "photos_backup.apple_photos.export.time.monotonic",
                side_effect=lambda: now[0],
            ),
            mock.patch(
                "photos_backup.apple_photos.export.generate_late_photo_additions_report",
                side_effect=report,
            ),
        ):
            result = self.run_export(run)
        self.assertEqual(result.elapsed_seconds, 7)

    def test_a_clean_full_export_advances_both_timestamps(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1), row("b.jpg", updated=1)])

        result = self.run_export(runner)

        self.assertTrue(result.clean)
        self.assertTrue(result.state_advanced)
        self.assertIs(result.plan.mode, ExportMode.FULL)
        self.assertEqual(result.files_transferred, 2)
        self.assertEqual(self.state.last_successful_export_at, THURSDAY)
        self.assertEqual(self.state.last_full_export_at, THURSDAY)
        self.assertEqual(self.state.last_report_path, result.report_path)

    def test_a_clean_incremental_export_leaves_the_full_timestamp(self) -> None:
        monday = datetime.datetime(2026, 8, 10, 6, 0, tzinfo=datetime.UTC)
        runner = FakeRunner([row("a.jpg", new=1)])

        result = self.run_export(
            runner,
            state=ArchiveState(
                initialized_at=monday,
                last_full_export_at=monday,
                last_successful_export_at=monday,
            ),
        )

        self.assertIs(result.plan.mode, ExportMode.INCREMENTAL)
        self.assertEqual(self.state.last_full_export_at, monday)
        self.assertEqual(self.state.last_successful_export_at, THURSDAY)
        self.assertEqual(
            runner.arguments["from_date"], monday - datetime.timedelta(days=14)
        )

    def test_the_report_and_export_database_live_inside_the_archive(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1)])

        result = self.run_export(runner)

        expected = self.paths.export_report(HOSTNAME, THURSDAY.date())
        self.assertEqual(result.report_path, expected)
        self.assertIn(SAFE_HOSTNAME, expected.name)
        self.assertTrue(expected.is_file())
        self.assertEqual(runner.arguments["exportdb"], str(self.paths.export_db))
        self.assertEqual(runner.arguments["dest"], str(self.archive))

    def test_report_names_carry_the_local_date(self) -> None:
        pin_timezone(self, "Asia/Tokyo")
        # 18:00 UTC on Thursday is already Friday morning in Tokyo.
        evening = datetime.datetime(2026, 8, 13, 18, 0, tzinfo=datetime.UTC)
        self.probes = dataclasses.replace(self.probes, now=lambda: evening)

        result = self.run_export(FakeRunner([row("a.jpg", new=1)]))

        self.assertEqual(
            result.report_path,
            self.paths.export_report(HOSTNAME, datetime.date(2026, 8, 14)),
        )

    def test_the_late_additions_report_is_written_next_to_the_export_report(
        self,
    ) -> None:
        runner = FakeRunner([row("a.jpg", new=1), row("b.jpg", skipped=1)])

        result = self.run_export(runner)

        expected = self.paths.late_additions_report(HOSTNAME, THURSDAY.date())
        self.assertEqual(result.late_additions_path, expected)
        self.assertEqual(result.late_additions_rows, 1)
        self.assertTrue(expected.is_file())
        self.assertEqual(expected.parent, result.report_path.parent)

    def test_reported_errors_prevent_state_advancement(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1), row("b.jpg", error=1)])

        result = self.run_export(runner)

        self.assertFalse(result.clean)
        self.assertFalse(result.state_advanced)
        self.assertEqual(result.error_count, 1)
        self.assertIsNone(self.state.last_successful_export_at)
        self.assertIsNone(self.state.last_full_export_at)
        self.assertIn("1 export error", str(result.failure_reason()))

    def test_supplementary_report_failures_warn_and_preserve_success(self) -> None:
        for error in (
            OSError("disk full"),
            UnicodeError("bad text"),
            csv.Error("bad CSV"),
        ):
            for use_progress in (False, True):
                with self.subTest(error=error, progress=use_progress):
                    messages = []
                    progress = (
                        ExportProgress(sink=messages.append, terminal=False)
                        if use_progress
                        else None
                    )

                    def fail_report(**kwargs):
                        kwargs["output_path"].write_text("partial report")
                        raise error

                    with (
                        mock.patch(
                            "photos_backup.apple_photos.export.generate_late_photo_additions_report",
                            side_effect=fail_report,
                        ),
                        mock.patch(
                            "photos_backup.apple_photos.export.click.echo"
                        ) as echo,
                    ):
                        result = self.run_export(
                            FakeRunner([row("a.jpg", new=1)]),
                            state=ArchiveState(),
                            progress=progress,
                        )
                    self.assertTrue(result.complete)
                    self.assertTrue(result.state_advanced)
                    self.assertIsNone(result.summary().error)
                    self.assertIsNone(result.late_additions_path)
                    self.assertEqual(result.late_additions_rows, 0)
                    self.assertEqual(self.state.last_full_export_at, THURSDAY)
                    self.assertEqual(self.state.last_report_path, result.report_path)
                    if use_progress:
                        warnings = [line for line in messages if "Warning:" in line]
                        self.assertEqual(len(warnings), 1)
                        warning = warnings[0]
                    else:
                        warnings = [
                            call
                            for call in echo.call_args_list
                            if "Warning:" in call.args[0]
                        ]
                        self.assertEqual(len(warnings), 1)
                        self.assertTrue(warnings[0].kwargs["err"])
                        warning = warnings[0].args[0]
                    self.assertIn("may be partial", warning)
                    self.assertIn(str(error), warning)
                    self.assertIn(str(result.report_path.parent), warning)

    def test_supplementary_failure_does_not_mask_an_incomplete_export(self) -> None:
        for runner in (
            FakeRunner([row("a.jpg", error=1)]),
            FakeRunner([row("a.jpg", missing=1)]),
            FakeRunner(exit_code=1),
        ):
            with (
                self.subTest(runner=runner),
                mock.patch(
                    "photos_backup.apple_photos.export.generate_late_photo_additions_report",
                    side_effect=OSError("disk full"),
                ),
                mock.patch("photos_backup.apple_photos.export.click.echo"),
            ):
                baseline = ArchiveState(last_successful_export_at=LAST_MONDAY)
                result = self.run_export(runner, state=baseline)
                self.assertFalse(result.complete)
                self.assertFalse(result.state_advanced)
                self.assertEqual(self.state, baseline)
                self.assertIsNotNone(result.failure_reason())

    def test_unreadable_main_report_still_blocks_state_and_supplementary_report(self):
        def invalid_report(arguments):
            Path(arguments["report"]).write_bytes(b"filename,new\n\xff,1\n")
            return 0

        with mock.patch(
            "photos_backup.apple_photos.export.generate_late_photo_additions_report"
        ) as supplementary:
            result = self.run_export(invalid_report)
        supplementary.assert_not_called()
        self.assertFalse(result.complete)
        self.assertFalse(result.state_advanced)
        self.assertIn("could not be read", result.failure_reason())
        self.assertIsNone(result.late_additions_path)
        self.assertIsNone(self.state.last_successful_export_at)

    def test_supplementary_report_interruption_is_not_swallowed(self):
        with mock.patch(
            "photos_backup.apple_photos.export.generate_late_photo_additions_report",
            side_effect=KeyboardInterrupt,
        ):
            with self.assertRaises(KeyboardInterrupt):
                self.run_export(FakeRunner())

    def test_a_failing_exit_code_prevents_state_advancement(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1)], exit_code=1)

        result = self.run_export(runner)

        self.assertFalse(result.clean)
        self.assertIsNone(self.state.last_successful_export_at)
        self.assertIn("status 1", str(result.failure_reason()))

    def test_a_missing_report_is_a_failed_export(self) -> None:
        result = self.run_export(ReportlessRunner())

        self.assertFalse(result.clean)
        self.assertFalse(result.state_advanced)
        self.assertIsNone(self.state.last_successful_export_at)
        self.assertIsNone(self.state.last_full_export_at)
        self.assertIn("wrote no report", str(result.failure_reason()))

    def test_an_unrecognized_report_is_a_failed_export(self) -> None:
        result = self.run_export(ReportlessRunner("something,else\n1,2\n"))

        self.assertFalse(result.clean)
        self.assertFalse(result.state_advanced)
        self.assertIsNone(self.state.last_successful_export_at)
        self.assertIsNone(self.state.last_full_export_at)
        self.assertIn("no column this tool recognizes", str(result.failure_reason()))

    def test_an_empty_report_file_is_a_failed_export(self) -> None:
        result = self.run_export(ReportlessRunner(""))

        self.assertFalse(result.clean)
        self.assertFalse(result.state_advanced)
        self.assertIsNone(self.state.last_successful_export_at)

    def test_a_header_only_report_is_a_clean_no_op(self) -> None:
        result = self.run_export(FakeRunner([]))

        self.assertTrue(result.clean)
        self.assertTrue(result.state_advanced)
        self.assertEqual(result.files_transferred, 0)
        self.assertIsNone(result.failure_reason())
        self.assertEqual(self.state.last_successful_export_at, THURSDAY)
        self.assertEqual(self.state.last_full_export_at, THURSDAY)

    def test_a_reportless_second_run_never_reuses_the_first_report(self) -> None:
        first = self.run_export(FakeRunner([row("a.jpg", new=1)]))
        first_state = self.state

        second = self.run_export(ReportlessRunner())

        self.assertTrue(first.clean)
        self.assertFalse(second.clean)
        self.assertFalse(second.state_advanced)
        self.assertIn("wrote no report", str(second.failure_reason()))
        self.assertEqual(self.state, first_state)
        self.assertNotEqual(second.report_path, first.report_path)
        self.assertFalse(second.report_path.exists())
        self.assertTrue(first.report_path.is_file())

    def test_a_second_run_the_same_day_keeps_both_reports(self) -> None:
        first = self.run_export(FakeRunner([row("a.jpg", new=1)]))

        second = self.run_export(FakeRunner([row("b.jpg", new=1)]))

        self.assertTrue(second.clean)
        self.assertTrue(second.state_advanced)
        self.assertNotEqual(second.report_path, first.report_path)
        self.assertNotEqual(second.late_additions_path, first.late_additions_path)
        self.assertTrue(first.report_path.is_file())
        self.assertTrue(second.report_path.is_file())
        self.assertEqual(self.state.last_report_path, second.report_path)

    def test_missing_assets_preserve_state_and_retry_window(self) -> None:
        for state in (
            ArchiveState(),
            ArchiveState(
                last_full_export_at=LAST_MONDAY, last_successful_export_at=LAST_MONDAY
            ),
        ):
            with self.subTest(state=state):
                runner = FakeRunner([row("a.jpg", new=1), row("b.jpg", missing=1)])
                result = self.run_export(runner, state=state)
                self.assertTrue(result.clean)
                self.assertFalse(result.complete)
                self.assertFalse(result.state_advanced)
                self.assertEqual(self.state, state)
                self.assertIn("1 file(s) remain missing", result.failure_reason())
                self.assertEqual(result.summary().error, result.failure_reason())
                self.assertTrue(result.report_path.is_file())
                retry = self.run_export(FakeRunner([row("b.jpg", new=1)]))
                self.assertEqual(retry.plan, result.plan)
                self.assertTrue(retry.complete)
                self.assertTrue(retry.state_advanced)

    def test_extra_arguments_override_the_defaults(self) -> None:
        runner = FakeRunner()

        self.run_export(runner, extra_arguments={"limit": 5, "use_photokit": False})

        self.assertEqual(runner.arguments["limit"], 5)
        self.assertFalse(runner.arguments["use_photokit"])

    def test_custom_exports_preserve_baseline_and_keep_reports(self) -> None:
        baseline = ArchiveState(
            last_full_export_at=LAST_MONDAY,
            last_successful_export_at=LAST_MONDAY,
            last_report_path=self.archive / "previous.csv",
        )
        for options in (
            {"limit": 5},
            {"extra_arguments": {"limit": 5}},
            {"extra_arguments": {"album": "Holiday"}},
            {"extra_arguments": {"skip_movies": True}},
            {"extra_arguments": {"from_date": THURSDAY}},
        ):
            with self.subTest(options=options):
                result = self.run_export(
                    FakeRunner([row("a.jpg", new=1)]), state=baseline, **options
                )
                self.assertTrue(result.complete)
                self.assertFalse(result.state_advanced)
                self.assertEqual(self.state, baseline)
                self.assertTrue(result.report_path.is_file())
                self.assertEqual(result.files_transferred, 1)

    def test_custom_export_does_not_establish_a_first_full_baseline(self) -> None:
        result = self.run_export(FakeRunner(), extra_arguments={"limit": 5})
        self.assertFalse(result.state_advanced)
        self.assertIsNone(self.state.last_full_export_at)
        self.assertIsNone(self.state.last_successful_export_at)

    def test_the_summary_names_the_step_and_the_failure(self) -> None:
        runner = FakeRunner([row("a.jpg", error=1)])

        summary = self.run_export(runner).summary()

        self.assertEqual(summary.step_name, "Apple Photos")
        self.assertIn("export error", str(summary.error))
