from __future__ import annotations

import datetime
import json
import shlex
import unittest
from unittest import mock

from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.archive.paths import ArchivePaths
from photos_backup.archive.probes import real_hostname
from photos_backup.archive.state import ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.progress import ExportProgress
from tests.archive_case import ArchiveCommandTestCase
from tests.test_export import THURSDAY, FakeRunner, row


class RecentTests(ArchiveCommandTestCase):
    def run_recent(self, runner, *args):
        def export(config, archive, **kwargs):
            kwargs["runner"] = runner
            return ApplePhotosExport(
                config, archive, metadata_reader=lambda _: {}, **kwargs
            )

        with (
            self.mounted(),
            mock.patch("photos_backup.archive.Archive.now", return_value=THURSDAY),
            mock.patch(
                "photos_backup.cli.exporting.ApplePhotosExport", side_effect=export
            ),
        ):
            return self.runner.invoke(
                cli, ["--config", str(self.config_path), "recent", *args]
            )

    def test_recent_can_export_before_bootstrap_without_advancing_baseline(self):
        runner = FakeRunner([row("new.mov", new=1)])

        result = self.run_recent(runner)

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertEqual(
            runner.arguments["from_date"], THURSDAY - datetime.timedelta(days=30)
        )
        self.assertTrue(runner.arguments["download_missing"])
        self.assertTrue(runner.arguments["update"])
        self.assertFalse(runner.arguments["cleanup"])
        self.assertIn("120s per asset; no total run limit", result.output)
        paths = ArchivePaths(self.volume, self.volume / "Media" / "Apple Photos")
        state = ArchiveStateStore(paths).load()
        self.assertIsNone(state.initialized_at)
        self.assertIsNone(state.last_successful_export_at)
        self.assertIsNone(state.last_full_export_at)

    def test_recent_preserves_an_existing_full_export_baseline(self):
        paths = self.initialized_archive(
            writer_hostname=real_hostname(),
            last_full_export_at=THURSDAY,
            last_successful_export_at=THURSDAY,
        )
        before = paths.state_file.read_bytes()

        result = self.run_recent(FakeRunner([row("a.jpg", new=1)]), "--days", "7")

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertEqual(paths.state_file.read_bytes(), before)

    def test_recent_prints_a_writer_change_after_the_live_progress(self):
        takeover = mock.Mock(
            status=WriterStatus.CLAIMED,
            previous_hostname=None,
            hostname="this Mac.local",
            verdict=None,
            migration=None,
            deletion_candidates=(),
        )
        with mock.patch(
            "photos_backup.cli.exporting.ensure_writer", return_value=takeover
        ):
            result = self.run_recent(FakeRunner([row("a.jpg", new=1)]))

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertLess(
            result.output.index("Total command time"),
            result.output.index("Archive writer: 'this Mac.local'"),
        )
        self.assertLess(
            result.output.index("Archive writer:"),
            result.output.index("Export report:"),
        )

    def test_missing_files_leave_recent_backup_incomplete(self):
        result = self.run_recent(FakeRunner([row("missing.mov", missing=1)]))

        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("backup is incomplete", result.output)
        self.assertIn("Export report:", result.output)

    def test_status_can_retry_recent_export_with_recorded_options(self):
        self.run_recent(
            FakeRunner([row("missing.mov", missing=1)]),
            "--days",
            "7",
            "--download-timeout",
            "19",
        )
        with self.mounted():
            status = self.runner.invoke(
                cli, ["--config", str(self.config_path), "status"]
            )
            document = json.loads(
                self.runner.invoke(
                    cli, ["--config", str(self.config_path), "status", "--json"]
                ).stdout
            )
        self.assertEqual(status.exit_code, 0, status.output)
        hint = next(
            line.split("Retry export: ")[1]
            for line in status.stdout.splitlines()
            if "Retry export:" in line
        )
        self.assertEqual(
            shlex.split(hint),
            [
                "photos-backup",
                "--config",
                str(self.config_path),
                "recent",
                "--days",
                "7",
                "--download-timeout",
                "19",
            ],
        )
        self.assertIn("Missing files: 1", status.stdout)
        self.assertIn(".downloads.json", status.stdout)
        self.assertEqual(document["last_export_attempt"]["missing_count"], 1)

    def test_dry_run_does_not_export_or_create_archive(self):
        runner = FakeRunner()

        result = self.run_recent(runner, "--dry-run")

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIsNone(runner.arguments)
        self.assertEqual(list(self.volume.iterdir()), [])
        self.assertIn("Would export (recent)", result.output)

    def test_timeout_is_forwarded_to_the_real_runner(self):
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.exporting.run_osxphotos_export",
                side_effect=lambda arguments, **kwargs: FakeRunner()(arguments),
            ) as runner,
        ):
            result = self.runner.invoke(
                cli,
                [
                    "--config",
                    str(self.config_path),
                    "recent",
                    "--download-timeout",
                    "10",
                ],
            )

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertEqual(
            {k: v for k, v in runner.call_args.kwargs.items() if k != "progress"},
            {"download_timeout": 10, "local_first": True},
        )

        self.assertIsInstance(runner.call_args.kwargs["progress"], ExportProgress)

    def test_invalid_limits_are_rejected(self):
        for option in ("--days", "--download-timeout"):
            result = self.run_recent(FakeRunner(), option, "0")
            self.assertEqual(result.exit_code, 2, result.output)


if __name__ == "__main__":
    unittest.main()
