import datetime
import io
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import click
from click.testing import CliRunner

from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.takeover import TakeoverCheck
from photos_backup.cli.apple_photos import _parse_extra_args
from photos_backup.cli.backup_all import _export_apple_photos, backup_all
from photos_backup.cli.cli import cli
from photos_backup.cli.context import CliContext
from photos_backup.summary import (
    parse_rsync_stats,
    print_mirror_outcome,
    print_transfer_history,
)

# Captured from macOS /usr/bin/rsync (openrsync, protocol 29) with -ah --stats.
OPENRSYNC_STATS = """\
Number of files: 6
Number of files transferred: 4
Total file size: 1500 kB
Total transferred file size: 1500 kB
Unmatched data: 1500 kB
Matched data: 0 B
File list size: 190 B
Total sent: 1501 kB
Total received: 120 B

sent 1501k bytes  received 120 bytes  185M bytes/sec
total size is 1500k  speedup is 1.00
"""

GNU_RSYNC_STATS = """\
Number of files: 1,206 (reg: 1,204, dir: 2)
Number of created files: 6 (reg: 4, dir: 2)
Number of deleted files: 0
Number of regular files transferred: 1,204
Total file size: 1.50G bytes
Total transferred file size: 1.50G bytes
"""


class RsyncStatsTests(unittest.TestCase):
    def test_openrsync_statistics_are_parsed(self):
        self.assertEqual(
            parse_rsync_stats(OPENRSYNC_STATS),
            {"files_transferred": 4, "total_size": "1500 kB"},
        )

    def test_openrsync_without_human_readable_sizes(self):
        stats = parse_rsync_stats(
            "Number of files transferred: 0\nTotal transferred file size: 0 B\n"
        )
        self.assertEqual(stats, {"files_transferred": 0, "total_size": "0 B"})

    def test_gnu_rsync_plain_sizes(self):
        stats = parse_rsync_stats(
            "Number of regular files transferred: 2\n"
            "Total transferred file size: 1,500,006 bytes\n"
        )
        self.assertEqual(
            stats, {"files_transferred": 2, "total_size": "1,500,006 bytes"}
        )

    def test_gnu_rsync_human_readable_statistics(self):
        self.assertEqual(
            parse_rsync_stats(GNU_RSYNC_STATS),
            {"files_transferred": 1204, "total_size": "1.50G bytes"},
        )


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


class DetailedTransferRetryTests(unittest.TestCase):
    TIMESTAMP = "2026-01-01T00:00:00+00:00"

    def render(self, **changes):
        attempt = {"started_at": self.TIMESTAMP, "status": "succeeded", "mode": "copy"}
        row = {
            "step": "SSD: All Photos",
            "source": "/archive",
            "destination": "/ssd",
            "last_attempt": attempt,
            "last_success": {**attempt, "completed_at": self.TIMESTAMP},
        }
        for key, value in changes.items():
            if key in ("status", "mode"):
                attempt[key] = value
            else:
                row[key] = value
        with cli.make_context("photos-backup", [], resilient_parsing=True) as ctx:
            ctx.obj = CliContext(config_path=None)
            with mock.patch("sys.stdout", new_callable=io.StringIO) as output:
                print_transfer_history(
                    [row], [], now=datetime.datetime.fromisoformat(self.TIMESTAMP)
                )
        return output.getvalue()

    def test_fresh_copy_suggests_nothing(self):
        text = self.render(archive_exported_since_copy=False)
        self.assertNotIn("copy:", text.replace("Last successful copy:", ""))
        self.assertNotIn("Preview mirror", text)

    def test_stale_rows_suggest_an_update(self):
        for hint in (
            "archive_exported_since_copy",
            "ssd_copied_since_upload",
            "sd_imported_since_copy",
        ):
            with self.subTest(hint=hint):
                self.assertIn(
                    "Update copy: photos-backup ssd", self.render(**{hint: True})
                )

    def test_stale_mirror_suggests_a_preview(self):
        text = self.render(mode="mirror", archive_exported_since_copy=True)
        self.assertIn(
            "Preview mirror update: photos-backup ssd --delete --dry-run", text
        )

    def test_started_attempt_suggests_a_retry(self):
        self.assertIn("Retry copy: photos-backup ssd", self.render(status="started"))


