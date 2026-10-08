from __future__ import annotations

import datetime
import io
import json
import shlex
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.download_report import summarize_downloads
from photos_backup.apple_photos.verify import Check, VerificationReport
from photos_backup.archive import ArchiveLocked, ArchiveUnavailable
from photos_backup.archive.state import ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.cli.context import CliContext
from photos_backup.status_report import print_transfer_history
from photos_backup.summary import BackupSummary
from photos_backup.transfers import (
    TransferHistory,
    annotate_archive_freshness,
    annotate_upstream_freshness,
)
from photos_backup.verification_history import (
    VerificationHistory,
    annotate_verification,
)
from tests.archive_case import ArchiveCommandTestCase
from tests.test_verify import MONDAY, THURSDAY
from tests.timezones import pin_timezone

EARLY = datetime.datetime(2026, 8, 10, tzinfo=datetime.UTC)
LATE = EARLY + datetime.timedelta(days=1)
TIMEOUT_COUNT = 3


def transfer(step, source, destination, when=EARLY):
    success = {
        "started_at": when.isoformat(),
        "completed_at": when.isoformat(),
        "status": "succeeded",
    }
    return {
        "step": step,
        "source": source,
        "destination": destination,
        "last_attempt": success,
        "last_success": success,
    }


class FreshnessTests(unittest.TestCase):
    def test_sd_import_requires_matching_current_route_and_compares_copy_start(self):
        card = transfer("SD Card", "/card", "/raw/card", LATE)
        ssd = transfer("SSD: SD Card", "/raw", "/ssd/raw")
        ssd["last_success"]["completed_at"] = (
            LATE + datetime.timedelta(hours=1)
        ).isoformat()
        with mock.patch(
            "pathlib.Path.resolve", side_effect=AssertionError("no probing")
        ):
            result = annotate_upstream_freshness([card, ssd])
        self.assertTrue(result[1]["sd_imported_since_copy"])
        for when, expected in (
            (EARLY, False),
            (EARLY - datetime.timedelta(days=1), False),
        ):
            card["last_success"]["completed_at"] = when.isoformat()
            self.assertIs(
                annotate_upstream_freshness([card, ssd])[1]["sd_imported_since_copy"],
                expected,
            )
        card["destination"] = "/unrelated/card"
        self.assertIsNone(
            annotate_upstream_freshness([card, ssd])[1]["sd_imported_since_copy"]
        )
        card["destination"] = "/raw/card"
        ssd["last_success"] = None
        self.assertIsNone(
            annotate_upstream_freshness([card, ssd])[1]["sd_imported_since_copy"]
        )

    def test_incomplete_export_is_possible_change_without_advancing_success(self):
        row = transfer("SSD: All Photos", "/archive", "/ssd/archive")
        for status in ("failed", "interrupted", "started"):
            attempt = {
                "status": status,
                "started_at": LATE.isoformat(),
                "completed_at": None,
            }
            result = annotate_archive_freshness(
                [row], Path("/archive"), EARLY, attempt
            )[0]
            self.assertFalse(result["archive_exported_since_copy"])
            self.assertTrue(result["archive_may_have_changed_since_copy"])
            attempt["started_at"] = EARLY.isoformat()
            self.assertFalse(
                annotate_archive_freshness([row], Path("/archive"), EARLY, attempt)[0][
                    "archive_may_have_changed_since_copy"
                ]
            )
        row["source"] = "/parent"
        self.assertIsNone(
            annotate_archive_freshness([row], Path("/archive"), EARLY, attempt)[0][
                "archive_may_have_changed_since_copy"
            ]
        )


