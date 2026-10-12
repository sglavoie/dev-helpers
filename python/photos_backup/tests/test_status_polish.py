from __future__ import annotations

import datetime
import json
import shlex
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.download_report import summarize_downloads
from photos_backup.archive import ArchiveUnavailable
from photos_backup.archive.state import ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.cli.status import (
    _annotate_awaiting_card,
    _require_nothing_suggested,
)
from photos_backup.errors import ActionRequired
from photos_backup.summary import BackupSummary
from photos_backup.transfers import (
    TransferHistory,
)
from tests.archive_case import ArchiveCommandTestCase
from tests.status_case import EARLY, LATE, TIMEOUT_COUNT


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

    def test_detailed_status_names_a_baseline_report_only_once(self):
        report = ".photos-backup/reports/latest.csv"
        paths = self.initialized_archive()
        self.attempt(paths, status="succeeded", report_path=report)
        store = ArchiveStateStore(paths)
        for baseline, shown in ((paths.archive / report, False), (None, True)):
            with self.subTest(baseline=baseline):
                store.update(last_report_path=baseline)
                output = self.invoke("status").stdout
                self.assertEqual("Export report (if written):" in output, shown, output)
                self.assertEqual(
                    "Export report: the baseline report above" in output, not shown
                )

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
        self.assertIn("Action needed: run: ", result.stderr)
        self.assertIn("bootstrap", result.output)

        with mock.patch("photos_backup.cli.status.print_short_status", return_value=[]):
            self.assertEqual(self.invoke("status", "--check").exit_code, 0)
        self.assertEqual(self.invoke("status", "--check", "--json").exit_code, 2)

    def test_check_message_leads_with_the_drive_to_connect(self):
        _require_nothing_suggested(["photos-backup ssd"], check=False)
        _require_nothing_suggested([], check=True)
        for suggested, expected in (
            (["photos-backup daily"], "run: photos-backup daily"),
            (
                [
                    "photos-backup sd-card  # connect /Volumes/SDSONY first",
                    "photos-backup verify --record",
                ],
                "connect /Volumes/SDSONY, then run: photos-backup sd-card "
                "(+1 more; see status --short)",
            ),
        ):
            with self.assertRaises(ActionRequired) as raised:
                _require_nothing_suggested(suggested, check=True)
            self.assertEqual(raised.exception.format_message(), expected)

    def test_an_occasional_card_marks_only_the_copies_it_holds_up(self):
        def awaiting(extra=""):
            self.config_path.write_text(
                '[sd_card]\nsource = "/Volumes/NoSuchCard/DCIM"\n'
                f'destination = "{self.root}/raw"\n{extra}'
            )
            rows = [
                {"step": step}
                for step in ("SD Card", "SSD: All Photos", "SSD: SD Card")
            ]
            marked = _annotate_awaiting_card(rows, self.config_path)
            return [row["step"] for row in marked if row["awaiting_card"]]

        self.assertEqual(awaiting(), [])
        self.assertEqual(awaiting("occasional = true\n"), ["SD Card", "SSD: SD Card"])
        (self.root / "raw").mkdir()
        self.assertEqual(awaiting("occasional = true\n"), ["SD Card"])
        self.config_path.write_text("")
        self.assertEqual(
            _annotate_awaiting_card([{"step": "SD Card"}], self.config_path),
            [{"step": "SD Card", "awaiting_card": False}],
        )

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

    def test_check_lets_a_recently_copied_stale_route_wait(self):
        now = datetime.datetime.now(datetime.UTC)
        routes = (
            f'[sd_card]\nsource = "{self.root}/card"\n'
            f'destination = "{self.root}/raw"\n'
            f'[ssd]\nsource = "{self.root}/archive"\n'
            f'destination = "{self.root}/ssd"\n'
        )
        self.config_path.write_text(routes)
        history = TransferHistory(self.config_path)
        for name, source, destination, days_ago in (
            ("SSD: All Photos", "archive", "ssd/archive", 0),
            ("SSD: SD Card", "raw", "ssd/raw", 2),
            ("SD Card", "card", "raw/card", 1),
        ):
            when = now - datetime.timedelta(days=days_ago)
            with mock.patch(
                "photos_backup.transfers._now", return_value=when.isoformat()
            ):
                history.run(
                    name,
                    self.root / source,
                    self.root / destination,
                    lambda: BackupSummary(name),
                    dry_run=False,
                )
        for status_section, code in (
            ("", 0),
            ("[status]\ncopy_max_age_days = 2\n", 3),
            ("[status]\ncopy_max_age_days = 0\n", 3),
        ):
            with self.subTest(status_section=status_section):
                self.config_path.write_text(routes + status_section)
                result = self.invoke("status", "--check")
                self.assertEqual(result.exit_code, code, result.output)
                self.assertIn(
                    "SSD: SD Card: needs updating; copied 2 days ago", result.stdout
                )
                self.assertIn(
                    shlex.join(
                        ["photos-backup", "--config", str(self.config_path), "ssd"]
                    ),
                    result.stdout,
                )
                rows = json.loads(self.invoke("status", "--json").stdout)[
                    "configured_transfers"
                ]
                grace = {row["step"]: row["within_grace"] for row in rows}
                self.assertEqual(grace["SSD: SD Card"], code == 0)

        self.config_path.write_text(routes + "[status]\ncopy_max_age_days = -1\n")
        result = self.invoke("status", "--check")
        self.assertEqual(result.exit_code, 2, result.output)
        self.assertIn("copy_max_age_days", result.output)

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
