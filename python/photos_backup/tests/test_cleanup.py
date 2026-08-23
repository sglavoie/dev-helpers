from __future__ import annotations

import contextlib
import dataclasses
import os
import tempfile
import unittest
from pathlib import Path

from photos_backup.apple_photos.adapter import ExportedFile, PhotosProbes
from photos_backup.apple_photos.cleanup import (
    ALREADY_MIRRORED,
    MirrorStatus,
    approve_cleanup,
    discard_cleanup,
    reconcile_mirror,
)
from photos_backup.apple_photos.plan import ExportMode, ExportPlan, ExportResult
from photos_backup.archive import (
    ArchivePaths,
    ArchiveState,
    ArchiveStateStore,
    SystemProbes,
    create_archive_tree,
    open_archive,
)
from tests.test_bootstrap import asset
from tests.test_export import HOSTNAME, THURSDAY, make_config

# A library large enough that losing one asset stays under the 0.1% cap.
UNTOUCHED = tuple(asset(f"untouched-{index}") for index in range(1500))
FULL_EXPORT = ExportResult(
    plan=ExportPlan(ExportMode.FULL, "a weekly full export"),
    exit_code=0,
    counts={"new": 1},
    state_advanced=True,
)


def relative(record: ExportedFile, root: Path) -> ExportedFile:
    """The same record in the current osxphotos format, relative to the export root."""
    return dataclasses.replace(record, path=Path(os.path.relpath(record.path, root)))


class MirrorTestCase(unittest.TestCase):
    """Gives every test an initialized archive holding two exported photos."""

    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        # Resolved because macOS temporary directories live under symlinks.
        self.root = Path(self._tmpdir.name).resolve()
        self.volume = self.root / "SanDisk"
        self.volume.mkdir()
        self.archive_root = self.volume / "Media" / "Apple Photos"
        self.config = make_config(self.volume, self.archive_root)
        volume = self.volume
        self.system_probes = SystemProbes(
            is_mount=lambda path: path == volume,
            hostname=lambda: HOSTNAME,
            now=lambda: THURSDAY,
        )
        self.paths = ArchivePaths(volume=self.volume, archive=self.archive_root)
        create_archive_tree(self.paths)
        self.save_state()

        self.kept = self.write_photo("2026/08/kept.jpg", b"a kept photo")
        self.gone = self.write_photo("2019/07/gone.jpg", b"a deleted photo")
        # Recorded once, so a later edit to a file no longer matches its record.
        self.records = (
            self.exported(self.kept, "kept"),
            self.exported(self.gone, "gone"),
        )

    def save_state(self, **changes) -> None:
        fields = {
            "initialized_at": THURSDAY,
            "last_successful_export_at": THURSDAY,
            "last_full_export_at": THURSDAY,
            "writer_hostname": HOSTNAME,
        }
        fields.update(changes)
        ArchiveStateStore(self.paths).save(ArchiveState(**fields))

    def write_photo(self, relative: str, content: bytes) -> Path:
        path = self.archive_root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(content)
        return path

    def exported(self, path: Path, uuid: str) -> ExportedFile:
        status = path.stat()
        return ExportedFile(
            path=path, uuid=uuid, size=status.st_size, mtime=status.st_mtime
        )

    def probes(self, *, library, recorded, files) -> PhotosProbes:
        return PhotosProbes(
            read_library=lambda _: library,
            read_export_db=lambda _: recorded,
            read_export_files=lambda _: files,
        )

    @contextlib.contextmanager
    def archive(self, *, dry_run: bool = False):
        with open_archive(
            self.config, dry_run=dry_run, probes=self.system_probes
        ) as opened:
            yield opened
            self.state = opened.state_store.load()

    def default_probes(self) -> PhotosProbes:
        """The library kept one photo and lost the other."""
        return self.probes(
            library=(asset("kept"), *UNTOUCHED),
            recorded=(asset("kept"), asset("gone"), *UNTOUCHED),
            files=self.records,
        )

    def reconcile(
        self,
        *,
        probes: PhotosProbes | None = None,
        result: ExportResult = FULL_EXPORT,
        dry_run: bool = False,
        config=None,
    ):
        with self.archive(dry_run=dry_run) as opened:
            return reconcile_mirror(
                config or self.config,
                opened,
                result,
                probes=probes or self.default_probes(),
            )

    def approve(self, run_id: str, *, probes: PhotosProbes | None = None):
        with self.archive() as opened:
            return approve_cleanup(
                self.config, opened, run_id, probes=probes or self.default_probes()
            )

    def discard(self, run_id: str):
        with self.archive() as opened:
            return discard_cleanup(opened, run_id)

    def manifests(self) -> list[Path]:
        return sorted(self.paths.cleanup.glob("*.json"))