class StatusPolishTests(ArchiveCommandTestCase):
    def invoke(self, *args):
        with self.mounted():
            return self.runner.invoke(cli, ["--config", str(self.config_path), *args])

    def attempt(self, paths, **changes):
        document = {
            "version": 1,
            "mode": "recent",
            "status": "failed",
            "hostname": "test",
            "started_at": LATE.isoformat(),
            "completed_at": LATE.isoformat(),
            "restricted": False,
            "baseline_advanced": False,
            "report_path": ".photos-backup/reports/latest.csv",
            "error": "missing files",
            "download_report_path": ".photos-backup/reports/latest.downloads.json",
            "retry_arguments": ["recent", "--days", "60", "--download-timeout", "300"],
            **changes,
        }
        (paths.metadata / "last-export-attempt.json").write_text(json.dumps(document))
        return {
            **document,
            "download_report_path": str(
                paths.archive / document["download_report_path"]
            ),
        }

    def test_short_failed_export_downloads_cleanup_and_context(self):
        paths = self.initialized_archive(
            last_successful_export_at=EARLY, pending_cleanup_run_id="run-7"
        )
        attempt = self.attempt(paths)
        report = Path(attempt["download_report_path"])
        report.parent.mkdir()
        report.write_text(
            json.dumps(
                [
                    {
                        "filename": f"image-{i}.jpg",
                        "reason": "download timed out"
                        if i < TIMEOUT_COUNT
                        else "PhotoKit error",
                    }
                    for i in range(4)
                ]
            )
        )
        before = {p: p.read_bytes() for p in self.volume.rglob("*") if p.is_file()}
        for args in (("status",), ("status", "--short")):
            result = self.invoke(*args)
            self.assertEqual(result.exit_code, 0, result.output)
            self.assertIn("3 timed out; 1 retrieval error", result.stdout)
            self.assertIn("image-0.jpg", result.stdout)
            self.assertNotIn("image-3.jpg", result.stdout)
            self.assertIn("and 1 more", result.stdout)
            self.assertIn("recent --days 60 --download-timeout 300", result.stdout)
        short = self.invoke("status", "--short").stdout
        self.assertIn("latest export incomplete", short)
        self.assertIn("Cleanup: pending run-7", short)
        self.assertIn("approve-cleanup run-7 --dry-run", short)
        self.assertIn(f"--config {self.config_path}", short)
        document = json.loads(self.invoke("status", "--json").stdout)
        self.assertEqual(document["download_summary"]["count"], 4)
        self.assertEqual(len(document["download_summary"]["samples"]), 3)
        self.assertEqual(
            before, {p: p.read_bytes() for p in self.volume.rglob("*") if p.is_file()}
        )
        self.assertFalse(self.history_root.parent.joinpath("verifications").exists())

    def test_download_report_absence_corruption_and_symlinks_are_tolerated(self):
        paths = self.initialized_archive()
        attempt = self.attempt(paths)
        report = Path(attempt["download_report_path"])
        self.assertEqual(summarize_downloads(attempt, paths.archive), (None, None))
        report.parent.mkdir()
        for invalid in (
            "invalid",
            "{}",
            '[{"filename": 3, "reason": "error"}]',
            '[{"filename": "x"}]',
        ):
            report.write_text(invalid)
            result = self.invoke("status", "--json")
            self.assertEqual(result.exit_code, 0, result.output)
            self.assertEqual(result.stderr, "")
            document = json.loads(result.stdout)
            self.assertIsNone(document["download_summary"])
            self.assertIn(
                "Could not read download report", document["download_report_error"]
            )
            self.assertEqual(report.read_text(), invalid)
        report.unlink()
        report.symlink_to(self.config_path)
        self.assertIn("symlinks", summarize_downloads(attempt, paths.archive)[1])
        attempt["status"] = "succeeded"
        self.assertEqual(summarize_downloads(attempt, paths.archive), (None, None))

    def test_short_copy_only_unknown_freshness_and_safe_mirror_retry(self):
        self.config_path.write_text(
            f'[rclone]\nsource = "{self.root}/offline"\nremote = "b2:photos"\n'
        )
        history = TransferHistory(self.config_path)
        history.run(
            "Remote",
            self.root / "offline",
            "b2:photos",
            lambda: BackupSummary("Remote", error="failed"),
            dry_run=False,
            delete_at_destination=True,
        )
        result = self.invoke("status", "--short")
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Apple Photos: not configured", result.stdout)
        self.assertIn("Remote: latest attempt failed", result.stdout)
        self.assertIn("remote --delete --dry-run", result.stdout)
        history.run(
            "Remote",
            self.root / "offline",
            "b2:photos",
            lambda: BackupSummary("Remote"),
            dry_run=False,
        )
        self.assertIn("freshness unknown", self.invoke("status", "--short").stdout)
        self.assertFalse((self.root / "offline").exists())

    def test_short_refuses_json_and_preserves_unavailable_archive_exit(self):
        self.assertEqual(self.invoke("status", "--short", "--json").exit_code, 2)
        with mock.patch(
            "photos_backup.cli.status.open_archive",
            side_effect=ArchiveUnavailable("disconnected"),
        ):
            result = self.invoke("status", "--short")
        self.assertEqual(result.exit_code, ArchiveUnavailable.exit_code)
        self.assertIn("Apple Photos: unavailable", result.stdout)
        self.assertNotIn("verify --record", result.stdout)

    def test_check_exits_three_only_when_a_command_is_suggested(self):
        result = self.invoke("status", "--check")
        self.assertEqual(result.exit_code, 3)
        self.assertIn("bootstrap incomplete", result.stdout)
        self.assertIn("Next: ", result.output)
        self.assertIn("bootstrap", result.output)

        with mock.patch("photos_backup.cli.status.print_short_status", return_value=[]):
            self.assertEqual(self.invoke("status", "--check").exit_code, 0)
        self.assertEqual(self.invoke("status", "--check", "--json").exit_code, 2)

    def test_short_and_json_show_new_freshness_hints(self):
        paths = self.initialized_archive(last_successful_export_at=EARLY)
        self.attempt(paths)
        with self.config_path.open("a") as config:
            config.write(
                f'\n[sd_card]\nsource = "{self.root}/card"\ndestination = "{self.root}/raw"\n[ssd]\nsource = "{paths.archive}"\ndestination = "{self.root}/ssd"\n'
            )
        history = TransferHistory(self.config_path)
        for name, source, destination, when in (
            (
                "SSD: All Photos",
                paths.archive,
                self.root / "ssd" / paths.archive.name,
                EARLY,
            ),
            ("SSD: SD Card", self.root / "raw", self.root / "ssd/raw", EARLY),
            ("SD Card", self.root / "card", self.root / "raw/card", LATE),
        ):
            with mock.patch(
                "photos_backup.transfers._now", return_value=when.isoformat()
            ):
                history.run(
                    name,
                    source,
                    destination,
                    lambda: BackupSummary(name),
                    dry_run=False,
                )
        short = self.invoke("status", "--short").stdout
        self.assertIn("SSD: All Photos: may need updating", short)
        self.assertIn("SSD: SD Card: needs updating", short)
        detailed = self.invoke("status").stdout
        self.assertIn("Archive may have changed", detailed)
        self.assertIn("SD-card import completed", detailed)
        rows = {
            row["step"]: row
            for row in json.loads(self.invoke("status", "--json").stdout)[
                "configured_transfers"
            ]
        }
        self.assertTrue(rows["SSD: All Photos"]["archive_may_have_changed_since_copy"])
        self.assertTrue(rows["SSD: SD Card"]["sd_imported_since_copy"])

    def test_short_bootstrap_unknown_routes_and_successful_recent_are_explicit(self):
        result = self.invoke("status", "--short")
        self.assertIn("bootstrap incomplete", result.stdout)
        self.assertIn("bootstrap", result.stdout)
        paths = self.initialized_archive(last_successful_export_at=EARLY)
        self.attempt(paths, status="succeeded", error=None)
        result = self.invoke("status", "--short")
        self.assertIn(
            "latest recent export succeeded (baseline unchanged)", result.stdout
        )
        self.assertNotIn("latest export incomplete", result.stdout)
        with self.config_path.open("a") as config:
            config.write(
                f'\n[rclone]\nsource = "{self.root}/offline"\nremote = "b2:photos"\n'
            )
        result = self.invoke("status", "--short")
        self.assertIn("no successful copy recorded; freshness unknown", result.stdout)

    def test_parent_report_symlink_and_directory_are_rejected(self):
        paths = self.initialized_archive()
        attempt = self.attempt(paths)
        report = Path(attempt["download_report_path"])
        report.parent.symlink_to(self.root)
        self.assertIn("symlinks", summarize_downloads(attempt, paths.archive)[1])
        report.parent.unlink()
        report.mkdir(parents=True)
        self.assertIn("regular file", summarize_downloads(attempt, paths.archive)[1])


