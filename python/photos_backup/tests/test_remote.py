from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.errors import ActionRequired
from photos_backup.summary import BackupSummary
from tests import isolate_transfer_history


class RemoteBehaviorTests(unittest.TestCase):
    def setUp(self):
        isolate_transfer_history(self)
        self.enterContext(
            mock.patch(
                "photos_backup.cli.backup_all.which", return_value="/test/bin/tool"
            )
        )
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
        # Deletions refuse to mirror an empty source.
        for directory in (self.source, self.destination):
            (directory / "IMG_0001.JPG").write_text("photo")
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


if __name__ == "__main__":
    unittest.main()
