from __future__ import annotations

import contextlib
import datetime
import tempfile
import unittest
from pathlib import Path

from photos_backup.apple_photos.adapter import PhotosProbes
from photos_backup.apple_photos.bootstrap import (
    FRESH_REASON,
    RESUME_REASON,
    bootstrap_archive,
)
from photos_backup.apple_photos.identity import (
    AssetIdentity,
    WriterStatus,
    assess_coverage,
)
from photos_backup.apple_photos.plan import ExportMode
from photos_backup.archive import ArchiveState, SystemProbes, open_archive
from photos_backup.errors import ActionRequired
from tests.test_export import HOSTNAME, THURSDAY, FakeRunner, make_config, row

LEGACY_EXPORT = Path("/Users/tester/Pictures/export")


def asset(uuid: str) -> AssetIdentity:
    return AssetIdentity(uuid=uuid, cloud_guid=f"guid-{uuid}", original_filename=uuid)


class CoverageTests(unittest.TestCase):
    def test_a_library_fully_recorded_is_complete(self) -> None:
        assets = (asset("a"), asset("b"))

        coverage = assess_coverage(assets, assets)

        self.assertTrue(coverage.complete)
        self.assertEqual(coverage.library, 2)
        self.assertEqual(coverage.recorded, 2)

    def test_an_unrecorded_library_asset_makes_coverage_incomplete(self) -> None:
        coverage = assess_coverage((asset("a"), asset("b")), (asset("a"),))

        self.assertFalse(coverage.complete)
        self.assertEqual([item.uuid for item in coverage.unrecorded], ["b"])

    def test_records_beyond_the_library_do_not_break_coverage(self) -> None:
        coverage = assess_coverage((asset("a"),), (asset("a"), asset("gone")))

        self.assertTrue(coverage.complete)
        self.assertEqual(coverage.recorded, 2)


class BootstrapTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        # Resolved because macOS temporary directories live under symlinks.
        self.root = Path(self._tmpdir.name).resolve()
        self.volume = self.root / "SanDisk"
        self.volume.mkdir()
        self.archive_root = self.volume / "Media" / "Apple Photos"
        self.config = make_config(
            self.volume, self.archive_root, legacy_export=LEGACY_EXPORT
        )
        volume = self.volume
        self.system_probes = SystemProbes(
            is_mount=lambda path: path == volume,
            hostname=lambda: HOSTNAME,
            now=lambda: THURSDAY,
        )

    @contextlib.contextmanager
    def archive(self, *, dry_run: bool = False):
        with open_archive(
            self.config, dry_run=dry_run, probes=self.system_probes
        ) as opened:
            yield opened

    def bootstrap(
        self,
        runner: FakeRunner,
        *,
        library: tuple[AssetIdentity, ...] = (),
        before: tuple[AssetIdentity, ...] = (),
        after: tuple[AssetIdentity, ...] | None = None,
        state: ArchiveState | None = None,
        dry_run: bool = False,
    ):
        """Bootstrap against a fake export database that the fake runner fills."""
        written = library if after is None else after

        def read_export_db(_path: Path) -> tuple[AssetIdentity, ...]:
            return written if runner.arguments is not None else before

        with self.archive(dry_run=dry_run) as opened:
            if state is not None:
                opened.state_store.save(state)
            result = bootstrap_archive(
                self.config,
                opened,
                probes=PhotosProbes(
                    read_library=lambda _: library, read_export_db=read_export_db
                ),
                runner=runner,
                metadata_reader=lambda path: {},
            )
            self.state = opened.state_store.load()
            self.paths = opened.paths
        return result


