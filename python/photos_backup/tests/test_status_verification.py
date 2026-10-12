from __future__ import annotations

import datetime
import json
from unittest import mock

from photos_backup.apple_photos.verify import Check, VerificationReport
from photos_backup.archive import ArchiveUnavailable
from photos_backup.cli.cli import cli
from photos_backup.verification_history import (
    VerificationHistory,
    annotate_verification,
)
from tests.archive_case import ArchiveCommandTestCase
from tests.status_case import EARLY, LATE


class VerificationHistoryTests(ArchiveCommandTestCase):
    def invoke(self, *args):
        with self.mounted():
            return self.runner.invoke(cli, ["--config", str(self.config_path), *args])

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
