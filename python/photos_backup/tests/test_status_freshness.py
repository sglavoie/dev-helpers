from __future__ import annotations

import datetime
import io
import json
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.cli.cli import cli
from photos_backup.cli.context import CliContext
from photos_backup.copy_safety import disconnected_volume
from photos_backup.status_report import _transfer_suggestions, print_transfer_history
from photos_backup.summary import BackupSummary
from photos_backup.transfers import (
    TransferHistory,
    annotate_archive_freshness,
    annotate_upstream_freshness,
)
from tests.archive_case import ArchiveCommandTestCase
from tests.status_case import EARLY, LATE, transfer
from tests.test_verify import THURSDAY


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


class TransferSuggestionTests(unittest.TestCase):
    TIMESTAMP = "2026-01-01T00:00:00+00:00"

    def row(self, step, *, stale=True, mode="copy", drive=None, grace=False):
        attempt = {"started_at": self.TIMESTAMP, "status": "succeeded", "mode": mode}
        return {
            "step": step,
            "last_attempt": attempt,
            "last_success": {**attempt, "completed_at": self.TIMESTAMP},
            "archive_exported_since_copy": stale,
            "disconnected_drive": drive,
            "within_grace": grace,
        }

    def test_one_copy_keeps_its_own_command(self):
        rows = [self.row("SD Card", stale=False), self.row("SSD: All Photos")]
        self.assertEqual(_transfer_suggestions(rows), [("photos-backup ssd", False)])

    def test_several_copies_collapse_into_backup_all_skipping_current_steps(self):
        rows = [
            self.row("SD Card", stale=False),
            self.row("SSD: All Photos", drive="/Volumes/Data", grace=True),
            self.row("SSD: SD Card", drive="/Volumes/Data"),
            self.row("Remote", drive="/Volumes/Data", grace=True),
        ]
        self.assertEqual(
            _transfer_suggestions(rows),
            [
                (
                    "photos-backup backup-all --skip-apple-photos --skip-sd-card"
                    "  # connect /Volumes/Data first",
                    False,
                )
            ],
        )

    def test_a_copy_awaiting_the_occasional_card_waits_on_its_own_command(self):
        card = {**self.row("SD Card", drive="/Volumes/SDSONY"), "awaiting_card": True}
        on_ssd = {**self.row("SSD: SD Card"), "awaiting_card": True}
        waiting = ("photos-backup sd-card  # connect /Volumes/SDSONY first", True)
        self.assertEqual(_transfer_suggestions([card]), [waiting])
        self.assertEqual(
            _transfer_suggestions([card, self.row("SSD: All Photos"), on_ssd]),
            [("photos-backup ssd", False), waiting],
        )
        self.assertEqual(
            _transfer_suggestions([card, on_ssd]),
            [waiting, ("photos-backup ssd", True)],
        )

    def test_mirror_previews_stay_separate_and_grace_applies_per_command(self):
        rows = [
            self.row("SD Card", drive="/Volumes/SDSONY", grace=True),
            self.row("SSD: All Photos", grace=True),
            self.row("Remote", mode="mirror"),
        ]
        self.assertEqual(
            _transfer_suggestions(rows),
            [
                (
                    "photos-backup backup-all --skip-apple-photos --skip-remote"
                    "  # connect /Volumes/SDSONY first",
                    True,
                ),
                ("photos-backup remote --delete --dry-run", False),
            ],
        )


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

    def test_unplugged_drive_is_named_beside_the_retry(self):
        text = self.render(status="started", disconnected_drive="/Volumes/Data")
        self.assertIn(
            "Retry copy: photos-backup ssd  # connect /Volumes/Data first", text
        )


class DisconnectedDriveTests(ArchiveCommandTestCase):
    def test_only_unmounted_volumes_are_reported_without_resolving_paths(self):
        mounted = {"/Volumes/SanDisk"}
        with (
            mock.patch(
                "photos_backup.copy_safety.os.path.ismount",
                side_effect=lambda path: path in mounted,
            ),
            mock.patch(
                "pathlib.Path.resolve",
                side_effect=AssertionError("no filesystem resolution"),
            ),
        ):
            for path, expected in (
                ("/Volumes/Data/Pictures", "/Volumes/Data"),
                ("/volumes/Data", "/volumes/Data"),
                ("/Volumes/SanDisk/Media", None),
                ("/Volumes", None),
                ("/", None),
                ("", None),
                ("/Users/tester/Pictures", None),
                ("b2:photos", None),
            ):
                with self.subTest(path=path):
                    self.assertEqual(disconnected_volume(path), expected)

    def test_status_names_the_drive_to_connect(self):
        self.config_path.write_text(
            '[ssd]\nsource = "/Volumes/NoSuchSource/Media"\n'
            f'destination = "{self.root}/ssd"\n'
        )
        with mock.patch(
            "photos_backup.copy_safety.os.path.ismount", return_value=False
        ):
            short = self.runner.invoke(
                cli, ["--config", str(self.config_path), "status", "--short"]
            )
            document = json.loads(
                self.runner.invoke(
                    cli, ["--config", str(self.config_path), "status", "--json"]
                ).stdout
            )
        self.assertIn("ssd  # connect /Volumes/NoSuchSource first", short.stdout)
        (row,) = document["configured_transfers"]
        self.assertEqual(row["disconnected_drive"], "/Volumes/NoSuchSource")
