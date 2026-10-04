from __future__ import annotations

from unittest import mock

import click

from photos_backup.apple_photos.export import ARCHIVE_MANAGED_ARGUMENTS
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.plan import ExportMode, ExportPlan, ExportResult
from photos_backup.apple_photos.takeover import TakeoverCheck
from photos_backup.cli.cli import cli
from photos_backup.errors import ActionRequired
from photos_backup.summary import BackupSummary, print_export_result
from tests.test_cli import ArchiveCommandTestCase
from tests.test_export import FakeRunner


class ExportEntryPointTests(ArchiveCommandTestCase):
    def invoke(self, command, *arguments):
        if command == "backup-all":
            arguments = (*arguments, "--skip-sd-card", "--skip-ssd", "--skip-remote")
        return self.runner.invoke(
            cli, ["--config", str(self.config_path), command, *arguments]
        )

    def test_both_commands_check_writer_before_export_and_preserve_exit_three(self):
        for command, module in (
            ("apple-photos", "apple_photos"),
            ("backup-all", "backup_all"),
        ):
            with (
                self.subTest(command=command),
                self.mounted(),
                mock.patch(
                    f"photos_backup.cli.{module}.ensure_writer",
                    side_effect=ActionRequired("different library"),
                ),
                mock.patch(f"photos_backup.cli.{module}.ApplePhotosExport") as export,
            ):
                result = self.invoke(command, "--dry-run")
                self.assertEqual(result.exit_code, 3, result.output)
                self.assertIn("different library", result.output)
                export.assert_not_called()
                self.assertEqual(list(self.volume.iterdir()), [])

    def test_previews_check_writer_without_invoking_exporter_or_creating_archive(self):
        for command, module in (
            ("apple-photos", "apple_photos"),
            ("backup-all", "backup_all"),
        ):
            with (
                self.subTest(command=command),
                self.mounted(),
                mock.patch(
                    f"photos_backup.cli.{module}.ensure_writer",
                    return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
                ) as writer,
                mock.patch(
                    "photos_backup.apple_photos.export.run_osxphotos_export"
                ) as exporter,
            ):
                result = self.invoke(command, "--dry-run")
                self.assertEqual(result.exit_code, 0, result.output)
                self.assertIn("PLANNED", result.output)
                self.assertTrue(writer.call_args.args[1].dry_run)
                exporter.assert_not_called()
                self.assertEqual(list(self.volume.iterdir()), [])

    def test_testing_retains_limited_simulation_and_discards_temporary_report(self):
        with self.config_path.open("a") as config:
            config.write("limit_export = 25\n")
        exporter = FakeRunner()
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.apple_photos.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
            ),
            mock.patch(
                "photos_backup.apple_photos.export.run_osxphotos_export",
                side_effect=exporter,
            ),
        ):
            result = self.invoke("apple-photos", "--testing")
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertTrue(exporter.arguments["dry_run"])
        self.assertTrue(exporter.arguments["verbose_flag"])
        self.assertGreater(exporter.arguments["limit"], 0)
        self.assertNotIn("Export report:", result.output)
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_explicit_preview_takes_precedence_over_testing(self):
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.apple_photos.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
            ),
            mock.patch(
                "photos_backup.apple_photos.export.run_osxphotos_export"
            ) as exporter,
        ):
            result = self.invoke("apple-photos", "--testing", "--dry-run")
        self.assertEqual(result.exit_code, 0, result.output)
        exporter.assert_not_called()

    def test_reserved_overrides_are_rejected_before_opening_archive(self):
        for name in ARCHIVE_MANAGED_ARGUMENTS:
            flag = "--" + name.replace("_", "-")
            # --dry-run is a first-class Click option; its underscore spelling
            # exercises an attempted passthrough override instead.
            if name == "dry_run":
                flag = "--dry_run"
            for arguments in ([flag, "override"], [f"{flag}=override"]):
                with (
                    self.subTest(arguments=arguments),
                    mock.patch(
                        "photos_backup.cli.apple_photos.open_archive"
                    ) as archive,
                ):
                    result = self.invoke("apple-photos", *arguments)
                    self.assertEqual(result.exit_code, 2, result.output)
                    self.assertIn("Archive-managed", result.output)
                    archive.assert_not_called()

    def test_ordinary_overrides_still_reach_exporter(self):
        exporter = FakeRunner()
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.apple_photos.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.UNCHANGED, "test.local"),
            ),
            mock.patch(
                "photos_backup.apple_photos.export.run_osxphotos_export",
                side_effect=exporter,
            ),
        ):
            result = self.invoke(
                "apple-photos", "--testing", "--limit=5", "--use-photokit"
            )
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertEqual(exporter.arguments["limit"], 5)
        self.assertTrue(exporter.arguments["use_photokit"])

    def test_actual_pipeline_failure_takes_precedence_over_blocked_takeover(self):
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.backup_all.ensure_writer",
                side_effect=ActionRequired("different library"),
            ),
            mock.patch(
                "photos_backup.cli.backup_all._optional_step",
                return_value=[BackupSummary("SSD", error="copy failed")],
            ),
        ):
            result = self.invoke("backup-all", "--dry-run")
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("different library", result.output)
        self.assertIn("copy failed", result.output)

    def test_shared_summary_includes_report_for_both_success_and_failure(self):
        report = self.root / "export.csv"
        for exit_code in (0, 1):

            @click.command()
            def show():
                print_export_result(
                    ExportResult(
                        ExportPlan(ExportMode.FULL, "test"),
                        exit_code,
                        report_path=report,
                    )
                )

            result = self.runner.invoke(show)
            self.assertEqual(result.output.count(f"Export report: {report}"), 1)
