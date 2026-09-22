from __future__ import annotations

import datetime
import unittest
from unittest import mock

from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.archive.paths import ArchivePaths
from photos_backup.archive.probes import real_hostname
from photos_backup.archive.state import ArchiveStateStore
from photos_backup.cli.cli import cli
from tests.test_cli import ArchiveCommandTestCase
from tests.test_export import FakeRunner, THURSDAY, row


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
                "photos_backup.cli.recent.ApplePhotosExport", side_effect=export
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

    def test_missing_files_leave_recent_backup_incomplete(self):
        result = self.run_recent(FakeRunner([row("missing.mov", missing=1)]))

        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("backup is incomplete", result.output)
        self.assertIn("Export report:", result.output)

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
                "photos_backup.cli.recent.run_osxphotos_export",
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
            runner.call_args.kwargs, {"download_timeout": 10, "local_first": True}
        )

    def test_invalid_limits_are_rejected(self):
        for option in ("--days", "--download-timeout"):
            result = self.run_recent(FakeRunner(), option, "0")
            self.assertEqual(result.exit_code, 2, result.output)


if __name__ == "__main__":
    unittest.main()