class FreshBootstrapTests(BootstrapTestCase):
    def test_a_complete_clean_export_initializes_the_archive(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1), row("b.jpg", new=1)])

        result = self.bootstrap(runner, library=(asset("a"), asset("b")))

        self.assertTrue(result.initialized)
        self.assertIsNone(result.blocking_reason())
        self.assertEqual(self.state.initialized_at, THURSDAY)
        self.assertTrue(self.state.initialized)
        self.assertIs(result.export.plan.mode, ExportMode.FULL)
        self.assertEqual(result.export.plan.reason, FRESH_REASON)

    def test_the_bootstrap_claims_the_archive_for_this_mac(self) -> None:
        result = self.bootstrap(FakeRunner())

        self.assertIs(result.takeover.status, WriterStatus.CLAIMED)
        self.assertEqual(self.state.writer_hostname, HOSTNAME)

    def test_nothing_is_imported_from_the_legacy_export(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1)])

        self.bootstrap(runner, library=(asset("a"),))

        self.assertEqual(runner.arguments["dest"], str(self.archive_root))
        self.assertNotIn(str(LEGACY_EXPORT), str(runner.arguments))

    def test_an_export_error_leaves_the_archive_uninitialized(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1), row("b.jpg", error=1)])

        result = self.bootstrap(runner, library=(asset("a"), asset("b")))

        self.assertFalse(result.initialized)
        self.assertIsNone(self.state.initialized_at)
        self.assertIn("not clean", str(result.blocking_reason()))

    def test_an_asset_missing_from_icloud_blocks_initialization(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1), row("b.jpg", missing=1)])

        result = self.bootstrap(runner, library=(asset("a"), asset("b")))

        self.assertTrue(result.export.clean)
        self.assertFalse(result.initialized)
        self.assertIn("could not be downloaded", str(result.blocking_reason()))

    def test_an_unrecorded_library_asset_blocks_initialization(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1)])

        result = self.bootstrap(
            runner, library=(asset("a"), asset("b")), after=(asset("a"),)
        )

        self.assertTrue(result.export.clean)
        self.assertFalse(result.initialized)
        self.assertIn("no export-database record", str(result.blocking_reason()))
        self.assertIsNone(self.state.initialized_at)


class ResumeBootstrapTests(BootstrapTestCase):
    def test_an_interrupted_bootstrap_resumes_with_a_full_export(self) -> None:
        self.archive_root.mkdir(parents=True)
        (self.archive_root / ".photos-backup").mkdir()
        (self.archive_root / ".photos-backup" / "export.db").touch()
        runner = FakeRunner([row("a.jpg", new=1)])

        result = self.bootstrap(runner, library=(asset("a"),))

        self.assertTrue(result.resumed)
        self.assertIs(result.export.plan.mode, ExportMode.FULL)
        self.assertEqual(result.export.plan.reason, RESUME_REASON)
        self.assertTrue(result.initialized)

    def test_a_previous_clean_export_still_resumes_with_a_full_export(self) -> None:
        monday = datetime.datetime(2026, 8, 10, 6, 0, tzinfo=datetime.UTC)
        runner = FakeRunner([row("a.jpg", new=1)])

        result = self.bootstrap(
            runner,
            library=(asset("a"),),
            state=ArchiveState(
                writer_hostname=HOSTNAME,
                last_full_export_at=monday,
                last_successful_export_at=monday,
            ),
        )

        self.assertTrue(result.resumed)
        self.assertIs(result.export.plan.mode, ExportMode.FULL)
        self.assertIsNone(runner.arguments["from_date"])
        self.assertTrue(result.initialized)

    def test_an_initialized_archive_is_refused(self) -> None:
        runner = FakeRunner()

        with self.assertRaises(ActionRequired) as raised:
            self.bootstrap(
                runner,
                library=(asset("a"),),
                state=ArchiveState(initialized_at=THURSDAY),
            )

        self.assertIn("already initialized", str(raised.exception))
        self.assertIn("daily", str(raised.exception))
        self.assertIsNone(runner.arguments)

    def test_an_archive_holding_foreign_content_is_refused(self) -> None:
        self.archive_root.mkdir(parents=True)
        (self.archive_root / "2019").mkdir()
        runner = FakeRunner()

        with self.assertRaises(ActionRequired) as raised:
            self.bootstrap(runner, library=(asset("a"),))

        self.assertIn("no photos-backup metadata", str(raised.exception))
        self.assertIn("'2019'", str(raised.exception))
        self.assertIsNone(runner.arguments)


class DryRunBootstrapTests(BootstrapTestCase):
    def test_a_dry_run_writes_nothing_and_never_initializes(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1)])

        result = self.bootstrap(runner, library=(asset("a"),), after=(), dry_run=True)

        self.assertTrue(runner.arguments["dry_run"])
        self.assertFalse(result.initialized)
        self.assertFalse(self.archive_root.exists())
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_a_dry_run_reports_the_coverage_it_would_have_to_reach(self) -> None:
        result = self.bootstrap(
            FakeRunner(), library=(asset("a"), asset("b")), after=(), dry_run=True
        )

        self.assertEqual(result.coverage.library, 2)
        self.assertEqual(len(result.coverage.unrecorded), 2)
        self.assertIn("no export-database record", str(result.blocking_reason()))


if __name__ == "__main__":
    unittest.main()