class VerificationHistoryTests(ArchiveCommandTestCase):
    invoke = StatusPolishTests.invoke

    def test_record_is_opt_in_handles_failed_checks_and_keeps_json_clean(self):
        paths = self.initialized_archive(last_successful_export_at=EARLY)
        history = VerificationHistory(self.config_path, paths.archive)
        before = paths.state_file.read_bytes()
        for passed, name, code in (
            (True, "state", 0),
            (False, "pending cleanup", 3),
            (False, "missing assets", 1),
        ):
            report = VerificationReport((Check(name, passed, "finding"),))
            with mock.patch(
                "photos_backup.cli.verify.verify_archive", return_value=report
            ):
                if passed:
                    result = self.invoke("verify", "--json")
                    self.assertEqual(result.exit_code, 0)
                    self.assertFalse(history.path.exists())
                result = self.invoke("verify", "--record", "--json")
            self.assertEqual(result.exit_code, code, result.output)
            self.assertIs(json.loads(result.stdout)["passed"], passed)
            receipt, error = history.read()
            self.assertIsNone(error)
            self.assertIs(receipt["passed"], passed)
            status = self.invoke("status", "--json")
            document = json.loads(status.stdout)
            self.assertEqual(status.stderr, "")
            self.assertIs(document["last_verification"]["passed"], passed)
            self.assertFalse(document["files_verified"])
        self.assertEqual(before, paths.state_file.read_bytes())
        self.assertFalse(history.path.is_relative_to(paths.archive))

    def test_archive_open_failure_is_recorded_and_visible_while_disconnected(self):
        paths = self.initialized_archive()
        with mock.patch(
            "photos_backup.cli.verify.open_archive",
            side_effect=ArchiveUnavailable("disconnected"),
        ):
            result = self.invoke("verify", "--record", "--json")
        self.assertEqual(result.exit_code, ArchiveUnavailable.exit_code)
        self.assertEqual(json.loads(result.stdout)["archive_error"], "disconnected")
        with mock.patch(
            "photos_backup.cli.status.open_archive",
            side_effect=ArchiveUnavailable("disconnected"),
        ):
            status = self.invoke("status", "--short")
        self.assertIn("Last verification (this Mac): failed", status.stdout)
        self.assertIn("archive freshness unknown", status.stdout)
        self.assertTrue(
            VerificationHistory(self.config_path, paths.archive).path.exists()
        )

    def test_receipts_are_scoped_to_config_and_archive_and_read_errors_do_not_write(
        self,
    ):
        paths = self.initialized_archive()
        history = VerificationHistory(self.config_path, paths.archive)
        self.assertNotEqual(
            history.path,
            VerificationHistory(self.root / "other.toml", paths.archive).path,
        )
        self.assertNotEqual(
            history.path,
            VerificationHistory(self.config_path, self.root / "other").path,
        )
        self.assertEqual(history.read(), (None, None))
        self.assertFalse(history.path.parent.exists())
        history.path.parent.mkdir(parents=True)
        for invalid in ("[]", "{}", "invalid"):
            history.path.write_text(invalid)
            result = self.invoke("status", "--json")
            self.assertEqual(result.exit_code, 0)
            self.assertIn(
                "Could not read verification receipt",
                json.loads(result.stdout)["verification_history_error"],
            )
            self.assertEqual(history.path.read_text(), invalid)

    def test_receipt_write_failure_does_not_mask_verification_result(self):
        self.initialized_archive()
        with (
            mock.patch(
                "photos_backup.cli.verify.verify_archive",
                return_value=VerificationReport((Check("state", True, "ok"),)),
            ),
            mock.patch(
                "photos_backup.verification_history.tempfile.NamedTemporaryFile",
                side_effect=OSError("disk full"),
            ),
        ):
            result = self.invoke("verify", "--record", "--json")
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertTrue(json.loads(result.stdout)["passed"])
        self.assertIn("could not save verification receipt", result.stderr)

    def test_record_refuses_state_directory_inside_archive(self):
        paths = self.initialized_archive()
        with (
            mock.patch(
                "photos_backup.transfers.history_root",
                return_value=paths.archive / "local/transfers",
            ),
            mock.patch(
                "photos_backup.cli.verify.verify_archive",
                return_value=VerificationReport((Check("state", True, "ok"),)),
            ),
        ):
            result = self.invoke("verify", "--record")
        self.assertEqual(result.exit_code, 0)
        self.assertIn("must be outside the archive", result.stderr)
        self.assertFalse((paths.archive / "local").exists())

    def test_verification_freshness_uses_start_and_preserves_unknown(self):
        receipt = {
            "started_at": EARLY.isoformat(),
            "completed_at": (LATE + datetime.timedelta(hours=1)).isoformat(),
        }
        result = annotate_verification(receipt, LATE, None, True)
        self.assertTrue(result["archive_exported_since_verification"])
        result = annotate_verification(
            receipt,
            EARLY,
            {"status": "failed", "started_at": LATE.isoformat(), "completed_at": None},
            True,
        )
        self.assertTrue(result["archive_may_have_changed_since_verification"])
        result = annotate_verification(receipt, None, None, False)
        self.assertIsNone(result["archive_exported_since_verification"])
        self.assertFalse(result["archive_state_available"])

    def test_later_started_verification_wins_and_status_shows_subsequent_exports(self):
        paths = self.initialized_archive(last_successful_export_at=LATE)
        history = VerificationHistory(self.config_path, paths.archive)
        report = {
            "started_at": EARLY.isoformat(),
            "completed_at": EARLY.isoformat(),
            "passed": True,
            "checks": [],
            "archive_error": None,
        }
        with mock.patch("photos_backup.verification_history.click.echo"):
            history.record(report)
        result = self.invoke("status", "--short")
        self.assertIn("archive exported since then", result.stdout)
        self.assertIn("verify --record", result.stdout)
        with mock.patch("photos_backup.verification_history.click.echo"):
            history.record(
                {
                    **report,
                    "started_at": LATE.isoformat(),
                    "completed_at": LATE.isoformat(),
                    "passed": False,
                }
            )
            history.record(report)
        receipt, error = history.read()
        self.assertIsNone(error)
        self.assertFalse(receipt["passed"])
        self.assertEqual(receipt["started_at"], LATE.isoformat())

    def test_receipt_validation_tolerates_invalid_fields_without_tracebacks(self):
        paths = self.initialized_archive()
        history = VerificationHistory(self.config_path, paths.archive)
        history.path.parent.mkdir(parents=True)
        document = {
            "version": 1,
            "archive": str(paths.archive),
            "passed": True,
            "failed_checks": [],
            "archive_error": None,
            "started_at": EARLY.isoformat(),
            "completed_at": LATE.isoformat(),
        }
        for changes in (
            {"version": True},
            {"archive": "/other"},
            {"passed": "yes"},
            {"failed_checks": [3]},
            {"archive_error": []},
            {"started_at": "yesterday"},
            {"completed_at": "2026-01-01T00:00:00"},
        ):
            with self.subTest(changes=changes):
                history.path.write_text(json.dumps({**document, **changes}))
                self.assertIsNone(history.read()[0])
                result = self.invoke("status", "--short")
                self.assertEqual(result.exit_code, 0, result.output)
                self.assertIn("Could not read verification receipt", result.stderr)


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