class MirrorOutcomeTests(unittest.TestCase):
    def test_pending_cleanup_offers_preview_approve_and_discard(self):
        outcome = mock.Mock(
            reason="awaiting approval",
            reconciliation=None,
            manifest_path=Path("/archive/cleanup/run-7.json"),
            run_id="run-7",
        )
        outcome.status.value = "pending"
        with cli.make_context("photos-backup", [], resilient_parsing=True) as ctx:
            ctx.obj = CliContext(config_path=None)
            with mock.patch("sys.stdout", new_callable=io.StringIO) as output:
                print_mirror_outcome(outcome)
        lines = output.getvalue().splitlines()
        self.assertEqual(
            lines[-3:],
            [
                "  Preview with: photos-backup approve-cleanup run-7 --dry-run",
                "  Approve with: photos-backup approve-cleanup run-7",
                "  Discard with: photos-backup approve-cleanup run-7 --discard",
            ],
        )


class ForwardedOsxphotosOptionTests(unittest.TestCase):
    def test_options_are_typed_by_osxphotos(self):
        self.assertEqual(
            _parse_extra_args(
                [
                    "--album",
                    "2024",
                    "--album=Holiday",
                    "--from-date",
                    "2024-01-02",
                    "--limit",
                    "5",
                    "-V",
                    "-V",
                    "--use-photokit",
                    "--sidecar",
                    "xmp",
                ]
            ),
            {
                "album": ("2024", "Holiday"),
                "from_date": datetime.datetime(2024, 1, 2),
                "limit": 5,
                "verbose_flag": 2,
                "use_photokit": True,
                "sidecar": ("xmp",),
            },
        )

    def test_only_given_options_are_returned(self):
        self.assertEqual(_parse_extra_args([]), {})
        self.assertEqual(_parse_extra_args(["--only-new"]), {"only_new": True})

    def test_invalid_options_are_usage_errors(self):
        for arguments, message in (
            (["--bogus"], "No such option"),
            (["--limit", "many"], "not a valid integer"),
            (["--limit"], "requires an argument"),
        ):
            with self.subTest(arguments=arguments):
                with self.assertRaises(click.UsageError) as raised:
                    _parse_extra_args(arguments)
                self.assertIn("osxphotos export:", raised.exception.format_message())
                self.assertIn(message, raised.exception.format_message())
        with self.assertRaisesRegex(click.UsageError, "Unexpected argument: photos"):
            _parse_extra_args(["photos"])

    def test_aliases_of_archive_managed_options_are_refused(self):
        with self.assertRaises(click.UsageError) as raised:
            _parse_extra_args(["--library", "/elsewhere.photoslibrary"])
        self.assertIn("Archive-managed option(s)", raised.exception.format_message())
        self.assertIn("--db", raised.exception.format_message())


class BackupAllTweaksTests(unittest.TestCase):
    def test_help_shows_only_the_current_ssd_deletion_flag(self):
        result = CliRunner().invoke(cli, ["backup-all", "--help"])
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("--delete-ssd", result.output)
        options = {param.name: param for param in backup_all.params}
        self.assertTrue(options["delete_alias"].hidden)
        self.assertIn("absent from the source", options["delete_remote"].help)
        self.assertNotIn("--delete ", result.output)
        self.assertNotIn("--delete,", result.output)

    def test_takeover_is_printed_after_the_live_status_line(self):
        events = []

        class Progress:
            def __enter__(self):
                events.append("progress started")
                return mock.MagicMock()

            def __exit__(self, *_):
                events.append("progress finished")

        config = mock.Mock(limit_export=25)
        with (
            mock.patch("photos_backup.cli.backup_all.ExportProgress", Progress),
            mock.patch("photos_backup.cli.backup_all.open_archive"),
            mock.patch(
                "photos_backup.cli.backup_all.ensure_writer",
                return_value=TakeoverCheck(WriterStatus.CLAIMED, "this.local"),
            ),
            mock.patch("photos_backup.cli.backup_all.ApplePhotosExport") as exporter,
            mock.patch(
                "photos_backup.cli.backup_all.print_takeover_check",
                side_effect=lambda *_, **__: events.append("takeover"),
            ),
            mock.patch("photos_backup.cli.backup_all.print_export_result"),
        ):
            _export_apple_photos(config, dry_run=True, download_timeout=5)
        self.assertEqual(events, ["progress started", "progress finished", "takeover"])
        self.assertNotIn("verbose", exporter.call_args.kwargs)
        self.assertNotIn("limit", exporter.call_args.kwargs)
        self.assertTrue(exporter.call_args.kwargs["plan_only"])


class ApplePhotosHelpTests(unittest.TestCase):
    def test_usage_shows_forwarded_options_and_an_example(self):
        result = CliRunner().invoke(cli, ["apple-photos", "--help"])
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("[OSXPHOTOS EXPORT OPTIONS]...", result.output)
        self.assertIn("apple-photos --album Holiday", result.output)


if __name__ == "__main__":
    unittest.main()
