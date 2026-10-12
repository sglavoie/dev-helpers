from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest import mock


from photos_backup.apple_photos.adapter import ExportedFile
from photos_backup.apple_photos.verify import (
    _inspect_files,
    EXPORT_DATABASE,
    MISSING_ASSETS,
    SIGNATURES,
    STATE,
)

from tests.verify_case import (
    VerifyTestCase,
)


class ExportedFileTests(VerifyTestCase):
    def test_a_directory_replacing_a_photo_fails(self):
        self.exported.unlink()
        self.exported.mkdir()
        report = self.verify()
        self.assertFalse(report.passed)
        self.assertIn("not a regular file", self.check(report, MISSING_ASSETS).detail)
        self.assertTrue(self.check(report, STATE).passed)

    def test_symlinked_files_and_parents_fail_even_when_target_is_unchanged(self):
        original = self.exported.read_bytes()
        for parent in (False, True):
            with self.subTest(parent=parent):
                target = self.root / ("outside-directory" if parent else "outside.jpg")
                if parent:
                    self.exported.parent.rename(target)
                    self.exported.parent.symlink_to(target, target_is_directory=True)
                else:
                    self.exported.rename(target)
                    self.exported.symlink_to(target)
                try:
                    report = self.verify()
                    self.assertFalse(report.passed)
                    self.assertIn("symlink", self.check(report, MISSING_ASSETS).detail)
                    self.assertEqual(self.exported.read_bytes(), original)
                finally:
                    link = self.exported.parent if parent else self.exported
                    link.unlink()
                    target.rename(link)

    def test_unreadable_or_disappearing_file_does_not_abort_other_checks(self):
        lstat = Path.lstat
        for error in (
            PermissionError("permission denied"),
            FileNotFoundError("vanished"),
        ):
            with self.subTest(error=error):

                def inspect(path):
                    if path == self.exported:
                        raise error
                    return lstat(path)

                with mock.patch.object(Path, "lstat", inspect):
                    report = self.verify()
                self.assertEqual(len(report.checks), 6)
                self.assertFalse(report.passed)
                self.assertIn(str(error), self.check(report, MISSING_ASSETS).detail)
                self.assertEqual(
                    self.check(report, MISSING_ASSETS).paths, (self.exported,)
                )
                self.assertTrue(self.check(report, STATE).passed)

    def test_database_probe_failure_does_not_abort_remaining_checks(self):
        def fail(_):
            raise PermissionError("database unavailable")

        report = self.verify(check_integrity=fail)
        self.assertFalse(self.check(report, EXPORT_DATABASE).passed)
        self.assertTrue(self.check(report, STATE).passed)
        self.assertTrue(self.check(report, MISSING_ASSETS).passed)

    def test_a_deleted_exported_file_is_reported_as_missing(self) -> None:
        self.exported.unlink()

        report = self.verify()

        missing = self.check(report, MISSING_ASSETS)
        self.assertFalse(missing.passed)
        self.assertIn(str(self.exported), missing.detail)
        self.assertTrue(self.check(report, SIGNATURES).passed)

    def test_a_changed_exported_file_fails_its_signature(self) -> None:
        self.exported.write_bytes(b"a different photo entirely")

        report = self.verify()

        signatures = self.check(report, SIGNATURES)
        self.assertFalse(signatures.passed)
        self.assertIn(str(self.exported), signatures.detail)
        self.assertTrue(self.check(report, MISSING_ASSETS).passed)


class InspectFilesTests(unittest.TestCase):
    def test_a_shared_parent_directory_is_checked_once(self):
        archive = Path(self.enterContext(tempfile.TemporaryDirectory())).resolve()
        month = archive / "2026" / "08"
        month.mkdir(parents=True)
        files = []
        for index in range(5):
            path = month / f"IMG_{index}.jpg"
            path.write_bytes(b"photo")
            files.append(ExportedFile(path=path, uuid=str(index), size=5, mtime=0))

        original = Path.lstat
        with mock.patch.object(
            Path, "lstat", autospec=True, side_effect=original
        ) as lstat:
            statuses, errors = _inspect_files(tuple(files), archive)

        self.assertEqual(errors, {})
        self.assertEqual(len(statuses), 5)
        # Two directories once each, then one call per file.
        self.assertEqual(lstat.call_count, 2 + 5)