class AutomaticMirrorTests(MirrorTestCase):
    def test_a_clean_full_export_deletes_what_the_library_lost(self) -> None:
        outcome = self.reconcile()

        self.assertIs(outcome.status, MirrorStatus.APPLIED)
        self.assertEqual(outcome.deleted, (self.gone,))
        self.assertFalse(self.gone.exists())
        self.assertTrue(self.kept.is_file())
        self.assertEqual(self.state.last_mirror_completed_at, THURSDAY)
        self.assertIsNone(self.state.pending_cleanup_run_id)
        self.assertEqual(self.manifests(), [])

    def test_the_directories_a_deletion_empties_are_pruned(self) -> None:
        self.reconcile()

        self.assertFalse((self.archive_root / "2019").exists())
        self.assertTrue((self.archive_root / "2026" / "08").is_dir())
        self.assertTrue(self.paths.reports.is_dir())

    def test_an_archive_that_already_mirrors_the_library_advances_the_mirror(
        self,
    ) -> None:
        outcome = self.reconcile(
            probes=self.probes(
                library=(asset("kept"), asset("gone")),
                recorded=(asset("kept"), asset("gone")),
                files=self.records,
            )
        )

        self.assertIs(outcome.status, MirrorStatus.CLEAN)
        self.assertEqual(outcome.reason, ALREADY_MIRRORED)
        self.assertTrue(self.gone.is_file())
        self.assertEqual(self.state.last_mirror_completed_at, THURSDAY)

    def test_macos_metadata_is_neither_deleted_nor_unexplained(self) -> None:
        stray = self.write_photo("2026/08/.DS_Store", b"finder")

        outcome = self.reconcile()

        self.assertIs(outcome.status, MirrorStatus.APPLIED)
        self.assertEqual(outcome.reconciliation.unknown, ())
        self.assertTrue(stray.is_file())


class RelativeRecordTests(MirrorTestCase):
    """The format current osxphotos writes: every filepath relative to the root."""

    def with_files(self, files: tuple[ExportedFile, ...]) -> PhotosProbes:
        return self.probes(
            library=(asset("kept"), *UNTOUCHED),
            recorded=(asset("kept"), asset("gone"), *UNTOUCHED),
            files=files,
        )

    def test_relative_records_recognize_the_files_in_the_archive(self) -> None:
        files = tuple(relative(record, self.archive_root) for record in self.records)

        outcome = self.reconcile(probes=self.with_files(files))

        self.assertIs(outcome.status, MirrorStatus.APPLIED)
        self.assertEqual(outcome.deleted, (self.gone,))
        self.assertEqual(outcome.reconciliation.unknown, ())
        self.assertFalse(self.gone.exists())
        self.assertTrue(self.kept.is_file())

    def test_a_record_that_traverses_out_of_the_archive_deletes_nothing(self) -> None:
        escaped = dataclasses.replace(self.records[1], path=Path("../../gone.jpg"))
        files = (relative(self.records[0], self.archive_root), escaped)

        outcome = self.reconcile(probes=self.with_files(files))

        self.assertIs(outcome.status, MirrorStatus.PENDING)
        self.assertIn("no export-database record", outcome.reason)
        self.assertEqual(outcome.reconciliation.unknown, (self.gone,))
        self.assertTrue(self.gone.is_file())


if __name__ == "__main__":
    unittest.main()
