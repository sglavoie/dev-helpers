from __future__ import annotations

import csv
import dataclasses
import datetime
import tempfile
import unittest
from pathlib import Path
from typing import Any

from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.plan import (
    DIRECTORY_TEMPLATE,
    FILENAME_TEMPLATE,
    RETRY_ATTEMPTS,
    SIDECAR_FORMATS,
    ExportMode,
    ExportPlan,
    cadence_start,
    export_arguments,
    plan_export,
)
from photos_backup.archive import ArchiveState, SystemProbes, open_archive
from photos_backup.config import ApplePhotosConfig

HOSTNAME = "Sebastiens MacBook.local"
SAFE_HOSTNAME = "Sebastiens-MacBook-local"
# 2026-08-13 is a Thursday; the most recent Monday is 2026-08-10.
THURSDAY = datetime.datetime(2026, 8, 13, 9, 30, tzinfo=datetime.UTC)
LAST_MONDAY = datetime.datetime(2026, 8, 10, 0, 0, tzinfo=datetime.UTC)

CSV_COLUMNS = ("filename", "new", "updated", "skipped", "missing", "error")


def make_config(volume: Path, archive: Path, **overrides: Any) -> ApplePhotosConfig:
    config = ApplePhotosConfig(
        volume=volume,
        archive=archive,
        library=Path("/Users/tester/Pictures/Photos Library.photoslibrary"),
        legacy_export=None,
        limit_export=25,
        spouse_device_models=("iPhone SE (2nd generation)",),
        incremental_overlap_days=14,
        full_export_weekday=0,
        full_export_max_age_days=7,
        mirror=True,
        cleanup_max_assets=10,
        cleanup_max_fraction=0.001,
    )
    return dataclasses.replace(config, **overrides)


class FakeRunner:
    """Stands in for osxphotos, recording arguments and writing a report."""

    def __init__(self, rows: list[dict[str, str]] | None = None, exit_code: int = 0):
        self.rows = rows if rows is not None else []
        self.exit_code = exit_code
        self.arguments: dict[str, Any] | None = None

    def __call__(self, arguments: dict[str, Any]) -> int:
        self.arguments = dict(arguments)
        report = Path(arguments["report"])
        report.parent.mkdir(parents=True, exist_ok=True)
        with report.open("w", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=CSV_COLUMNS)
            writer.writeheader()
            writer.writerows(self.rows)
        return self.exit_code


class ReportlessRunner:
    """Stands in for osxphotos exiting 0 without writing a usable report."""

    def __init__(self, contents: str | None = None, exit_code: int = 0):
        self.contents = contents
        self.exit_code = exit_code
        self.arguments: dict[str, Any] | None = None

    def __call__(self, arguments: dict[str, Any]) -> int:
        self.arguments = dict(arguments)
        if self.contents is not None:
            report = Path(arguments["report"])
            report.parent.mkdir(parents=True, exist_ok=True)
            report.write_text(self.contents)
        return self.exit_code


def row(filename: str, **flags: int) -> dict[str, str]:
    values = dict.fromkeys(CSV_COLUMNS, "0")
    values["filename"] = filename
    for name, flag in flags.items():
        values[name] = str(flag)
    return values


