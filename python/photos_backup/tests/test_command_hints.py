from __future__ import annotations

import shlex
import unittest
from unittest import mock

from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.plan import ExportMode, ExportPlan, ExportResult
from photos_backup.cli.backup_all import backup_all
from photos_backup.cli.cli import cli
from photos_backup.cli.context import suggested_command
from photos_backup.cli.context import export_retry_arguments
from tests.test_cli import ArchiveCommandTestCase


class CommandHintTests(ArchiveCommandTestCase):
    def setUp(self):
        super().setUp()
        new_config = self.root / "my photos' settings.toml"
        self.config_path.rename(new_config)
        self.config_path = new_config
        self.volume = self.root / "my photos' volume"
        self.volume.mkdir()
        self.options = ["--config", str(self.config_path), "--volume", str(self.volume)]

    def test_pipeline_retry_only_repeats_export_and_preserves_timeout(self):
        with backup_all.make_context(
            "backup-all", ["--download-timeout", "19", "--delete-remote"]
        ):
            arguments = export_retry_arguments()
        self.assertEqual(
            arguments,
            [
                "backup-all",
                "--skip-sd-card",
                "--skip-ssd",
                "--skip-remote",
                "--download-timeout",
                "19",
            ],
        )

    def test_status_bootstrap_hint_preserves_quoted_archive_options(self):
        result = self.runner.invoke(cli, [*self.options, "status"])
        self.assertEqual(result.exit_code, 0, result.output)
        hint = next(
            line.split("Next step: ", 1)[1]
            for line in result.output.splitlines()
            if "Next step:" in line
        )
        self.assertEqual(
            shlex.split(hint), ["photos-backup", *self.options, "bootstrap"]
        )
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_status_cleanup_hint_targets_the_same_archive_when_pasted(self):
        self.initialized_archive(pending_cleanup_run_id="run-7")
        result = self.runner.invoke(cli, [*self.options, "status"])
        self.assertEqual(result.exit_code, 0, result.output)
        hint = next(
            line.split("Approve with: ", 1)[1]
            for line in result.output.splitlines()
            if "Approve with:" in line
        )
        arguments = shlex.split(hint)
        self.assertEqual(
            arguments, ["photos-backup", *self.options, "approve-cleanup", "run-7"]
        )
        discard_hint = next(
            line.split("Discard with: ", 1)[1]
            for line in result.output.splitlines()
            if "Discard with:" in line
        )
        self.assertEqual(shlex.split(discard_hint), [*arguments, "--discard"])
        preview_hint = next(
            line.split("Preview with: ", 1)[1]
            for line in result.output.splitlines()
            if "Preview with:" in line
        )
        self.assertEqual(shlex.split(preview_hint), [*arguments, "--dry-run"])
        # Exercise Click parsing of the pasted command without approving anything.
        with mock.patch("photos_backup.cli.approve_cleanup.open_archive") as opened:
            opened.side_effect = RuntimeError("stop before approval")
            self.runner.invoke(cli, arguments[1:])
        self.assertEqual(
            opened.call_args.args[0].archive, self.volume / "Media" / "Apple Photos"
        )

    def test_daily_bootstrap_error_preserves_options(self):
        result = self.runner.invoke(cli, [*self.options, "daily", "--dry-run"])
        self.assertEqual(result.exit_code, 3, result.output)
        self.assertIn(
            shlex.join(["photos-backup", *self.options, "bootstrap"]), result.output
        )

    def test_daily_pending_cleanup_preserves_options_in_summary_and_error(self):
        self.initialized_archive(pending_cleanup_run_id="run-7")
        export = ExportResult(
            ExportPlan(ExportMode.FULL, "weekly"), 0, state_advanced=True
        )
        with (
            mock.patch(
                "photos_backup.cli.daily.ensure_writer",
                return_value=mock.Mock(status=WriterStatus.UNCHANGED),
            ),
            mock.patch("photos_backup.cli.daily.ApplePhotosExport") as exporter,
        ):
            exporter.return_value.export.return_value = export
            result = self.runner.invoke(cli, [*self.options, "daily"])
        self.assertEqual(result.exit_code, 3, result.output)
        hint = shlex.join(["photos-backup", *self.options, "approve-cleanup", "run-7"])
        self.assertIn(f"  Preview with: {hint} --dry-run\n", result.output)
        self.assertIn(f"  Approve with: {hint}\n", result.output)
        self.assertIn(f"and preview with `{hint} --dry-run`", result.output)

    def test_verify_recovery_hint_preserves_options(self):
        result = self.runner.invoke(cli, [*self.options, "verify"])
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn(
            shlex.join(["photos-backup", *self.options, "bootstrap"]), result.output
        )


class PlainCommandHintTests(unittest.TestCase):
    def test_non_cli_call_has_no_archive_options(self):
        self.assertEqual(suggested_command("bootstrap"), "photos-backup bootstrap")
