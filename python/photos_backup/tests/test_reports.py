from __future__ import annotations

import dataclasses
import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.reports import prune_reports, report_run
from tests.test_export import ExportTestCase, FakeRunner, row


class PruneReportsTests(unittest.TestCase):
    def setUp(self) -> None:
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.reports = Path(directory.name)

    def run_files(self, run: str, age: int, *, downloads: bool = False) -> list[Path]:
        names = [f"photos_export_{run}.csv", f"late_photo_additions_{run}.csv"]
        if downloads:
            names.append(f"photos_export_{run}.downloads.json")
        paths = [self.reports / name for name in names]
        for path in paths:
            path.write_text("x")
            os.utime(path, (1_000_000 - age, 1_000_000 - age))
        return paths

    def test_report_run_groups_only_files_this_tool_writes(self):
        for name, expected in (
            ("photos_export_mac-local_2026-10-06.csv", "mac-local_2026-10-06"),
            (
                "photos_export_mac-local_2026-10-06_2.downloads.json",
                "mac-local_2026-10-06_2",
            ),
            (
                "late_photo_additions_mac-local_2026-10-06_2.csv",
                "mac-local_2026-10-06_2",
            ),
            ("notes.csv", None),
            ("photos_export_mac.txt", None),
        ):
            self.assertEqual(report_run(Path(name)), expected, name)

    def test_keeps_the_newest_runs_and_every_protected_run(self):
        newest = self.run_files("mac_2026-10-08", 0, downloads=True)
        middle = self.run_files("mac_2026-10-07", 10)
        baseline = self.run_files("mac_2026-10-01", 20)
        oldest = self.run_files("mac_2026-09-30", 30, downloads=True)
        stray = self.reports / "notes.csv"
        stray.write_text("keep me")

        removed, warnings = prune_reports(self.reports, 2, (baseline[0], None))

        self.assertEqual(warnings, [])
        self.assertEqual(sorted(removed), sorted(oldest))
        for path in (*newest, *middle, *baseline, stray):
            self.assertTrue(path.exists(), path)

    def test_symlinks_are_never_followed_or_removed(self):
        target = self.reports.parent / f"{self.reports.name}-target.csv"
        target.write_text("outside")
        self.addCleanup(target.unlink)
        link = self.reports / "photos_export_mac_2026-01-01.csv"
        link.symlink_to(target)
        self.run_files("mac_2026-10-08", 0)

        removed, _ = prune_reports(self.reports, 1, ())

        self.assertEqual(removed, [])
        self.assertTrue(link.is_symlink())
        self.assertEqual(target.read_text(), "outside")

    def test_a_failed_removal_warns_and_continues(self):
        self.run_files("mac_2026-10-08", 0)
        self.run_files("mac_2026-10-01", 10)
        with mock.patch.object(Path, "unlink", side_effect=OSError("read-only")):
            removed, warnings = prune_reports(self.reports, 1, ())
        self.assertEqual(removed, [])
        self.assertEqual(len(warnings), 2)
        self.assertIn("read-only", warnings[0])


class ExportReportRetentionTests(ExportTestCase):
    def old_reports(self) -> list[Path]:
        reports = self.archive / ".photos-backup" / "reports"
        reports.mkdir(parents=True)
        paths = [reports / f"photos_export_old_2026-01-0{day}.csv" for day in (1, 2)]
        for age, path in enumerate(paths):
            path.write_text("x")
            os.utime(path, (1_000_000 - age, 1_000_000 - age))
        return paths

    def test_a_clean_export_prunes_only_when_configured(self):
        old = self.old_reports()
        result = self.run_export(FakeRunner([row("a.jpg", new=1)]))
        self.assertTrue(result.state_advanced)
        self.assertTrue(all(path.exists() for path in old))

        self.config = dataclasses.replace(self.config, keep_reports=2)
        result = self.run_export(FakeRunner([row("b.jpg", new=1)]))

        self.assertTrue(result.report_path.exists())
        self.assertTrue(self.state.last_report_path.exists())
        self.assertEqual([path.exists() for path in old], [False, False])

    def test_a_failed_export_prunes_nothing(self):
        old = self.old_reports()
        self.config = dataclasses.replace(self.config, keep_reports=1)
        result = self.run_export(FakeRunner([row("a.jpg", missing=1)]))
        self.assertFalse(result.state_advanced)
        self.assertTrue(all(path.exists() for path in old))


if __name__ == "__main__":
    unittest.main()
