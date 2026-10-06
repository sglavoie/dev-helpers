import datetime
import dataclasses
import json
from unittest import mock

from photos_backup.apple_photos.attempt import ExportAttemptStore
from photos_backup.apple_photos.plan import ExportMode, ExportPlan
from photos_backup.archive import ArchiveStateStore, open_archive
from photos_backup.cli.cli import cli
from photos_backup.cli.status import _latest_export_at
from tests.test_cli import ArchiveCommandTestCase
from tests.test_export import ExportTestCase, FakeRunner, THURSDAY, row


class ExportAttemptTests(ExportTestCase):
    def read_attempt(self):
        with open_archive(self.config, dry_run=True, probes=self.probes) as archive:
            attempt, error = ExportAttemptStore(archive).read()
        self.assertIsNone(error)
        return attempt

    def snapshot(self):
        return {
            p.relative_to(self.volume): p.read_bytes()
            for p in self.volume.rglob("*")
            if p.is_file()
        }

    def test_recent_and_failed_attempts_preserve_successful_baseline(self):
        full = self.run_export(FakeRunner())
        state = self.paths.state_file.read_bytes()
        self.assertTrue(self.read_attempt()["baseline_advanced"])
        recent = self.run_export(
            FakeRunner(), plan=ExportPlan(ExportMode.RECENT, "recent", THURSDAY)
        )
        attempt = self.read_attempt()
        self.assertEqual(attempt["mode"], "recent")
        self.assertEqual(attempt["status"], "succeeded")
        self.assertFalse(attempt["baseline_advanced"])

        self.assertIsNone(attempt["retry_arguments"])
        self.assertEqual(attempt["report_path"], str(recent.report_path))
        failed = self.run_export(FakeRunner([row("missing.mov", missing=1)]))
        attempt = self.read_attempt()
        self.assertEqual(attempt["status"], "failed")
        self.assertEqual(attempt["error"], failed.failure_reason())
        self.assertEqual(attempt["missing_count"], 1)
        self.assertEqual(attempt["error_count"], 0)
        self.assertEqual(
            attempt["download_report_path"],
            str(failed.report_path.with_suffix(".downloads.json")),
        )
        self.assertEqual(self.paths.state_file.read_bytes(), state)
        self.assertEqual(self.state.last_report_path, full.report_path)

    def test_successful_recent_time_survives_failures_and_interruption(self):
        self.run_export(FakeRunner())
        later = THURSDAY + datetime.timedelta(days=1)
        self.probes = dataclasses.replace(self.probes, now=lambda: later)
        self.run_export(
            FakeRunner(), plan=ExportPlan(ExportMode.RECENT, "recent", later)
        )
        self.run_export(FakeRunner([row("missing.mov", missing=1)]))
        self.assertEqual(_latest_export_at(THURSDAY, self.read_attempt()), later)

        # Inspect the receipt directly inside the writer lock, avoiding a second lock.
        def stop(arguments):
            raw = json.loads(
                (self.paths.metadata / "last-export-attempt.json").read_text()
            )
            self.assertEqual(raw["last_successful_export_at"], later.isoformat())
            raise KeyboardInterrupt()

        with self.assertRaises(KeyboardInterrupt):
            self.run_export(stop)
        self.assertEqual(_latest_export_at(THURSDAY, self.read_attempt()), later)

    def test_older_success_receipt_seeds_retained_time(self):
        self.run_export(FakeRunner())
        path = self.paths.metadata / "last-export-attempt.json"
        document = json.loads(path.read_text())
        del document["last_successful_export_at"]
        path.write_text(json.dumps(document))
        self.run_export(FakeRunner([row("missing.mov", missing=1)]))
        self.assertEqual(
            self.read_attempt()["last_successful_export_at"], document["completed_at"]
        )

    def test_unreadable_export_report_does_not_record_zero_missing_files(self):
        self.run_export(lambda arguments: 0)
        attempt = self.read_attempt()
        self.assertEqual(attempt["status"], "failed")
        self.assertIsNone(attempt["missing_count"])
        self.assertIsNone(attempt["error_count"])

    def test_exceptions_and_interruptions_are_recorded_then_reraised(self):
        self.run_export(FakeRunner())
        state = self.paths.state_file.read_bytes()
        for error in (OSError("export unavailable"), KeyboardInterrupt()):

            def fail(arguments):
                raw = json.loads(
                    (self.paths.metadata / "last-export-attempt.json").read_text()
                )
                self.assertEqual(raw["status"], "started")
                raise error

            with self.subTest(error=error), self.assertRaises(type(error)):
                self.run_export(fail)
            attempt = self.read_attempt()
            self.assertEqual(
                attempt["status"],
                "failed" if isinstance(error, Exception) else "interrupted",
            )
            self.assertEqual(attempt["error"], str(error) or type(error).__name__)
            self.assertIsNotNone(attempt["completed_at"])
        self.assertEqual(self.paths.state_file.read_bytes(), state)

    def test_dry_runs_leave_existing_and_absent_receipts_untouched(self):
        for existing in (False, True):
            if existing:
                self.run_export(FakeRunner())
            before = self.snapshot()
            self.run_export(FakeRunner(), dry_run=True)
            self.run_export(FakeRunner(), dry_run=True, plan_only=True)
            self.assertEqual(self.snapshot(), before)

    def test_custom_export_is_distinguished_from_a_full_baseline(self):
        self.run_export(FakeRunner(), extra_arguments={"album": ("Selected",)})
        attempt = self.read_attempt()
        self.assertTrue(attempt["restricted"])
        self.assertFalse(attempt["baseline_advanced"])

    def test_receipt_write_failure_does_not_mask_success_or_export_exception(self):
        with (
            mock.patch(
                "photos_backup.apple_photos.attempt.os.replace",
                side_effect=OSError("receipt unavailable"),
            ),
            mock.patch("photos_backup.apple_photos.attempt.click.echo") as echo,
        ):
            # A recent export does not invoke the archive state's atomic writer.
            self.assertTrue(
                self.run_export(
                    FakeRunner(), plan=ExportPlan(ExportMode.RECENT, "recent")
                ).complete
            )
            with self.assertRaisesRegex(RuntimeError, "export error"):
                self.run_export(mock.Mock(side_effect=RuntimeError("export error")))
        self.assertIn("could not save export attempt", echo.call_args.args[0])
        self.assertEqual(list(self.paths.metadata.glob("tmp*")), [])

    def test_relative_report_path_survives_archive_relocation(self):
        result = self.run_export(FakeRunner())
        raw = json.loads((self.paths.metadata / "last-export-attempt.json").read_text())
        self.assertEqual(
            raw["report_path"], str(result.report_path.relative_to(self.archive))
        )
        relocated = self.volume / "relocated"
        self.archive.rename(relocated)
        self.config = dataclasses.replace(self.config, archive=relocated)
        self.assertEqual(
            self.read_attempt()["report_path"], str(relocated / raw["report_path"])
        )
        self.assertEqual(
            self.read_attempt()["download_report_path"],
            str(relocated / raw["download_report_path"]),
        )


