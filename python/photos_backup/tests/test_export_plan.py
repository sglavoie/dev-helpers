from __future__ import annotations

import dataclasses
import datetime
import unittest
from pathlib import Path
from typing import Any

from photos_backup.apple_photos.plan import (
    DIRECTORY_TEMPLATE,
    FILENAME_TEMPLATE,
    RETRY_ATTEMPTS,
    SIDECAR_FORMATS,
    ExportMode,
    ExportPlan,
    cadence_start,
    export_arguments,
    next_full_export_at,
    plan_export,
)
from photos_backup.archive import ArchiveState
from tests.export_case import (
    LAST_MONDAY,
    THURSDAY,
    make_config,
)
from tests.timezones import pin_timezone


class PlanTests(unittest.TestCase):
    def setUp(self) -> None:
        # The cadence follows local midnight; these fixtures are written in UTC.
        pin_timezone(self, "UTC")
        self.config = make_config(Path("/Volumes/Test"), Path("/Volumes/Test/Media"))

    def plan(self, state: ArchiveState, now: datetime.datetime = THURSDAY):
        return plan_export(self.config, state, now)

    def test_cadence_start_is_midnight_of_the_configured_weekday(self) -> None:
        self.assertEqual(cadence_start(THURSDAY, 0), LAST_MONDAY)

    def test_cadence_start_on_the_weekday_itself_is_that_midnight(self) -> None:
        monday = datetime.datetime(2026, 8, 10, 23, 59, tzinfo=datetime.UTC)
        self.assertEqual(cadence_start(monday, 0), LAST_MONDAY)

    def test_the_cadence_turns_over_at_local_not_utc_midnight(self) -> None:
        pin_timezone(self, "America/New_York")
        # Sunday 21:00 in UTC-4 is already Monday 01:00 in UTC.
        sunday_evening = datetime.datetime(
            2026, 8, 9, 21, 0, tzinfo=datetime.timezone(-datetime.timedelta(hours=4))
        )
        saturday = datetime.datetime(2026, 8, 8, 12, 0, tzinfo=datetime.UTC)

        self.assertEqual(
            cadence_start(sunday_evening, 0),
            datetime.datetime(
                2026, 8, 3, tzinfo=datetime.timezone(-datetime.timedelta(hours=4))
            ),
        )
        plan = self.plan(
            ArchiveState(
                last_full_export_at=saturday, last_successful_export_at=saturday
            ),
            now=sunday_evening.astimezone(datetime.UTC),
        )
        self.assertIs(plan.mode, ExportMode.INCREMENTAL)

    def test_next_full_export_is_where_the_plan_turns_full(self) -> None:
        tuesday = datetime.datetime(2026, 8, 11, 14, 0, tzinfo=datetime.UTC)
        state = ArchiveState(
            last_full_export_at=tuesday, last_successful_export_at=tuesday
        )
        next_monday = LAST_MONDAY + datetime.timedelta(days=7)
        for max_age, expected in (
            (7, next_monday),
            (3, tuesday + datetime.timedelta(days=3)),
        ):
            self.config = dataclasses.replace(
                self.config, full_export_max_age_days=max_age
            )
            due = next_full_export_at(self.config, state)
            self.assertEqual(due, expected)
            assert due is not None
            before = due - datetime.timedelta(seconds=1)
            self.assertIs(self.plan(state, before).mode, ExportMode.INCREMENTAL)
            self.assertIs(self.plan(state, due).mode, ExportMode.FULL)
        self.assertIsNone(next_full_export_at(self.config, ArchiveState()))

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
                "not_hidden": False,
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
