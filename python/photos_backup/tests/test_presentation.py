from __future__ import annotations

import contextlib
import io
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import click
from click.testing import CliRunner

from photos_backup.apple_photos.verify import Check, VerificationReport
from photos_backup.cli.cli import cli
from photos_backup.remote.backup import _parse_rclone_stats
from photos_backup.summary import (
    BackupSummary,
    parse_rsync_stats,
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


class PreviewSummaryTests(unittest.TestCase):
    def setUp(self):
        self.enterContext(
            mock.patch(
                "photos_backup.cli.backup_all.which", return_value="/test/bin/tool"
            )
        )

    def test_copy_previews_never_claim_files_were_transferred(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / "config.toml"
            config.write_text(
                f'[sd_card]\nsource = "{root / "card"}"\ndestination = "{root / "sd"}"\n'
                f'[ssd]\nsource = "{root / "photos"}"\ndestination = "{root / "ssd"}"\n'
                f'[rclone]\nsource = "{root}"\nremote = "b2:photos"\n'
            )
            (root / "card").mkdir()
            (root / "photos").mkdir()
            (root / "sd").mkdir()
            completed = subprocess.CompletedProcess(
                [],
                0,
                stdout="Number of regular files transferred: 3\n"
                "Total transferred file size: 42 bytes\n"
                "Transferred: 42 bytes / 42 bytes, (xfr#3/3)\n",
            )
            for command in ("sd-card", "ssd", "remote", "backup-all"):
                with (
                    self.subTest(command=command),
                    mock.patch(
                        "photos_backup.remote.backup.shutil.which",
                        return_value="rclone",
                    ),
                    mock.patch(
                        "photos_backup.remote.backup.stream_command",
                        return_value=completed,
                    ),
                    mock.patch(
                        "photos_backup.sd_card.backup.stream_command",
                        return_value=completed,
                    ),
                    mock.patch(
                        "photos_backup.ssd.backup.stream_command",
                        return_value=completed,
                    ),
                ):
                    args = ["--config", str(config), command, "--dry-run"]
                    if command == "backup-all":
                        args.append("--skip-apple-photos")
                    result = CliRunner().invoke(cli, args)
                    self.assertEqual(result.exit_code, 0, result.output)
                    self.assertIn("DRY RUN", result.output)
                    self.assertIn("proposed transfers", result.output.lower())
                    self.assertNotIn("Files transferred:", result.output)
                    self.assertNotIn("Status: OK", result.output)
                    self.assertNotIn("ALL OK", result.output)
            self.assertFalse((root / "ssd").exists())


class TransferCountTests(unittest.TestCase):
    def test_absent_statistics_are_not_presented_as_a_successful_zero(self):
        for parse in (parse_rsync_stats, _parse_rclone_stats):
            self.assertIsNone(parse("unrecognized output")["files_transferred"])
        for count in (None, 0):
            for dry_run in (False, True):
                summary = BackupSummary(
                    "Remote", files_transferred=count, dry_run=dry_run
                )
                output = io.StringIO()
                with contextlib.redirect_stdout(output):
                    print_summary(summary)
                    print_pipeline_summary([summary])
                rendered = output.getvalue()
                if count is None:
                    self.assertIn("Transfer count: unavailable", rendered)
                    self.assertNotIn("transfers: 0", rendered)
                    self.assertNotIn("transferred: 0", rendered)
                else:
                    self.assertIn(
                        "Proposed transfers: 0" if dry_run else "Files transferred: 0",
                        rendered,
                    )
                    self.assertNotIn("unavailable", rendered)