class ExportAttemptStatusTests(ArchiveCommandTestCase):
    def status(self, *args):
        with self.mounted():
            return self.runner.invoke(
                cli, ["--config", str(self.config_path), "status", *args]
            )

    def test_absent_receipt_creates_nothing(self):
        result = self.status("--json")
        self.assertEqual(result.exit_code, 0, result.output)
        document = json.loads(result.stdout)
        self.assertIsNone(document["last_export_attempt"])
        self.assertIsNone(document["export_attempt_error"])
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_status_reads_receipt_and_reports_corruption_without_changing_state(self):
        paths = self.initialized_archive(last_successful_export_at=THURSDAY)
        path = paths.metadata / "last-export-attempt.json"
        receipt = {
            "version": 1,
            "mode": "recent",
            "status": "failed",
            "hostname": "test.local",
            "started_at": THURSDAY.isoformat(),
            "completed_at": THURSDAY.isoformat(),
            "restricted": False,
            "baseline_advanced": False,
            "report_path": ".photos-backup/reports/latest.csv",
            "error": "2 files remain missing",
        }
        path.write_text(json.dumps(receipt))
        state = ArchiveStateStore(paths).load()
        result = self.status()
        self.assertIn("recent — failed", result.stdout)
        self.assertIn("2 files remain missing", result.stdout)
        self.assertIn("Baseline advanced: no", result.stdout)
        document = json.loads(self.status("--json").stdout)
        self.assertEqual(
            document["last_export_attempt"]["report_path"],
            str(paths.archive / receipt["report_path"]),
        )
        self.assertEqual(
            document["state"]["last_successful_export_at"],
            state.last_successful_export_at.isoformat(),
        )
        for invalid in (
            "invalid",
            "[]",
            json.dumps({**receipt, "version": 7}),
            json.dumps({**receipt, "report_path": "../escape"}),
            json.dumps({**receipt, "started_at": "yesterday"}),
            json.dumps({**receipt, "missing_count": -1}),
            json.dumps({**receipt, "error_count": True}),
            json.dumps({**receipt, "download_report_path": "../escape"}),
            json.dumps({**receipt, "retry_arguments": "recent --days 7"}),
        ):
            with self.subTest(invalid=invalid):
                path.write_text(invalid)
                result = self.status("--json")
                self.assertEqual(result.exit_code, 0, result.output)
                document = json.loads(result.stdout)
                self.assertIsNone(document["last_export_attempt"])
                self.assertIn(
                    "Could not read export attempt", document["export_attempt_error"]
                )
                self.assertEqual(result.stderr, "")
                self.assertEqual(path.read_text(), invalid)
        self.assertEqual(ArchiveStateStore(paths).load(), state)

    def test_unfinished_attempt_does_not_claim_process_is_still_running(self):
        paths = self.initialized_archive()
        (paths.metadata / "last-export-attempt.json").write_text(
            json.dumps(
                {
                    "version": 1,
                    "mode": "full",
                    "status": "started",
                    "hostname": "test.local",
                    "started_at": THURSDAY.isoformat(),
                    "completed_at": None,
                    "restricted": False,
                    "baseline_advanced": False,
                    "report_path": ".photos-backup/reports/latest.csv",
                    "error": None,
                }
            )
        )
        self.assertIn(
            "completion not recorded (running or interrupted)", self.status().stdout
        )
