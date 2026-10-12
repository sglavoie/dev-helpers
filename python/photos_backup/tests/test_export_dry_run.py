from __future__ import annotations

import unittest

import click

from photos_backup.apple_photos.plan import (
    ExportMode,
)
from tests.export_case import (
    ExportTestCase,
    FakeRunner,
    row,
)


class DryRunExportTests(ExportTestCase):
    def test_archive_managed_overrides_are_rejected_by_export_service(self) -> None:
        runner = FakeRunner()
        with self.assertRaises(click.UsageError):
            self.run_export(runner, dry_run=True, extra_arguments={"cleanup": True})
        self.assertIsNone(runner.arguments)
        self.assertFalse(self.archive.exists())

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
