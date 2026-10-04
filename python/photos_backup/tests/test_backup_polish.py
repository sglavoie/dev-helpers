from __future__ import annotations

import contextlib
import io
import sqlite3
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.errors import ActionRequired
from photos_backup.remote.backup import _parse_rclone_stats
from photos_backup.summary import (
    BackupSummary,
    parse_rsync_stats,
    print_pipeline_summary,
    print_summary,
)
from photos_backup.apple_photos.verify import SIGNATURES
from photos_backup.apple_photos.cleanup import approve_cleanup
from photos_backup.archive import ArchiveUnsafe
from tests.test_bootstrap import asset
from tests.test_cleanup import MirrorTestCase
from tests.test_verify import VerifyTestCase, write_export_db


class CleanupPreviewTests(MirrorTestCase):
    def setUp(self):
        super().setUp()
        self.write_photo("stranger.jpg", b"unrecorded")
        self.run_id = self.reconcile().run_id
        self.paths.lock_file.unlink()
        self.config_path = self.root / "config.toml"
        self.config_path.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\n'
            f'archive = "{self.archive_root}"\nlibrary = "{self.config.library}"\n'
        )

    def snapshot(self):
        return {
            path: (
                path.stat().st_mtime_ns,
                path.read_bytes() if path.is_file() else None,
            )
            for path in self.archive_root.rglob("*")
        }

    def preview(self, *args, probes=None, run_id=None):
        with (
            mock.patch(
                "photos_backup.archive.SystemProbes", return_value=self.system_probes
            ),
            mock.patch(
                "photos_backup.apple_photos.cleanup.PhotosProbes",
                return_value=probes or self.default_probes(),
            ),
        ):
            return CliRunner().invoke(
                cli,
                [
                    "--config",
                    str(self.config_path),
                    "approve-cleanup",
                    run_id or self.run_id,
                    "--dry-run",
                    *args,
                ],
            )

    def test_preview_revalidates_and_lists_deletions_without_any_writes(self):
        before = self.snapshot()
        result = self.preview()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn(f"Would delete: {self.gone}", result.output)
        self.assertIn(f"({self.gone.stat().st_size} bytes)", result.output)
        self.assertIn("cleanup remains pending", result.output)
        self.assertNotIn("Approved cleanup", result.output)
        self.assertEqual(self.snapshot(), before)
        self.assertFalse(self.paths.lock_file.exists())

        # Approval revalidates again; a preview never authorizes a later edit.
        self.gone.write_bytes(b"edited after preview")
        approval = self.approve(self.run_id)
        self.assertEqual(approval.deleted, ())
        self.assertEqual(approval.stale, (self.gone,))

    def test_preview_explains_changed_and_unreviewed_files_that_will_be_kept(self):
        self.gone.write_bytes(b"changed since manifest")
        probes = self.probes(
            library=(), recorded=(asset("kept"), asset("gone")), files=self.records
        )
        before = self.snapshot()
        result = self.preview(probes=probes)
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Would delete: 0 archive file(s)", result.output)
        self.assertIn("(0 bytes)", result.output)
        self.assertIn("No longer deletable, so kept: 1", result.output)
        self.assertIn("Never reviewed, so kept: 1", result.output)
        self.assertIn(f"Kept: {self.gone}", result.output)
        self.assertIn(f"Kept: {self.kept}", result.output)
        self.assertEqual(self.snapshot(), before)

    def test_preview_keeps_a_photo_restored_to_the_library(self):
        probes = self.probes(
            library=(asset("kept"), asset("gone")),
            recorded=(asset("kept"), asset("gone")),
            files=self.records,
        )
        result = self.preview(probes=probes)
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn(f"Kept: {self.gone}", result.output)
        self.assertTrue(self.gone.exists())

    def test_preview_refusals_write_nothing(self):
        for refusal in ("wrong-run", "wrong-owner", "malformed", "missing", "discard"):
            with self.subTest(refusal=refusal):
                manifest = self.paths.cleanup_manifest(self.run_id)
                original = manifest.read_bytes()
                if refusal == "wrong-owner":
                    self.save_state(
                        writer_hostname="other.local",
                        pending_cleanup_run_id=self.run_id,
                    )
                elif refusal == "malformed":
                    manifest.write_text("{}")
                elif refusal == "missing":
                    manifest.unlink()
                before = self.snapshot()
                result = self.preview(
                    *(["--discard"] if refusal == "discard" else []),
                    run_id="another-run" if refusal == "wrong-run" else self.run_id,
                )
                self.assertEqual(
                    result.exit_code,
                    2 if refusal in ("discard", "malformed") else 3,
                    result.output,
                )
                self.assertEqual(self.snapshot(), before)
                manifest.write_bytes(original)
                self.save_state(pending_cleanup_run_id=self.run_id)

    def test_read_only_archive_cannot_be_used_for_actual_approval(self):
        before = self.snapshot()
        with self.archive(dry_run=True) as archive:
            with self.assertRaises(ArchiveUnsafe):
                approve_cleanup(self.config, archive, self.run_id)
        self.assertEqual(self.snapshot(), before)


class RemoteBehaviorTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name).resolve()
        self.source = self.root / "photos"
        self.source.mkdir()
        self.destination = self.root / "ssd"
        self.destination.mkdir()
        self.config = self.root / "config.toml"

    def configure(self, remote_source):
        self.config.write_text(
            f'[ssd]\nsource = "{self.source}"\ndestination = "{self.destination}"\n'
            '[rclone]\nremote = "b2:photos"\n'
            + (f'source = "{remote_source}"\n' if remote_source is not None else "")
        )

    def invoke(self, command="backup-all", *args):
        options = (
            ["--skip-apple-photos", "--skip-sd-card"] if command == "backup-all" else []
        )
        return CliRunner().invoke(
            cli, ["--config", str(self.config), command, *options, *args]
        )

    def test_remote_deletion_is_explicit_and_independent_of_ssd_deletion(self):
        self.configure(self.destination)
        for command, flags, verb, ssd_delete in (
            ("remote", [], "copy", False),
            ("remote", ["--delete"], "sync", False),
            ("backup-all", [], "copy", False),
            ("backup-all", ["--delete"], "copy", True),
            ("backup-all", ["--delete-remote"], "sync", False),
            ("backup-all", ["--delete", "--delete-remote"], "sync", True),
        ):
            for dry_run in (False, True):
                with (
                    self.subTest(command=command, flags=flags, dry_run=dry_run),
                    mock.patch(
                        "photos_backup.remote.backup.shutil.which",
                        return_value="rclone",
                    ),
                    mock.patch(
                        "photos_backup.remote.backup.stream_command",
                        return_value=subprocess.CompletedProcess(
                            [], 0, stdout="Transferred: 0 / 0\n"
                        ),
                    ) as remote,
                    mock.patch(
                        "photos_backup.ssd.backup.stream_command",
                        return_value=subprocess.CompletedProcess(
                            [], 0, stdout="Number of regular files transferred: 0\n"
                        ),
                    ) as ssd,
                ):
                    result = self.invoke(
                        command, *flags, *(["--dry-run"] if dry_run else [])
                    )
                self.assertEqual(result.exit_code, 0, result.output)
                argv = remote.call_args.args[0]
                self.assertEqual(
                    argv[:4], ["rclone", verb, str(self.destination), "b2:photos"]
                )
                self.assertEqual("--dry-run" in argv, dry_run)
                if command == "backup-all":
                    self.assertEqual("--delete" in ssd.call_args.args[0], ssd_delete)
                if command == "backup-all":
                    expected = "0 proposed transfers" if dry_run else "0 files"
                else:
                    expected = (
                        "Proposed transfers: 0" if dry_run else "Files transferred: 0"
                    )
                self.assertIn(expected, result.output)

    def test_failed_ssd_blocks_dependent_remote_sources_including_aliases(self):
        alias = self.root / "alias"
        alias.symlink_to(self.destination, target_is_directory=True)
        for source in (
            None,
            self.destination,
            self.destination / "photos",
            self.root,
            alias / "photos",
        ):
            self.configure(source)
            for dry_run in (False, True):
                for failure, code in (
                    (RuntimeError("copy failed"), 1),
                    (ActionRequired("drive disconnected"), 3),
                ):
                    with (
                        self.subTest(source=source, dry_run=dry_run, failure=failure),
                        mock.patch(
                            "photos_backup.cli.backup_all.SsdBackup.backup",
                            side_effect=failure,
                        ),
                        mock.patch(
                            "photos_backup.cli.backup_all.RemoteBackup"
                        ) as remote,
                    ):
                        result = self.invoke(
                            "backup-all", *(["--dry-run"] if dry_run else [])
                        )
                    self.assertEqual(result.exit_code, code, result.output)
                    self.assertIn(
                        "SSD copy did not complete; remote source overlaps",
                        result.output,
                    )
                    remote.assert_not_called()

    def test_partial_ssd_failure_blocks_remote_and_preserves_copy_results(self):
        self.configure(None)
        with (
            mock.patch(
                "photos_backup.cli.backup_all.SsdBackup.backup",
                return_value=[
                    BackupSummary("SSD: All Photos", files_transferred=4),
                    BackupSummary("SSD: SD Card", error="copy failed"),
                ],
            ),
            mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote,
        ):
            result = self.invoke()
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("4 files", result.output)
        self.assertIn("SSD copy did not complete", result.output)
        remote.assert_not_called()

    def test_independent_remote_still_runs_after_ssd_failure(self):
        for source in (self.source, self.root / "ssd-independent"):
            self.configure(source)
            with (
                mock.patch(
                    "photos_backup.cli.backup_all.SsdBackup.backup",
                    side_effect=RuntimeError("copy failed"),
                ),
                mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote,
            ):
                remote.return_value.backup.return_value = BackupSummary(
                    "Remote", files_transferred=0
                )
                result = self.invoke()
            self.assertEqual(result.exit_code, 1, result.output)
            remote.return_value.backup.assert_called_once_with()
            self.assertNotIn("remote source overlaps", result.output)

    def test_unresolvable_dependency_preserves_ssd_failure_and_skips_remote(self):
        self.configure(None)
        original_resolve = Path.resolve
        failed = False

        def fail_copy():
            nonlocal failed
            failed = True
            raise RuntimeError("copy failed")

        def resolve(path, *args, **kwargs):
            if failed and path == self.destination:
                raise PermissionError("destination unavailable")
            return original_resolve(path, *args, **kwargs)

        with (
            mock.patch(
                "photos_backup.cli.backup_all.SsdBackup.backup", side_effect=fail_copy
            ),
            mock.patch.object(Path, "resolve", resolve),
            mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote,
        ):
            result = self.invoke()
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("could not check remote source independence", result.output)
        self.assertIn("copy failed", result.output)
        remote.assert_not_called()

    def test_explicit_skip_uses_existing_ssd_backup_and_remote_skip_keeps_reason(self):
        self.configure(None)
        with (
            mock.patch("photos_backup.cli.backup_all.SsdBackup") as ssd,
            mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote,
        ):
            remote.return_value.backup.return_value = BackupSummary(
                "Remote", files_transferred=0
            )
            result = self.invoke("backup-all", "--skip-ssd")
        self.assertEqual(result.exit_code, 0, result.output)
        ssd.assert_not_called()
        remote.return_value.backup.assert_called_once_with()

        with (
            mock.patch(
                "photos_backup.cli.backup_all.SsdBackup.backup",
                side_effect=RuntimeError("copy failed"),
            ),
            mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote,
        ):
            result = self.invoke("backup-all", "--skip-remote")
        self.assertEqual(result.exit_code, 1, result.output)
        self.assertIn("SKIPPED — Skipped by request", result.output)
        self.assertNotIn("remote source overlaps", result.output)
        remote.assert_not_called()


