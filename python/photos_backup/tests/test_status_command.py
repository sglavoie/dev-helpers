from __future__ import annotations

import datetime
import json
import shlex
from unittest import mock

from photos_backup.archive import ArchiveLocked, ArchiveUnavailable
from photos_backup.archive.state import ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.summary import BackupSummary
from photos_backup.transfers import (
    TransferHistory,
)
from tests.archive_case import ArchiveCommandTestCase
from tests.test_verify import MONDAY, THURSDAY
from tests.timezones import pin_timezone


class StatusTests(ArchiveCommandTestCase):
    def setUp(self):
        super().setUp()
        # The full-export cadence follows local midnight; fixtures are in UTC.
        pin_timezone(self, "UTC")

    def test_unavailable_archive_still_shows_history_without_writes(self):
        history = TransferHistory(self.config_path)
        history.run(
            "Remote",
            self.root / "photos",
            "b2:photos",
            lambda: BackupSummary("Remote", files_transferred=2),
            dry_run=False,
        )
        before = {p: p.read_bytes() for p in history.directory.iterdir()}
        for error in (ArchiveUnavailable("Drive disconnected"), ArchiveLocked("Busy")):
            for as_json in (False, True):
                with (
                    self.subTest(error=error, as_json=as_json),
                    mock.patch(
                        "photos_backup.cli.status.open_archive", side_effect=error
                    ),
                ):
                    result = self.invoke_status(*(["--json"] if as_json else []))
                self.assertEqual(result.exit_code, error.exit_code, result.output)
                self.assertIn("b2:photos", result.stdout)
                if as_json:
                    document = json.loads(result.stdout)
                    self.assertIsNone(document["state"])
                    self.assertIsNone(document["next_export"])
                    self.assertEqual(document["archive_error"], str(error))
                    self.assertFalse(document["files_verified"])
                else:
                    self.assertIn("Archive unavailable", result.stdout)
        self.assertEqual(
            {p: p.read_bytes() for p in history.directory.iterdir()}, before
        )
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_json_status_is_one_document_with_iso_dates_and_no_writes(self):
        paths = self.initialized_archive(
            writer_hostname="test.local",
            pending_cleanup_run_id="run-7",
            last_full_export_at=MONDAY,
            last_successful_export_at=THURSDAY,
            last_report_path=self.root / "report.csv",
        )
        before = paths.state_file.read_bytes()
        with mock.patch("photos_backup.archive.Archive.now", return_value=THURSDAY):
            result = self.invoke_status("--json")
        self.assertEqual(result.exit_code, 0, result.output)
        document = json.loads(result.stdout)
        self.assertEqual(document["version"], 1)
        self.assertFalse(document["files_verified"])
        self.assertTrue(document["archive_configured"])
        self.assertEqual(
            document["state"]["last_successful_export_at"], THURSDAY.isoformat()
        )
        self.assertEqual(document["state"]["pending_cleanup_run_id"], "run-7")
        self.assertEqual(document["next_export"]["mode"], "incremental")
        self.assertEqual(document["transfers"], [])
        self.assertEqual(document["transfer_history_errors"], [])
        self.assertEqual(result.stderr, "")
        self.assertEqual(paths.state_file.read_bytes(), before)
        self.assertFalse(paths.lock_file.exists())
        self.assertFalse(self.history_root.exists())

    def test_json_status_of_uninitialized_archive_keeps_nulls(self):
        result = self.invoke_status("--json")
        self.assertEqual(result.exit_code, 0, result.output)
        document = json.loads(result.stdout)
        self.assertIsNone(document["state"]["initialized_at"])
        self.assertIsNone(document["state"]["last_report_path"])
        self.assertEqual(document["next_export"]["mode"], "full")
        self.assertFalse(self.history_root.exists())
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_status_shows_local_transfer_results_and_reports_corrupt_receipts(self):
        self.initialized_archive()
        history = TransferHistory(self.config_path)
        history.run(
            "Remote",
            self.root / "photos",
            "b2:photos",
            lambda: BackupSummary("Remote", files_transferred=2),
            dry_run=False,
        )
        result = self.invoke_status()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Last successful copy:", result.output)
        self.assertIn("b2:photos", result.output)
        document = json.loads(self.invoke_status("--json").stdout)
        self.assertEqual(
            document["transfers"][0]["last_success"]["files_transferred"], 2
        )
        receipt = next(history.directory.glob("*.json"))
        receipt.write_text("invalid")
        result = self.invoke_status("--json")
        self.assertEqual(result.exit_code, 0, result.output)
        document = json.loads(result.stdout)
        self.assertEqual(len(document["transfer_history_errors"]), 1)
        self.assertEqual(document["transfers"], [])
        self.assertEqual(receipt.read_text(), "invalid")

    def test_status_uses_existing_cadence_and_shows_latest_report(self):
        report = self.root / "export.csv"
        self.initialized_archive(
            last_full_export_at=MONDAY,
            last_successful_export_at=THURSDAY,
            last_report_path=report,
        )
        with mock.patch("photos_backup.archive.Archive.now", return_value=THURSDAY):
            result = self.invoke_status()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Next export: incremental", result.output)
        self.assertIn(
            "Next full export: Mon 2026-08-17 00:00 (in 3 days)", result.output
        )
        with mock.patch("photos_backup.archive.Archive.now", return_value=THURSDAY):
            short = self.invoke_status("--short").stdout
            document = json.loads(self.invoke_status("--json").stdout)
        self.assertIn("next export incremental (full Mon 2026-08-17)", short)
        self.assertEqual(
            document["next_export"]["full_due_at"], "2026-08-17T00:00:00+00:00"
        )
        self.assertIn(str(report), result.output)
        self.assertIn(
            f"{THURSDAY.astimezone():%Y-%m-%d %H:%M} (just now)", result.output
        )
        self.assertIn(
            f"{MONDAY.astimezone():%Y-%m-%d %H:%M} (3 days ago)", result.output
        )
        self.assertNotIn(THURSDAY.isoformat(), result.output)
        self.assertIn("Last archive cleanup reconciliation:", result.output)

    def test_check_suggests_daily_once_the_last_full_export_is_too_old(self):
        self.initialized_archive(
            last_full_export_at=MONDAY, last_successful_export_at=MONDAY
        )
        for days, overdue in ((6, False), (7, True)):
            now = MONDAY + datetime.timedelta(days=days)
            with mock.patch("photos_backup.archive.Archive.now", return_value=now):
                result = self.invoke_status("--check")
                document = json.loads(self.invoke_status("--json").stdout)
            self.assertIs(document["next_export"]["overdue"], overdue)
            self.assertEqual("daily export overdue" in result.stdout, overdue)
            daily = shlex.join(
                ["photos-backup", "--config", str(self.config_path), "daily"]
            )
            self.assertEqual(f"  {daily}\n" in result.stdout, overdue)

    def test_status_relative_ages_handle_units_and_future_timestamps(self):
        store = ArchiveStateStore(self.initialized_archive())
        for offset, expected in (
            (datetime.timedelta(seconds=59), "just now"),
            (datetime.timedelta(minutes=1), "1 minute ago"),
            (datetime.timedelta(minutes=2), "2 minutes ago"),
            (datetime.timedelta(hours=1), "1 hour ago"),
            (datetime.timedelta(days=1), "1 day ago"),
            (datetime.timedelta(seconds=-30), "in less than a minute"),
            (datetime.timedelta(hours=-2), "in 2 hours"),
        ):
            with self.subTest(expected=expected):
                timestamp = THURSDAY - offset
                store.update(last_successful_export_at=timestamp)
                with mock.patch(
                    "photos_backup.archive.Archive.now", return_value=THURSDAY
                ):
                    result = self.invoke_status()
                self.assertEqual(result.exit_code, 0, result.output)
                self.assertIn(
                    f"Last successful export: {timestamp.astimezone():%Y-%m-%d %H:%M} ({expected})",
                    result.output,
                )

    def invoke_status(self, *args):
        with (
            self.mounted(),
            mock.patch("photos_backup.apple_photos.adapter.PhotosProbes") as photos,
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "status", *args]
            )
        photos.assert_not_called()
        return result

    def test_new_archive_status_creates_nothing_and_suggests_bootstrap(self):
        result = self.invoke_status()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("(unclaimed)", result.output)
        self.assertIn(
            shlex.join(
                ["photos-backup", "--config", str(self.config_path), "bootstrap"]
            ),
            result.output,
        )
        self.assertIn("Next export: full", result.output)
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_status_shows_pending_cleanup_and_does_not_modify_state(self):
        paths = self.initialized_archive(
            writer_hostname="another-mac", pending_cleanup_run_id="run-7"
        )
        before = paths.state_file.read_bytes()
        result = self.invoke_status()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("another-mac", result.output)
        self.assertIn("approve-cleanup run-7", result.output)
        self.assertIn("approve-cleanup run-7 --discard", result.output)
        self.assertIn("files have not been verified", result.output)
        self.assertEqual(paths.state_file.read_bytes(), before)
        self.assertFalse(paths.lock_file.exists())
        self.assertFalse(paths.export_db.exists())

    def test_corrupt_state_is_reported_without_repair(self):
        paths = self.initialized_archive()
        paths.state_file.write_text("invalid")
        result = self.invoke_status()
        self.assertNotEqual(result.exit_code, 0)
        self.assertIn("not valid JSON", result.output)
        self.assertEqual(paths.state_file.read_text(), "invalid")
