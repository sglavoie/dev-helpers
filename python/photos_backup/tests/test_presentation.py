from __future__ import annotations

import io
import unittest
from unittest import mock

import click

from photos_backup.apple_photos.verify import Check, VerificationReport
from photos_backup.summary import (
    BackupSummary,
    print_pipeline_summary,
    print_summary,
    print_verification_report,
)


class Terminal(io.StringIO):
    def isatty(self):
        return True


class PresentationTests(unittest.TestCase):
    def setUp(self):
        self.stream = Terminal()
        self.enterContext(mock.patch("sys.stdout", self.stream))
        self.enterContext(
            mock.patch.dict("os.environ", {"TERM": "xterm-256color", "COLUMNS": "120"})
        )

    def test_terminal_summary_keeps_literal_filenames_and_unknown_counts(self):
        print_summary(BackupSummary("Photos [red].jpg", dry_run=True))
        output = click.unstyle(self.stream.getvalue())
        self.assertIn("Photos [red].jpg", output)
        self.assertIn("DRY RUN", output)
        self.assertIn("unavailable", output)
        self.assertNotIn("Status: OK", output)
        self.assertIn("╭", output)

    def test_terminal_pipeline_distinguishes_previews_skips_and_failures(self):
        print_pipeline_summary(
            [
                BackupSummary("Photos", planned=True),
                BackupSummary("SD", skipped=True, skip_reason="not configured"),
                BackupSummary("SSD", error="Drive [red] missing", action_required=True),
                BackupSummary("Remote", error="Connection failed"),
            ]
        )
        output = click.unstyle(self.stream.getvalue())
        for expected in (
            "PLANNED",
            "SKIPPED",
            "not configured",
            "ACTION REQUIRED",
            "Drive [red] missing",
            "ERROR: Connection failed",
            "COMPLETED WITH ERRORS",
        ):
            self.assertIn(expected, output)
        self.assertNotIn("ALL OK", output)

    def test_verification_retains_failed_details_without_markup(self):
        print_verification_report(
            VerificationReport(
                (
                    Check("state", True, "Readable"),
                    Check("files", False, "Missing [red].jpg"),
                )
            )
        )
        output = click.unstyle(self.stream.getvalue())
        for expected in ("PASS", "FAIL", "Missing [red].jpg", "1 of 2 check(s) failed"):
            self.assertIn(expected, output)

    def test_dumb_terminal_uses_the_same_plain_summary_as_redirected_output(self):
        summary = BackupSummary("Photos", files_transferred=2)
        with mock.patch.dict("os.environ", {"TERM": "dumb"}):
            print_summary(summary)
        redirected = io.StringIO()
        with mock.patch("sys.stdout", redirected):
            print_summary(summary)
        self.assertEqual(self.stream.getvalue(), redirected.getvalue())
        self.assertNotIn("\x1b", redirected.getvalue())
        self.assertIn("Files transferred: 2", redirected.getvalue())
