from __future__ import annotations

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
from photos_backup.config import SsdConfig
from photos_backup.errors import ActionRequired
from photos_backup.ssd.backup import Backup


class SsdLockingTests(unittest.TestCase):
    def setUp(self):
        temporary = self.enterContext(tempfile.TemporaryDirectory())
        self.root = Path(temporary).resolve()
        self.source = self.root / "photos"
        self.paths = ArchivePaths(self.root, self.source)
        self.paths.metadata.mkdir(parents=True)
        self.paths.lock_file.write_text("previous writer\n")
        self.destination = self.root / "backup"
        self.probes = SystemProbes(hostname=lambda: "test.local")

    def backup(self, *, source=None, dry_run=False):
        return Backup(
            SsdConfig(source or self.source, self.destination, None), False, dry_run
        )

    def writer_exit_code(self):
        # An independent process uses the same advisory lock protocol as daily.
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

    def test_copy_blocks_writer_and_releases_lock_after_success_failure_or_interrupt(
        self,
    ):
        before = self.paths.lock_file.read_bytes()
        for failure in (
            None,
            subprocess.CalledProcessError(23, ["rsync"]),
            KeyboardInterrupt(),
        ):
            with self.subTest(failure=failure):

                def transfer(*args, **kwargs):
                    self.assertEqual(self.writer_exit_code(), 3)
                    # Readers can still inspect the archive during the copy.
                    with archive_lock(self.paths, self.probes, dry_run=True):
                        pass
                    if failure is not None:
                        raise failure
                    return subprocess.CompletedProcess([], 0, stdout="")

                with mock.patch(
                    "photos_backup.ssd.backup.stream_command", side_effect=transfer
                ):
                    if isinstance(failure, KeyboardInterrupt):
                        with self.assertRaises(KeyboardInterrupt):
                            self.backup().backup()
                    else:
                        results = self.backup().backup()
                        self.assertEqual(bool(results[0].error), failure is not None)
                self.assertEqual(self.writer_exit_code(), 0)
                self.assertEqual(self.paths.lock_file.read_bytes(), before)

    def test_busy_writer_blocks_real_and_preview_copies_before_destination_creation(
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
                    mock.patch("photos_backup.ssd.backup.stream_command") as run,
                ):
                    with self.assertRaisesRegex(ActionRequired, "archive lock"):
                        self.backup(source=source, dry_run=dry_run).backup()
                    run.assert_not_called()
                    self.assertFalse(self.destination.exists())

    def test_missing_or_symlinked_lock_is_refused_without_creating_it(self):
        self.paths.lock_file.unlink()
        other = self.root / "other-lock"
        other.write_text("keep")
        for symlink in (False, True):
            if symlink:
                self.paths.lock_file.symlink_to(other)
            with mock.patch("photos_backup.ssd.backup.stream_command") as run:
                with self.assertRaises(ActionRequired):
                    self.backup().backup()
                run.assert_not_called()
                self.assertFalse(self.destination.exists())
        self.assertEqual(other.read_text(), "keep")

    def test_busy_archive_is_action_required_and_blocks_dependent_remote(self):
        config = self.root / "config.toml"
        config.write_text(
            f'[ssd]\nsource = "{self.source}"\ndestination = "{self.destination}"\n'
            '[rclone]\nremote = "b2:photos"\n'
        )
        for command in ("ssd", "backup-all"):
            with (
                self.subTest(command=command),
                archive_lock(self.paths, self.probes),
                mock.patch("photos_backup.cli.backup_all.which", return_value="/tool"),
                mock.patch("photos_backup.cli.backup_all.RemoteBackup") as remote,
                mock.patch("photos_backup.ssd.backup.stream_command") as run,
            ):
                args = ["--config", str(config), command]
                if command == "backup-all":
                    args.append("--skip-apple-photos")
                result = CliRunner().invoke(cli, args)
            self.assertEqual(result.exit_code, 3, result.output)
            run.assert_not_called()
            remote.assert_not_called()

    def test_preview_holds_lock_without_creating_destination_or_changing_source(self):
        before = self.paths.lock_file.read_bytes()

        def transfer(*args, **kwargs):
            self.assertEqual(self.writer_exit_code(), 3)
            return subprocess.CompletedProcess([], 0, stdout="")

        with mock.patch(
            "photos_backup.ssd.backup.stream_command", side_effect=transfer
        ):
            self.backup(dry_run=True).backup()
        self.assertFalse(self.destination.exists())
        self.assertEqual(self.paths.lock_file.read_bytes(), before)
