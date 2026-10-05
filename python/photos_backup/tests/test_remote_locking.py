import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.archive import ArchivePaths, SystemProbes
from photos_backup.archive.lock import archive_lock
from photos_backup.cli.cli import cli
from photos_backup.config import RcloneConfig
from photos_backup.errors import ActionRequired
from photos_backup.remote.backup import Backup
from tests import isolate_transfer_history


class RemoteLockingTests(unittest.TestCase):
    def setUp(self):
        self.root = Path(self.enterContext(tempfile.TemporaryDirectory())).resolve()
        self.source = self.root / "photos"
        self.paths = ArchivePaths(self.root, self.source)
        self.paths.metadata.mkdir(parents=True)
        self.paths.lock_file.write_text("previous writer\n")
        self.probes = SystemProbes(hostname=lambda: "test.local")
        isolate_transfer_history(self)
        self.enterContext(
            mock.patch(
                "photos_backup.remote.backup.shutil.which", return_value="rclone"
            )
        )

    def backup(self, source=None, dry_run=False):
        return Backup(
            RcloneConfig("b2:photos", source or self.source),
            source or self.source,
            dry_run,
        )

    def writer_exit_code(self):
        script = """
import fcntl, sys
with open(sys.argv[1], 'r+') as handle:
    try:
        fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        sys.exit(3)
"""
        return subprocess.run(
            [sys.executable, "-c", script, str(self.paths.lock_file)], check=False
        ).returncode

    def test_busy_writer_blocks_archive_alias_and_nested_uploads_including_previews(
        self,
    ):
        alias = self.root / "alias"
        alias.symlink_to(self.source)
        nested = self.source / "2026"
        nested.mkdir()
        for source in (self.source, alias, nested):
            for dry_run in (False, True):
                with (
                    self.subTest(source=source, dry_run=dry_run),
                    archive_lock(self.paths, self.probes),
                    mock.patch("photos_backup.remote.backup.stream_command") as run,
                ):
                    with self.assertRaisesRegex(
                        ActionRequired, "Remote source archive"
                    ):
                        self.backup(source, dry_run).backup()
                    run.assert_not_called()

    def test_upload_locks_writer_and_releases_after_success_failure_or_interrupt(self):
        before = self.paths.lock_file.read_bytes()
        for failure in (None, OSError("transfer failed"), KeyboardInterrupt()):
            for dry_run in (False, True):
                with self.subTest(failure=failure, dry_run=dry_run):

                    def transfer(*args, **kwargs):
                        self.assertEqual(self.writer_exit_code(), 3)
                        with archive_lock(self.paths, self.probes, dry_run=True):
                            pass
                        if failure is not None:
                            raise failure
                        return subprocess.CompletedProcess([], 0, stdout="")

                    with mock.patch(
                        "photos_backup.remote.backup.stream_command",
                        side_effect=transfer,
                    ):
                        if failure is None:
                            self.backup(dry_run=dry_run).backup()
                        else:
                            with self.assertRaises(
                                KeyboardInterrupt
                                if isinstance(failure, KeyboardInterrupt)
                                else Exception
                            ):
                                self.backup(dry_run=dry_run).backup()
                    self.assertEqual(self.writer_exit_code(), 0)
                    self.assertEqual(self.paths.lock_file.read_bytes(), before)

    def test_missing_and_symlinked_locks_are_refused(self):
        self.paths.lock_file.unlink()
        other = self.root / "other"
        other.write_text("keep")
        for symlink in (False, True):
            if symlink:
                self.paths.lock_file.symlink_to(other)
            with mock.patch("photos_backup.remote.backup.stream_command") as run:
                with self.assertRaises(ActionRequired):
                    self.backup().backup()
                run.assert_not_called()
        self.assertEqual(other.read_text(), "keep")

    def test_busy_writer_returns_exit_three_for_remote_and_pipeline(self):
        config = self.root / "config.toml"
        config.write_text(f'[rclone]\nremote = "b2:photos"\nsource = "{self.source}"\n')
        for command in ("remote", "backup-all"):
            with (
                archive_lock(self.paths, self.probes),
                mock.patch("photos_backup.cli.backup_all.which", return_value="rclone"),
                mock.patch("photos_backup.remote.backup.stream_command") as run,
            ):
                args = ["--config", str(config), command]
                if command == "backup-all":
                    args += ["--skip-apple-photos", "--skip-sd-card", "--skip-ssd"]
                result = CliRunner().invoke(cli, args)
            self.assertEqual(result.exit_code, 3, result.output)
            run.assert_not_called()