class ArchiveFreshnessTests(ArchiveCommandTestCase):
    def test_status_compares_exact_archive_sources_without_probing_destinations(self):
        paths = self.initialized_archive(last_successful_export_at=THURSDAY)
        with self.config_path.open("a") as config:
            config.write(
                f'\n[ssd]\nsource = "{paths.archive}"\ndestination = "{self.root}/offline"\n'
                f'\n[rclone]\nsource = "{paths.archive.parent}"\nremote = "b2:photos"\n'
            )
        older = THURSDAY - datetime.timedelta(days=1)
        newer = THURSDAY + datetime.timedelta(days=1)
        history = TransferHistory(self.config_path)
        for copied_at, expected in ((older, True), (newer, False)):
            with mock.patch(
                "photos_backup.transfers._now", return_value=copied_at.isoformat()
            ):
                history.run(
                    "SSD: All Photos",
                    paths.archive,
                    self.root / "offline" / paths.archive.name,
                    lambda: BackupSummary(step_name="SSD: All Photos"),
                    dry_run=False,
                )
                history.run(
                    "Remote",
                    paths.archive.parent,
                    "b2:photos",
                    lambda: BackupSummary(step_name="Remote"),
                    dry_run=False,
                )
            with mock.patch(
                "pathlib.Path.resolve",
                side_effect=AssertionError("no filesystem resolution"),
            ):
                rows, errors = history.read()
                annotated = annotate_archive_freshness(rows, paths.archive, THURSDAY)
            self.assertEqual(errors, [])
            by_step = {row["step"]: row for row in annotated}
            self.assertIs(
                by_step["SSD: All Photos"]["archive_exported_since_copy"], expected
            )
            self.assertIsNone(by_step["Remote"]["archive_exported_since_copy"])
            with self.mounted():
                result = self.runner.invoke(
                    cli, ["--config", str(self.config_path), "status", "--json"]
                )
                text = self.runner.invoke(
                    cli, ["--config", str(self.config_path), "status"]
                )
            self.assertEqual(result.exit_code, 0, result.output)
            rows = {
                row["step"]: row
                for row in json.loads(result.stdout)["configured_transfers"]
            }
            self.assertIs(
                rows["SSD: All Photos"]["archive_exported_since_copy"], expected
            )
            self.assertEqual(
                "Archive exported since this copy" in text.stdout, expected
            )
            self.assertFalse((self.root / "offline").exists())


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
