from __future__ import annotations

import contextlib
import datetime
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.cleanup import (
    CleanupApproval,
    CleanupManifest,
    write_manifest,
)
from photos_backup.apple_photos.verify import Check, VerificationReport
from photos_backup.archive import ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.errors import ACTION_REQUIRED_EXIT_CODE
from tests.test_cli import ArchiveCommandTestCase


class ApproveCleanupTests(ArchiveCommandTestCase):
    def test_approving_with_nothing_pending_asks_for_a_daily_run(self) -> None:
        self.initialized_archive()

        with self.mounted():
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "approve-cleanup", "run-7"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("No cleanup is waiting", result.output)

    def test_an_approval_reports_what_it_deleted_and_kept(self) -> None:
        self.initialized_archive(pending_cleanup_run_id="run-7")
        approval = CleanupApproval(
            run_id="run-7",
            manifest=None,
            deleted=(Path("/a/gone.jpg"),),
            stale=(Path("/a/edited.jpg"),),
            unreviewed=(),
            completed_at=datetime.datetime(2026, 8, 13, 9, 30, tzinfo=datetime.UTC),
        )

        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.approve_cleanup.run_cleanup_approval",
                return_value=approval,
            ),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "approve-cleanup", "run-7"]
            )

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Approved cleanup run 'run-7'", result.output)
        self.assertIn("Deleted: 1 archive file(s)", result.output)
        self.assertIn("No longer deletable, so kept: 1", result.output)
        self.assertIn("Mirror completed at", result.output)

    def test_discarding_clears_the_pending_run_without_deleting(self) -> None:
        paths = self.initialized_archive(pending_cleanup_run_id="run-7")
        write_manifest(
            paths.cleanup_manifest("run-7"),
            CleanupManifest(
                run_id="run-7",
                created_at=datetime.datetime(2026, 8, 13, 9, 30, tzinfo=datetime.UTC),
                hostname="tester",
                archive=paths.archive,
                reason="one archive file has no export-database record",
                assets=(),
                candidates=(),
                changed=(),
                unknown=(Path("/a/stranger.jpg"),),
                ambiguous=(),
            ),
        )

        with self.mounted():
            result = self.runner.invoke(
                cli,
                [
                    "--config",
                    str(self.config_path),
                    "approve-cleanup",
                    "run-7",
                    "--discard",
                ],
            )

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Discarded cleanup run 'run-7'", result.output)
        self.assertIn("nothing was deleted", result.output)
        self.assertIsNone(ArchiveStateStore(paths).load().pending_cleanup_run_id)


class LocalExportCleanupTests(ArchiveCommandTestCase):
    """The destructive branch is never reached here: no test has a terminal."""

    def setUp(self) -> None:
        super().setUp()
        self.export = self.root / "export"
        self.photo = self.export / "2022" / "03" / "a.JPG"
        self.photo.parent.mkdir(parents=True)
        self.photo.write_text("12345")
        self.config_path.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\n'
            f'archive = "{self.volume}/Media/Apple Photos"\n'
            'library = "/Users/tester/Pictures/Photos Library.photoslibrary"\n'
            f'legacy_export = "{self.export}"\n'
        )

    def run_cleanup(self, *args: str, healthy: bool = False):
        report = VerificationReport(checks=(Check("state", True, "healthy"),))
        with contextlib.ExitStack() as stack:
            stack.enter_context(self.mounted())
            if healthy:
                stack.enter_context(
                    mock.patch(
                        "photos_backup.cli.cleanup_local_export.verify_archive",
                        return_value=report,
                    )
                )
            return self.runner.invoke(
                cli,
                ["--config", str(self.config_path), "cleanup-local-export", *args],
            )

    def test_a_run_without_a_terminal_is_refused(self) -> None:
        result = self.run_cleanup()

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("never from a script", result.output)
        self.assertTrue(self.photo.is_file())

    def test_an_uninitialized_archive_is_refused(self) -> None:
        result = self.run_cleanup("--dry-run")

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("is not initialized", result.output)
        self.assertTrue(self.photo.is_file())

    def test_a_failing_verification_stops_the_cleanup(self) -> None:
        self.initialized_archive()

        result = self.run_cleanup("--dry-run")

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("Archive check(s) failed", result.output)
        self.assertTrue(self.photo.is_file())

    def test_a_dry_run_shows_the_path_count_and_size_it_would_delete(self) -> None:
        self.initialized_archive()

        result = self.run_cleanup("--dry-run", healthy=True)

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn(str(self.export), result.output)
        self.assertIn("Files: 1", result.output)
        self.assertIn("Size: 5 B", result.output)
        self.assertIn("nothing was deleted", result.output)
        self.assertTrue(self.photo.is_file())

    def test_unrecognized_contents_stop_the_cleanup(self) -> None:
        self.initialized_archive()
        (self.export / "taxes.pdf").write_text("not a photo")

        result = self.run_cleanup("--dry-run", healthy=True)

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("not something an Apple Photos export writes", result.output)
        self.assertTrue(self.photo.is_file())

    def test_an_export_configured_on_the_archive_volume_is_refused(self) -> None:
        self.initialized_archive()
        self.config_path.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\n'
            f'archive = "{self.volume}/Media/Apple Photos"\n'
            'library = "/Users/tester/Pictures/Photos Library.photoslibrary"\n'
            f'legacy_export = "{self.volume}"\n'
        )

        result = self.run_cleanup("--dry-run", healthy=True)

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("archive volume", result.output)
        self.assertTrue(self.photo.is_file())


if __name__ == "__main__":
    unittest.main()