class SignatureCoverageTests(VerifyTestCase):
    def test_missing_signature_fields_are_explicit_without_changing_exit_policy(self):
        for field in ("dest_size", "dest_mtime"):
            write_export_db(self.paths.export_db, [self.exported])
            with sqlite3.connect(self.paths.export_db) as connection:
                connection.execute(f"UPDATE export_data SET {field} = NULL")
            report = self.verify()
            check = self.check(report, SIGNATURES)
            self.assertTrue(report.passed)
            self.assertIn("Coverage incomplete", check.detail)
            self.assertIn("0 matched, 0 changed, 1 missing signatures", check.detail)
            self.assertIn("file contents were not checksummed", check.detail)

    def test_mixed_coverage_accounts_for_every_record(self):
        changed, unsigned, missing = (
            self.archive_root / name
            for name in ("changed.jpg", "unsigned.jpg", "missing.jpg")
        )
        for path in (changed, unsigned, missing):
            path.write_bytes(b"photo")
        write_export_db(
            self.paths.export_db, [self.exported, changed, unsigned, missing]
        )
        with sqlite3.connect(self.paths.export_db) as connection:
            connection.execute(
                "UPDATE export_data SET dest_size = NULL WHERE uuid = 'uuid-2'"
            )
        changed.write_bytes(b"changed photo")
        missing.unlink()
        check = self.check(self.verify(), SIGNATURES)
        self.assertFalse(check.passed)
        self.assertIn(
            "1 matched, 1 changed, 1 missing signatures, 1 unavailable", check.detail
        )
        self.assertEqual(check.paths, (changed,))


class TransferCountTests(unittest.TestCase):
    def test_absent_statistics_are_not_presented_as_a_successful_zero(self):
        for parse in (parse_rsync_stats, _parse_rclone_stats):
            self.assertIsNone(parse("unrecognized output")["files_transferred"])
        for count in (None, 0):
            for dry_run in (False, True):
                summary = BackupSummary(
                    "Remote", files_transferred=count, dry_run=dry_run
                )
                output = io.StringIO()
                with contextlib.redirect_stdout(output):
                    print_summary(summary)
                    print_pipeline_summary([summary])
                rendered = output.getvalue()
                if count is None:
                    self.assertIn("Transfer count: unavailable", rendered)
                    self.assertNotIn("transfers: 0", rendered)
                    self.assertNotIn("transferred: 0", rendered)
                else:
                    self.assertIn(
                        "Proposed transfers: 0" if dry_run else "Files transferred: 0",
                        rendered,
                    )
                    self.assertNotIn("unavailable", rendered)


if __name__ == "__main__":
    unittest.main()
