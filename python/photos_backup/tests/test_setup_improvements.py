import datetime
import io
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.cli.context import CliContext
from photos_backup.summary import print_transfer_history
from photos_backup.transfers import annotate_upstream_freshness
from tests.test_cli import ArchiveCommandTestCase
from tests.test_takeover import asset, write_export_db


class SetupImprovementsTests(unittest.TestCase):
    def test_doctor_collects_failures_without_writing_or_transferring(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / "config.toml"
            config.write_text(
                "[apple_photos]\nvolume = 42\n"
                f'[sd_card]\nsource = "{root}/missing-card"\ndestination = "{root}/new-copy"\n'
                f'exclude_file = "{root}/missing-exclude"\n'
                f'[rclone]\nsource = "{root}/missing-cloud-source"\nremote = "b2:photos"\n'
            )
            with (
                mock.patch("photos_backup.cli.doctor.shutil.which", return_value=None),
                mock.patch("photos_backup.cli.doctor.subprocess.run") as run,
            ):
                result = CliRunner().invoke(cli, ["--config", str(config), "doctor"])
            self.assertEqual(result.exit_code, 2, result.output)
            for expected in (
                "FAIL [apple_photos]",
                "missing-card",
                "missing-exclude",
                "missing-cloud-source",
                "FAIL rsync",
                "FAIL rclone",
            ):
                self.assertIn(expected, result.output)
            self.assertEqual(list(root.iterdir()), [config])
            run.assert_not_called()

    def test_doctor_healthy_copy_setup_only_queries_tool_version(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "card"
            source.mkdir()
            config = root / "config.toml"
            config.write_text(
                f'[sd_card]\nsource = "{source}"\ndestination = "{root}/new-copy"\n'
            )
            with (
                mock.patch(
                    "photos_backup.cli.doctor.shutil.which",
                    return_value="/usr/bin/rsync",
                ),
                mock.patch(
                    "photos_backup.cli.doctor.subprocess.run",
                    return_value=mock.Mock(stdout="rsync test\n"),
                ) as run,
            ):
                result = CliRunner().invoke(cli, ["--config", str(config), "doctor"])
            self.assertEqual(result.exit_code, 0, result.output)
            self.assertEqual(run.call_args.args[0], ["/usr/bin/rsync", "--version"])
            self.assertEqual(run.call_args.kwargs["timeout"], 5)
            self.assertFalse((root / "new-copy").exists())

    def test_doctor_separates_actions_from_failures_and_skips_dependent_checks(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "archive").mkdir()
            config = root / "config.toml"
            config.write_text(
                f'[sd_card]\nsource = "{root}/missing-card"\n'
                f'destination = "{root}/card-copy"\n'
                f'[ssd]\nsource = "{root}/archive"\ndestination = "{root}/ssd"\n'
            )
            with mock.patch(
                "photos_backup.cli.doctor._tool_version", return_value="test"
            ):
                result = CliRunner().invoke(cli, ["--config", str(config), "doctor"])
            self.assertEqual(result.exit_code, 3, result.output)
            self.assertIn("ACTION SD Card source", result.output)
            self.assertNotIn("FAIL", result.output)
            self.assertIn(
                "SKIP SD Card copy layout: needs the paths above", result.output
            )
            self.assertIn(
                f"SKIP SSD source {root}/card-copy: not created yet; run `photos-backup",
                result.output,
            )
            self.assertIn(
                "sd-card` before copying it to the SSD",
                result.output,
            )
            self.assertIn(
                f"PASS SSD destination {root}/ssd: created on the first copy",
                result.output,
            )
            self.assertFalse((root / "card-copy").exists())

    def test_version_needs_no_configuration(self):
        result = CliRunner().invoke(cli, ["--config", "/missing/config", "--version"])
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("photos-backup, version", result.output)

    def test_cloud_freshness_uses_upload_start_and_current_ssd_routes(self):
        older, newer = "2026-01-01T00:00:00+00:00", "2026-01-02T00:00:00+00:00"
        rows = [
            {
                "step": "SSD: All Photos",
                "source": "/archive",
                "destination": "/ssd/photos",
                "last_success": {"completed_at": newer},
            },
            {
                "step": "Remote",
                "source": "/ssd",
                "destination": "b2:photos",
                "last_success": {"started_at": older, "completed_at": newer},
            },
        ]
        with mock.patch(
            "pathlib.Path.resolve", side_effect=AssertionError("must not probe drives")
        ):
            self.assertTrue(
                annotate_upstream_freshness(rows)[1]["ssd_copied_since_upload"]
            )
            rows[1]["last_success"]["started_at"] = newer
            self.assertFalse(
                annotate_upstream_freshness(rows)[1]["ssd_copied_since_upload"]
            )
            rows[1]["source"] = "/unrelated"
            self.assertIsNone(
                annotate_upstream_freshness(rows)[1]["ssd_copied_since_upload"]
            )
            rows[1]["last_success"] = None
            self.assertIsNone(
                annotate_upstream_freshness(rows)[1]["ssd_copied_since_upload"]
            )

    def test_retry_hints_quote_config_and_preview_mirrors_only_on_current_routes(self):
        timestamp = "2026-01-01T00:00:00+00:00"
        row = {
            "step": "Remote",
            "source": "/ssd",
            "destination": "b2:photos",
            "last_success": None,
            "last_attempt": {
                "started_at": timestamp,
                "status": "failed",
                "mode": "mirror",
            },
        }
        with cli.make_context("photos-backup", [], resilient_parsing=True) as ctx:
            ctx.obj = CliContext(config_path=Path("/config with spaces.toml"))
            with mock.patch("sys.stdout", new_callable=io.StringIO) as output:
                print_transfer_history(
                    [row],
                    [],
                    now=datetime.datetime.fromisoformat(timestamp),
                    historical_receipts=[row],
                )
            text = output.getvalue()
        self.assertEqual(text.count("Preview mirror retry:"), 1)
        self.assertIn(
            "photos-backup --config '/config with spaces.toml' remote --delete --dry-run",
            text,
        )


class DoctorArchiveTests(ArchiveCommandTestCase):
    def test_archive_check_is_read_only_and_does_not_load_photos(self):
        paths = self.initialized_archive()
        before = {
            path: path.read_bytes() for path in self.volume.rglob("*") if path.is_file()
        }
        with (
            self.mounted(),
            mock.patch("photos_backup.cli.doctor._tool_version", return_value="test"),
            mock.patch("photos_backup.apple_photos.adapter.PhotosProbes") as photos,
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "doctor"]
            )
        self.assertIn("PASS Archive access: initialized", result.output)
        self.assertIn("Photos library directory", result.output)
        self.assertEqual(
            before,
            {
                path: path.read_bytes()
                for path in self.volume.rglob("*")
                if path.is_file()
            },
        )
        self.assertTrue(paths.state_file.exists())
        photos.assert_not_called()

    def test_export_database_newer_than_osxphotos_needs_action(self):
        paths = self.initialized_archive()
        write_export_db(paths.export_db, (asset("uuid-1", "guid-1"),), version="99.0")
        with (
            self.mounted(),
            mock.patch("photos_backup.cli.doctor._tool_version", return_value="test"),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "doctor"]
            )
        self.assertEqual(result.exit_code, 3, result.output)
        self.assertIn("ACTION Export database: schema version 99.0", result.output)
