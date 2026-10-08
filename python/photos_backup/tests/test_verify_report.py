from __future__ import annotations

import datetime
import json
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.archive import ArchiveLocked, ArchiveUnavailable
from photos_backup.cli.cli import cli
from tests.test_verify import VerifyTestCase, write_export_db


class VerificationReportTests(VerifyTestCase):
    def assert_timing(self, document):
        started = datetime.datetime.fromisoformat(document["started_at"])
        completed = datetime.datetime.fromisoformat(document["completed_at"])
        self.assertEqual(started.utcoffset(), datetime.timedelta(0))
        self.assertEqual(completed.utcoffset(), datetime.timedelta(0))
        self.assertGreaterEqual(completed, started)
        self.assertGreaterEqual(document["elapsed_seconds"], 0)

    def test_json_archive_open_errors_preserve_exit_codes_without_scanning(self):
        before = self.paths.state_file.read_bytes()
        for error in (
            ArchiveUnavailable("archive drive is not mounted"),
            ArchiveLocked("archive is busy"),
            PermissionError("archive cannot be read"),
        ):
            with (
                self.subTest(error=error),
                mock.patch("photos_backup.cli.verify.open_archive", side_effect=error),
                mock.patch("photos_backup.cli.verify.verify_archive") as scan,
            ):
                result = self.invoke_verify(None, "--json")
            self.assertEqual(result.exit_code, getattr(error, "exit_code", 1))
            document = json.loads(result.stdout)
            self.assert_timing(document)
            self.assertEqual(document["archive"], str(self.archive_root))
            self.assertFalse(document["passed"])
            self.assertEqual(document["checks"], [])
            self.assertEqual(document["archive_error"], str(error))
            self.assertIn(str(error), result.stderr)
            scan.assert_not_called()
        self.assertEqual(self.paths.state_file.read_bytes(), before)
        self.assertFalse(self.paths.lock_file.exists())

    def test_json_stdout_matches_report_and_preserves_outcome_codes(self):
        before = self.paths.state_file.read_bytes()
        for condition, expected_code in (
            ("healthy", 0),
            ("cleanup", 3),
            ("missing", 1),
        ):
            if condition == "cleanup":
                self.save_state(pending_cleanup_run_id="run-7")
            elif condition == "missing":
                self.exported.unlink()
            for save_report in (False, True):
                with self.subTest(condition=condition, save_report=save_report):
                    destination = (
                        self.root / f"{condition}.json" if save_report else None
                    )
                    result = self.invoke_verify(destination, "--json")
                    self.assertEqual(result.exit_code, expected_code, result.output)
                    document = json.loads(result.stdout)
                    self.assert_timing(document)
                    self.assertEqual(document["passed"], expected_code == 0)
                    self.assertIsNone(document["archive_error"])
                    self.assertEqual(len(document["checks"]), 6)
                    self.assertIn("files checked", result.stderr)
                    if destination:
                        self.assertEqual(document, json.loads(destination.read_text()))
                        self.assertIn("Verification report:", result.stderr)
            if condition == "healthy":
                self.assertEqual(self.paths.state_file.read_bytes(), before)

    def test_pending_cleanup_is_action_required_unless_integrity_also_fails(self):
        self.save_state(pending_cleanup_run_id="run-7")
        before = self.paths.state_file.read_bytes()
        for damaged in (False, True):
            with self.subTest(damaged=damaged):
                if damaged:
                    self.exported.unlink()
                destination = self.root / f"cleanup-{damaged}.json"
                result = self.invoke_verify(destination)
                self.assertEqual(result.exit_code, 1 if damaged else 3, result.output)
                self.assertIn("approve-cleanup run-7", result.output)
                document = json.loads(destination.read_text())
                self.assertFalse(document["passed"])
                failed = [
                    check["name"] for check in document["checks"] if not check["passed"]
                ]
                self.assertIn("pending cleanup", failed)
                if damaged:
                    self.assertIn("missing assets", failed)
                else:
                    self.assertEqual(failed, ["pending cleanup"])
                self.assertEqual(self.paths.state_file.read_bytes(), before)

    def invoke_verify(self, destination, *extra):
        config = self.root / "config.toml"
        config.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\narchive = "{self.archive_root}"\n'
            'library = "/unused.photoslibrary"\n'
        )
        return CliRunner().invoke(
            cli,
            [
                "--config",
                str(config),
                "--volume",
                str(self.volume),
                "verify",
                *(["--report", str(destination)] if destination else []),
                *extra,
            ],
        )

    def test_report_keeps_all_missing_and_changed_paths_on_failure(self):
        missing = [self.archive_root / f"missing-{index}.jpg" for index in range(5)]
        for path in missing:
            path.write_bytes(b"photo")
        write_export_db(self.paths.export_db, [self.exported, *missing])
        for path in missing:
            path.unlink()
        self.exported.write_bytes(b"changed")
        before = self.paths.state_file.read_bytes()
        destination = self.root / "findings.json"
        result = self.invoke_verify(destination)
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("and 2 more", result.output)
        document = json.loads(destination.read_text())
        self.assertFalse(document["passed"])
        checks = {check["name"]: check for check in document["checks"]}
        self.assertEqual(
            checks["missing assets"]["paths"], [str(path) for path in missing]
        )
        self.assertEqual(checks["signatures"]["paths"], [str(self.exported)])
        self.assertEqual(self.paths.state_file.read_bytes(), before)

    def test_report_can_record_a_healthy_archive(self):
        destination = self.root / "healthy.json"
        result = self.invoke_verify(destination)
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertTrue(json.loads(destination.read_text())["passed"])
        self.assertIn("Reading export database", result.stderr)
        self.assertIn("files checked 1/1", result.stderr)
        self.assertNotIn("unresolved downloads", result.stderr)
        self.assertNotIn("files checked", result.stdout)

    def test_report_never_overwrites_an_existing_file_or_writes_inside_archive(self):
        existing = self.root / "existing.json"
        existing.write_text("keep me")
        for destination in (
            existing,
            self.archive_root / "new.json",
            self.paths.state_file,
        ):
            with self.subTest(destination=destination):
                result = self.invoke_verify(destination)
                self.assertEqual(result.exit_code, 2, result.output)
        self.assertEqual(existing.read_text(), "keep me")
        self.assertFalse((self.archive_root / "new.json").exists())

    def test_invalid_report_parent_stops_before_scanning_or_creating_anything(self):
        not_directory = self.root / "file"
        not_directory.write_text("keep")
        for parent in (self.root / "absent", not_directory):
            with (
                self.subTest(parent=parent),
                mock.patch("photos_backup.cli.verify.verify_archive") as scan,
            ):
                result = self.invoke_verify(parent / "report.json")
            self.assertEqual(result.exit_code, 2, result.output)
            self.assertIn("must be an existing directory", result.output)
            scan.assert_not_called()
        self.assertFalse((self.root / "absent").exists())
        self.assertEqual(not_directory.read_text(), "keep")

    def test_report_write_error_after_scan_is_a_readable_cli_failure(self):
        destination = self.root / "report.json"
        original_open = Path.open

        def fail_report(path, mode="r", *args, **kwargs):
            if path == destination and mode == "x":
                raise PermissionError("permission changed during scan")
            return original_open(path, mode, *args, **kwargs)

        with mock.patch.object(Path, "open", fail_report):
            result = self.invoke_verify(destination)
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("Could not write report", result.output)


if __name__ == "__main__":
    unittest.main()