class PlanTests(unittest.TestCase):
    def setUp(self) -> None:
        self.config = make_config(Path("/Volumes/Test"), Path("/Volumes/Test/Media"))

    def plan(self, state: ArchiveState, now: datetime.datetime = THURSDAY):
        return plan_export(self.config, state, now)

    def test_cadence_start_is_midnight_of_the_configured_weekday(self) -> None:
        self.assertEqual(cadence_start(THURSDAY, 0), LAST_MONDAY)

    def test_cadence_start_on_the_weekday_itself_is_that_midnight(self) -> None:
        monday = datetime.datetime(2026, 8, 10, 23, 59, tzinfo=datetime.UTC)
        self.assertEqual(cadence_start(monday, 0), LAST_MONDAY)

    def test_a_never_exported_archive_runs_a_full_export(self) -> None:
        plan = self.plan(ArchiveState())

        self.assertIs(plan.mode, ExportMode.FULL)
        self.assertIsNone(plan.from_date)
        self.assertIn("no full export", plan.reason)

    def test_a_full_export_before_this_monday_runs_a_full_export(self) -> None:
        stale = datetime.datetime(2026, 8, 9, 23, 59, tzinfo=datetime.UTC)
        plan = self.plan(
            ArchiveState(last_full_export_at=stale, last_successful_export_at=stale)
        )

        self.assertIs(plan.mode, ExportMode.FULL)
        self.assertIn("2026-08-10", plan.reason)

    def test_an_elapsed_max_age_runs_a_full_export(self) -> None:
        self.config = make_config(
            Path("/Volumes/Test"),
            Path("/Volumes/Test/Media"),
            full_export_max_age_days=3,
        )
        monday = datetime.datetime(2026, 8, 10, 6, 0, tzinfo=datetime.UTC)
        plan = self.plan(
            ArchiveState(last_full_export_at=monday, last_successful_export_at=monday)
        )

        self.assertIs(plan.mode, ExportMode.FULL)
        self.assertIn("3 day(s) old", plan.reason)

    def test_a_recent_full_export_runs_an_incremental_export(self) -> None:
        monday = datetime.datetime(2026, 8, 10, 6, 0, tzinfo=datetime.UTC)
        wednesday = datetime.datetime(2026, 8, 12, 6, 0, tzinfo=datetime.UTC)
        plan = self.plan(
            ArchiveState(
                last_full_export_at=monday, last_successful_export_at=wednesday
            )
        )

        self.assertIs(plan.mode, ExportMode.INCREMENTAL)
        self.assertEqual(plan.from_date, wednesday - datetime.timedelta(days=14))

    def test_the_overlap_window_is_configurable(self) -> None:
        self.config = make_config(
            Path("/Volumes/Test"),
            Path("/Volumes/Test/Media"),
            incremental_overlap_days=2,
        )
        monday = datetime.datetime(2026, 8, 10, 6, 0, tzinfo=datetime.UTC)
        plan = self.plan(
            ArchiveState(last_full_export_at=monday, last_successful_export_at=monday)
        )

        self.assertEqual(plan.from_date, monday - datetime.timedelta(days=2))


class ArgumentTests(unittest.TestCase):
    def setUp(self) -> None:
        self.config = make_config(Path("/Volumes/Test"), Path("/Volumes/Test/Media"))

    def build(self, plan: ExportPlan, **overrides: Any) -> dict[str, Any]:
        arguments = {
            "dest": Path("/Volumes/Test/Media"),
            "export_db": Path("/Volumes/Test/Media/.photos-backup/export.db"),
            "report_path": Path("/Volumes/Test/Media/.photos-backup/reports/r.csv"),
        }
        arguments.update(overrides)
        return export_arguments(self.config, plan, **arguments)

    def test_full_export_arguments_are_exact(self) -> None:
        arguments = self.build(ExportPlan(ExportMode.FULL, "first run"))

        self.assertEqual(
            arguments,
            {
                "dest": "/Volumes/Test/Media",
                "db": "/Users/tester/Pictures/Photos Library.photoslibrary",
                "exportdb": "/Volumes/Test/Media/.photos-backup/export.db",
                "report": "/Volumes/Test/Media/.photos-backup/reports/r.csv",
                "directory": DIRECTORY_TEMPLATE,
                "filename_template": FILENAME_TEMPLATE,
                "sidecar": SIDECAR_FORMATS,
                "album_keyword": True,
                "exiftool": True,
                "export_aae": True,
                "download_missing": True,
                "use_photokit": True,
                "retry": RETRY_ATTEMPTS,
                "update": True,
                "update_errors": True,
                "skip_bursts": False,
                "skip_edited": False,
                "skip_live": False,
                "skip_original_if_edited": False,
                "skip_raw": False,
                "cleanup": False,
                "from_date": None,
                "dry_run": False,
                "verbose_flag": False,
                "limit": 0,
            },
        )

    def test_the_directory_template_carries_no_album(self) -> None:
        self.assertNotIn("album", DIRECTORY_TEMPLATE)
        self.assertEqual(DIRECTORY_TEMPLATE, "{created.year}/{created.mm}")

    def test_incremental_arguments_carry_the_from_date(self) -> None:
        from_date = datetime.datetime(2026, 7, 30, tzinfo=datetime.UTC)
        arguments = self.build(
            ExportPlan(ExportMode.INCREMENTAL, "since", from_date=from_date)
        )

        self.assertEqual(arguments["from_date"], from_date)

    def test_testing_arguments_stay_read_only_and_limited(self) -> None:
        arguments = self.build(
            ExportPlan(ExportMode.FULL, "first run"),
            dry_run=True,
            verbose=True,
            limit=25,
        )

        self.assertTrue(arguments["dry_run"])
        self.assertTrue(arguments["verbose_flag"])
        self.assertEqual(arguments["limit"], 25)


class ExportTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        # Resolved because macOS temporary directories live under symlinks.
        self.root = Path(self._tmpdir.name).resolve()
        self.volume = self.root / "SanDisk"
        self.volume.mkdir()
        self.archive = self.volume / "Media" / "Apple Photos"
        self.config = make_config(self.volume, self.archive)
        volume = self.volume
        self.probes = SystemProbes(
            is_mount=lambda path: path == volume,
            hostname=lambda: HOSTNAME,
            now=lambda: THURSDAY,
        )

    def run_export(
        self,
        runner: FakeRunner,
        *,
        state: ArchiveState | None = None,
        dry_run: bool = False,
        **kwargs: Any,
    ):
        with open_archive(self.config, dry_run=dry_run, probes=self.probes) as archive:
            if state is not None:
                archive.state_store.save(state)
            export = ApplePhotosExport(
                config=self.config,
                archive=archive,
                runner=runner,
                metadata_reader=lambda path: {},
                **kwargs,
            )
            result = export.export()
            self.state = archive.state_store.load()
            self.paths = archive.paths
        return result


class DirectExportTests(ExportTestCase):
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

    def test_missing_assets_are_reported_without_blocking_state(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1), row("b.jpg", missing=1)])

        result = self.run_export(runner)

        self.assertEqual(result.missing_count, 1)
        self.assertTrue(result.clean)
        self.assertEqual(self.state.last_successful_export_at, THURSDAY)

    def test_extra_arguments_override_the_defaults(self) -> None:
        runner = FakeRunner()

        self.run_export(runner, extra_arguments={"limit": 5, "use_photokit": False})

        self.assertEqual(runner.arguments["limit"], 5)
        self.assertFalse(runner.arguments["use_photokit"])

    def test_the_summary_names_the_step_and_the_failure(self) -> None:
        runner = FakeRunner([row("a.jpg", error=1)])

        summary = self.run_export(runner).summary()

        self.assertEqual(summary.step_name, "Apple Photos")
        self.assertIn("export error", str(summary.error))


class DryRunExportTests(ExportTestCase):
    def test_a_planning_only_dry_run_never_invokes_osxphotos(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1)])

        result = self.run_export(runner, dry_run=True, plan_only=True)

        self.assertIsNone(runner.arguments)
        self.assertFalse(result.performed)
        self.assertTrue(result.summary().planned)

    def test_a_dry_run_writes_nothing_into_the_archive(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1)])

        result = self.run_export(runner, dry_run=True, verbose=True, limit=25)

        self.assertTrue(runner.arguments["dry_run"])
        self.assertTrue(runner.arguments["verbose_flag"])
        self.assertEqual(runner.arguments["limit"], 25)
        self.assertFalse(result.state_advanced)
        self.assertEqual(result.late_additions_rows, 0)
        self.assertIsNone(result.late_additions_path)
        self.assertFalse(self.archive.exists())
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_a_dry_run_still_reports_the_planned_counts(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1), row("b.jpg", new=1)])

        result = self.run_export(runner, dry_run=True)

        self.assertEqual(result.files_transferred, 2)
        self.assertIs(result.plan.mode, ExportMode.FULL)

    def test_a_dry_run_report_is_discarded(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1)])

        result = self.run_export(runner, dry_run=True)

        self.assertFalse(result.report_path.exists())


if __name__ == "__main__":
    unittest.main()
