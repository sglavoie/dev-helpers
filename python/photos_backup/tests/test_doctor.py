import datetime
import io
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.cli.context import CliContext
from photos_backup.status_report import print_transfer_history
from photos_backup.transfers import annotate_upstream_freshness
from tests.archive_case import ArchiveCommandTestCase
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
                "remove exclude_file from [sd_card]",
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


def _completed(stdout):
    return mock.Mock(stdout=stdout)


class DoctorTests(unittest.TestCase):
    def setUp(self):
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        self.root = Path(self._tmpdir.name)
        (self.root / "ssd").mkdir()
        self.config = self.root / "config.toml"

    def doctor(self, remote, listremotes, *arguments):
        self.config.write_text(
            f'[rclone]\nsource = "{self.root}/ssd"\nremote = "{remote}"\n'
        )

        def run(command, **kwargs):
            self.assertEqual(kwargs["timeout"], 5)
            self.assertIs(kwargs["stdin"], subprocess.DEVNULL)
            if command[1] == "--version":
                return _completed("rclone v1.70.0\n")
            self.assertEqual(command[1:], ["listremotes", "--ask-password=false"])
            if isinstance(listremotes, Exception):
                raise listremotes
            return _completed(listremotes)

        with (
            mock.patch(
                "photos_backup.cli.doctor.shutil.which",
                return_value="/opt/homebrew/bin/rclone",
            ),
            mock.patch(
                "photos_backup.cli.doctor.subprocess.run", side_effect=run
            ) as runner,
        ):
            result = CliRunner().invoke(
                cli, ["--config", str(self.config), "doctor", *arguments]
            )
        return result, runner

    def test_configured_rclone_remote_passes(self):
        result, _ = self.doctor("b2:photos", "local:\nb2:\n")
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("PASS rclone remote: 'b2:' is configured", result.output)

    def test_missing_rclone_remote_needs_action(self):
        result, _ = self.doctor("b2,hard_delete=true:photos", "local:\n")
        self.assertEqual(result.exit_code, 3, result.output)
        self.assertIn(
            "ACTION rclone remote: 'b2:' is not in the rclone configuration",
            result.output,
        )

    def test_unreadable_rclone_configuration_needs_action(self):
        error = subprocess.CalledProcessError(1, ["rclone", "listremotes"])
        result, _ = self.doctor("b2:photos", error)
        self.assertEqual(result.exit_code, 3, result.output)
        self.assertIn("set RCLONE_CONFIG_PASS", result.output)

    def test_on_the_fly_remote_needs_no_configuration(self):
        result, runner = self.doctor(":b2:photos", AssertionError("not called"))
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("SKIP rclone remote", result.output)
        self.assertEqual(runner.call_count, 1)

    def test_missing_rclone_skips_remote_lookup(self):
        self.config.write_text('[rclone]\nremote = "b2:photos"\n')
        with (
            mock.patch("photos_backup.cli.doctor.shutil.which", return_value=None),
            mock.patch("photos_backup.cli.doctor.subprocess.run") as run,
        ):
            result = CliRunner().invoke(cli, ["--config", str(self.config), "doctor"])
        self.assertIn("SKIP rclone remote: needs rclone above", result.output)
        run.assert_not_called()

    def test_json_reports_versions_checks_and_exit_code(self):
        result, _ = self.doctor("b2:photos", "local:\n", "--json")
        self.assertEqual(result.exit_code, 3, result.output)
        document = json.loads(result.output)
        self.assertEqual(
            set(document["versions"]), {"python", "photos_backup", "click", "osxphotos"}
        )
        self.assertEqual(document["exit_code"], 3)
        self.assertIn(
            {"status": "PASS", "check": "[rclone] configuration", "detail": ""},
            document["checks"],
        )
        self.assertIn(
            "ACTION",
            {check["status"] for check in document["checks"]},
        )

    def test_openrsync_first_on_path_is_named(self):
        (self.root / "card").mkdir()
        self.config.write_text(
            f'[sd_card]\nsource = "{self.root}/card"\n'
            f'destination = "{self.root}/copy"\n'
        )
        with (
            mock.patch(
                "photos_backup.cli.doctor.shutil.which", return_value="/usr/bin/rsync"
            ),
            mock.patch(
                "photos_backup.cli.doctor.subprocess.run",
                return_value=_completed("openrsync: protocol version 29\n"),
            ),
        ):
            result = CliRunner().invoke(cli, ["--config", str(self.config), "doctor"])
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("macOS built-in openrsync, not GNU rsync", result.output)
