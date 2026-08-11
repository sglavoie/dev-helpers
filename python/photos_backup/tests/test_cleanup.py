from __future__ import annotations

import contextlib
import dataclasses
import datetime
import os
import tempfile
import unittest
from pathlib import Path

from photos_backup.apple_photos.adapter import ExportedFile, PhotosProbes
from photos_backup.apple_photos.cleanup import (
    ALREADY_MIRRORED,
    INCREMENTAL_EXPORT,
    MIRROR_DISABLED,
    CleanupManifest,
    MirrorStatus,
    approve_cleanup,
    discard_cleanup,
    new_run_id,
    read_manifest,
    reconcile_mirror,
    write_manifest,
)
from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.identity import AssetIdentity, LibraryComparison
from photos_backup.apple_photos.plan import ExportMode, ExportPlan, ExportResult
from photos_backup.apple_photos.reconcile import (
    ArchiveFile,
    CandidateFile,
    approval_reason,
    plan_reconciliation,
)
from photos_backup.archive import (
    ArchivePaths,
    ArchiveState,
    ArchiveStateStore,
    ArchiveUnsafe,
    SystemProbes,
    create_archive_tree,
    open_archive,
    safe_name,
)
from photos_backup.errors import ActionRequired
from tests.test_bootstrap import asset
from tests.test_export import (
    HOSTNAME,
    THURSDAY,
    FakeRunner,
    ReportlessRunner,
    make_config,
    row,
)

OTHER_HOST = "Sebastiens Mac mini.local"
# A library large enough that losing one asset stays under the 0.1% cap.
UNTOUCHED = tuple(asset(f"untouched-{index}") for index in range(1500))
FULL_EXPORT = ExportResult(
    plan=ExportPlan(ExportMode.FULL, "a weekly full export"),
    exit_code=0,
    counts={"new": 1},
    state_advanced=True,
)


def comparison(
    absent: tuple[AssetIdentity, ...], *, recorded: int = 10
) -> LibraryComparison:
    return LibraryComparison(
        recorded=recorded,
        unidentifiable=0,
        matched=recorded - len(absent),
        uuid_changed=0,
        absent=absent,
    )


def record(path: str, uuid: str, *, size: int = 4, mtime: float = 100.0):
    return ExportedFile(path=Path(path), uuid=uuid, size=size, mtime=mtime)


def on_disk(path: str, *, size: int = 4, mtime: float = 100.0) -> ArchiveFile:
    return ArchiveFile(path=Path(path), size=size, mtime=mtime)


def relative(record: ExportedFile, root: Path) -> ExportedFile:
    """The same record in the current osxphotos format, relative to the export root."""
    return dataclasses.replace(record, path=Path(os.path.relpath(record.path, root)))


class ReconciliationTests(unittest.TestCase):
    def test_an_asset_that_left_the_library_becomes_a_candidate(self) -> None:
        plan = plan_reconciliation(
            comparison((asset("gone"),)),
            [record("/a/gone.jpg", "gone"), record("/a/kept.jpg", "kept")],
            [on_disk("/a/gone.jpg"), on_disk("/a/kept.jpg")],
        )

        self.assertEqual([str(path) for path in plan.paths], ["/a/gone.jpg"])
        self.assertEqual(plan.candidates[0].uuid, "gone")
        self.assertEqual(plan.unknown, ())
        self.assertEqual(plan.ambiguous, ())

    def test_a_file_two_assets_claim_is_ambiguous_and_never_a_candidate(self) -> None:
        plan = plan_reconciliation(
            comparison((asset("gone"),)),
            [record("/a/shared.jpg", "gone"), record("/a/shared.jpg", "kept")],
            [on_disk("/a/shared.jpg")],
        )

        self.assertEqual(plan.candidates, ())
        self.assertEqual([str(path) for path in plan.ambiguous], ["/a/shared.jpg"])

    def test_a_file_no_record_claims_is_unknown(self) -> None:
        plan = plan_reconciliation(
            comparison(()),
            [record("/a/kept.jpg", "kept")],
            [on_disk("/a/kept.jpg"), on_disk("/a/stranger.jpg")],
        )

        self.assertEqual(plan.candidates, ())
        self.assertEqual([str(path) for path in plan.unknown], ["/a/stranger.jpg"])

    def test_a_candidate_that_changed_on_disk_is_not_a_candidate(self) -> None:
        plan = plan_reconciliation(
            comparison((asset("gone"),)),
            [record("/a/gone.jpg", "gone", size=4)],
            [on_disk("/a/gone.jpg", size=9)],
        )

        self.assertEqual(plan.candidates, ())
        self.assertEqual([str(path) for path in plan.changed], ["/a/gone.jpg"])

    def test_a_record_without_a_signature_is_treated_as_changed(self) -> None:
        plan = plan_reconciliation(
            comparison((asset("gone"),)),
            [
                ExportedFile(
                    path=Path("/a/gone.jpg"), uuid="gone", size=None, mtime=None
                )
            ],
            [on_disk("/a/gone.jpg")],
        )

        self.assertEqual(plan.candidates, ())
        self.assertEqual([str(path) for path in plan.changed], ["/a/gone.jpg"])

    def test_a_record_whose_file_is_already_gone_is_not_a_candidate(self) -> None:
        plan = plan_reconciliation(
            comparison((asset("gone"),)), [record("/a/gone.jpg", "gone")], []
        )

        self.assertEqual(plan.candidates, ())
        self.assertEqual(plan.changed, ())
        self.assertEqual(plan.unknown, ())


class ApprovalReasonTests(unittest.TestCase):
    def reason(self, plan, library, **overrides) -> str | None:
        limits = {"max_absent_assets": 10, "max_absent_fraction": 0.001}
        limits.update(overrides)
        return approval_reason(plan, library, **limits)

    def test_a_small_explained_reconciliation_needs_no_approval(self) -> None:
        library = comparison((asset("gone"),), recorded=10000)
        plan = plan_reconciliation(
            library, [record("/a/gone.jpg", "gone")], [on_disk("/a/gone.jpg")]
        )

        self.assertIsNone(self.reason(plan, library))

    def test_unexplained_archive_files_need_approval(self) -> None:
        library = comparison((), recorded=10000)
        plan = plan_reconciliation(library, [], [on_disk("/a/stranger.jpg")])

        self.assertIn("no export-database record", str(self.reason(plan, library)))

    def test_ambiguous_files_need_approval(self) -> None:
        library = comparison((asset("gone"),), recorded=10000)
        plan = plan_reconciliation(
            library,
            [record("/a/shared.jpg", "gone"), record("/a/shared.jpg", "kept")],
            [on_disk("/a/shared.jpg")],
        )

        self.assertIn("more than one", str(self.reason(plan, library)))

    def test_changed_candidates_need_approval(self) -> None:
        library = comparison((asset("gone"),), recorded=10000)
        plan = plan_reconciliation(
            library,
            [record("/a/gone.jpg", size=4, uuid="gone")],
            [on_disk("/a/gone.jpg", size=9)],
        )

        self.assertIn("no longer carry", str(self.reason(plan, library)))

    def test_too_many_absent_assets_need_approval(self) -> None:
        absent = tuple(asset(f"gone-{index}") for index in range(11))
        library = comparison(absent, recorded=100000)
        plan = plan_reconciliation(library, [], [])

        reason = str(self.reason(plan, library))
        self.assertIn("11 of 100000", reason)
        self.assertIn("limit of 10 asset(s)", reason)

    def test_too_large_a_fraction_needs_approval(self) -> None:
        absent = tuple(asset(f"gone-{index}") for index in range(5))
        library = comparison(absent, recorded=2000)
        plan = plan_reconciliation(library, [], [])

        self.assertIn("0.250%", str(self.reason(plan, library)))


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


class RefusedMirrorTests(MirrorTestCase):
    def assert_nothing_happened(self, outcome) -> None:
        self.assertIs(outcome.status, MirrorStatus.SKIPPED)
        self.assertTrue(self.gone.is_file())
        self.assertIsNone(self.state.last_mirror_completed_at)
        self.assertEqual(self.manifests(), [])

    def export(self, runner) -> ExportResult:
        with self.archive() as opened:
            return ApplePhotosExport(
                config=self.config,
                archive=opened,
                runner=runner,
                plan=ExportPlan(ExportMode.FULL, "a weekly full export"),
                metadata_reader=lambda path: {},
            ).export()

    def test_a_stale_same_day_report_never_stands_in_for_a_reportless_run(self) -> None:
        first = self.export(FakeRunner([row("a.jpg", new=1)]))
        second = self.export(ReportlessRunner())

        outcome = self.reconcile(result=second)

        self.assertTrue(first.clean)
        self.assert_nothing_happened(outcome)
        self.assertIn("wrote no report", outcome.reason)
        self.assertNotEqual(second.report_path, first.report_path)
        self.assertTrue(first.report_path.is_file())

    def test_an_incremental_export_never_deletes(self) -> None:
        incremental = ExportResult(
            plan=ExportPlan(ExportMode.INCREMENTAL, "since Monday"), exit_code=0
        )

        outcome = self.reconcile(result=incremental)

        self.assert_nothing_happened(outcome)
        self.assertEqual(outcome.reason, INCREMENTAL_EXPORT)

    def test_an_export_that_reported_errors_never_deletes(self) -> None:
        errored = ExportResult(
            plan=ExportPlan(ExportMode.FULL, "weekly"),
            exit_code=0,
            counts={"error": 1},
        )

        outcome = self.reconcile(result=errored)

        self.assert_nothing_happened(outcome)
        self.assertIn("not clean", outcome.reason)

    def test_an_export_without_a_readable_report_never_deletes(self) -> None:
        unreported = ExportResult(
            plan=ExportPlan(ExportMode.FULL, "weekly"),
            exit_code=0,
            report_problem="osxphotos wrote no report at '/tmp/missing.csv'",
        )

        outcome = self.reconcile(result=unreported)

        self.assert_nothing_happened(outcome)
        self.assertIn("wrote no report", outcome.reason)

    def test_assets_missing_from_icloud_stop_the_reconciliation(self) -> None:
        incomplete = ExportResult(
            plan=ExportPlan(ExportMode.FULL, "weekly"),
            exit_code=0,
            counts={"missing": 2},
        )

        outcome = self.reconcile(result=incomplete)

        self.assert_nothing_happened(outcome)
        self.assertIn("missing from iCloud", outcome.reason)

    def test_a_disabled_mirror_never_deletes(self) -> None:
        outcome = self.reconcile(config=dataclasses.replace(self.config, mirror=False))

        self.assert_nothing_happened(outcome)
        self.assertEqual(outcome.reason, MIRROR_DISABLED)

    def test_a_dry_run_previews_without_writing_anything(self) -> None:
        outcome = self.reconcile(dry_run=True)

        self.assertIs(outcome.status, MirrorStatus.PREVIEW)
        self.assertEqual(outcome.reconciliation.paths, (self.gone,))
        self.assertTrue(self.gone.is_file())
        self.assertIsNone(self.state.last_mirror_completed_at)
        self.assertEqual(self.manifests(), [])


class PendingMirrorTests(MirrorTestCase):
    def over_the_limit(self) -> PhotosProbes:
        """Eleven assets left the library, one more than the configured cap."""
        absent = tuple(asset(f"gone-{index}") for index in range(11))
        return self.probes(
            library=(asset("kept"),),
            recorded=(asset("kept"), *absent),
            files=(
                self.exported(self.kept, "kept"),
                self.exported(self.gone, "gone-0"),
            ),
        )

    def test_too_many_deletions_are_left_pending(self) -> None:
        outcome = self.reconcile(probes=self.over_the_limit())

        self.assertIs(outcome.status, MirrorStatus.PENDING)
        self.assertTrue(outcome.pending)
        self.assertTrue(self.gone.is_file())
        self.assertIsNone(self.state.last_mirror_completed_at)
        self.assertEqual(self.state.pending_cleanup_run_id, outcome.run_id)
        self.assertEqual(self.manifests(), [outcome.manifest_path])

    def test_the_manifest_records_the_exact_candidates_and_the_reason(self) -> None:
        outcome = self.reconcile(probes=self.over_the_limit())

        manifest = read_manifest(outcome.manifest_path)
        self.assertEqual(manifest.run_id, outcome.run_id)
        self.assertEqual(manifest.archive, self.archive_root)
        self.assertEqual([item.path for item in manifest.candidates], [self.gone])
        self.assertEqual(len(manifest.assets), 11)
        self.assertIn("limit of 10 asset(s)", manifest.reason)

    def test_an_unexplained_archive_file_is_left_pending(self) -> None:
        stranger = self.write_photo("2026/08/stranger.jpg", b"not ours")

        outcome = self.reconcile()

        self.assertIs(outcome.status, MirrorStatus.PENDING)
        self.assertIn("no export-database record", outcome.reason)
        self.assertEqual(outcome.reconciliation.unknown, (stranger,))
        self.assertTrue(self.gone.is_file())

    def test_a_pending_run_is_never_recomputed_or_rewritten(self) -> None:
        first = self.reconcile(probes=self.over_the_limit())

        second = self.reconcile()

        self.assertIs(second.status, MirrorStatus.PENDING)
        self.assertEqual(second.run_id, first.run_id)
        self.assertIsNone(second.reconciliation)
        self.assertEqual(self.manifests(), [first.manifest_path])
        self.assertTrue(self.gone.is_file())


class ApprovalTests(MirrorTestCase):
    def setUp(self) -> None:
        super().setUp()
        self.write_photo("2026/08/stranger.jpg", b"not ours")
        self.pending = self.reconcile()
        self.run_id = self.pending.run_id

    def test_approving_deletes_exactly_the_reviewed_candidates(self) -> None:
        approval = self.approve(self.run_id)

        self.assertEqual(approval.deleted, (self.gone,))
        self.assertFalse(self.gone.exists())
        self.assertTrue(self.kept.is_file())
        self.assertTrue(approval.complete)
        self.assertIsNone(self.state.pending_cleanup_run_id)
        self.assertEqual(self.state.last_mirror_completed_at, THURSDAY)

    def test_an_unexplained_file_is_never_deleted_by_an_approval(self) -> None:
        self.approve(self.run_id)

        self.assertTrue((self.archive_root / "2026" / "08" / "stranger.jpg").is_file())

    def test_a_candidate_that_changed_since_the_manifest_is_kept(self) -> None:
        self.gone.write_bytes(b"someone edited this file after the export")

        approval = self.approve(self.run_id)

        self.assertEqual(approval.deleted, ())
        self.assertEqual(approval.stale, (self.gone,))
        self.assertTrue(self.gone.is_file())
        self.assertIsNone(self.state.pending_cleanup_run_id)

    def test_a_candidate_whose_record_was_refreshed_too_is_kept(self) -> None:
        self.gone.write_bytes(b"a replacement file, exported and recorded again")
        os.utime(self.gone, (200.0, 200.0))
        refreshed = self.probes(
            library=(asset("kept"), *UNTOUCHED),
            recorded=(asset("kept"), asset("gone"), *UNTOUCHED),
            files=(
                self.exported(self.kept, "kept"),
                self.exported(self.gone, "gone"),
            ),
        )

        approval = self.approve(self.run_id, probes=refreshed)

        self.assertEqual(approval.deleted, ())
        self.assertEqual(approval.stale, (self.gone,))
        self.assertEqual(approval.unreviewed, (self.gone,))
        self.assertTrue(self.gone.is_file())
        self.assertFalse(approval.complete)
        self.assertIsNone(self.state.last_mirror_completed_at)

    def test_a_candidate_nobody_reviewed_is_kept_and_the_mirror_stays_open(
        self,
    ) -> None:
        approval = self.approve(
            self.run_id,
            probes=self.probes(
                library=(),
                recorded=(asset("kept"), asset("gone")),
                files=(
                    self.exported(self.kept, "kept"),
                    self.exported(self.gone, "gone"),
                ),
            ),
        )

        self.assertEqual(approval.deleted, (self.gone,))
        self.assertEqual(approval.unreviewed, (self.kept,))
        self.assertTrue(self.kept.is_file())
        self.assertFalse(approval.complete)
        self.assertIsNone(self.state.last_mirror_completed_at)

    def test_approving_another_run_refuses_and_deletes_nothing(self) -> None:
        with self.assertRaises(ActionRequired) as raised:
            self.approve("some-other-run")

        self.assertIn(self.run_id, str(raised.exception))
        self.assertTrue(self.gone.is_file())

    def test_approving_with_nothing_pending_refuses(self) -> None:
        self.save_state()

        with self.assertRaises(ActionRequired) as raised:
            self.approve(self.run_id)

        self.assertIn("No cleanup is waiting", str(raised.exception))
        self.assertTrue(self.gone.is_file())

    def test_another_mac_may_not_approve(self) -> None:
        self.save_state(writer_hostname=OTHER_HOST, pending_cleanup_run_id=self.run_id)

        with self.assertRaises(ActionRequired) as raised:
            self.approve(self.run_id)

        self.assertIn(OTHER_HOST, str(raised.exception))
        self.assertIn("daily", str(raised.exception))
        self.assertTrue(self.gone.is_file())

    def test_a_malformed_manifest_refuses(self) -> None:
        self.paths.cleanup_manifest(self.run_id).write_text('{"version": 99}')

        with self.assertRaises(ArchiveUnsafe) as raised:
            self.approve(self.run_id)

        self.assertIn("malformed", str(raised.exception))
        self.assertTrue(self.gone.is_file())

    def test_a_vanished_manifest_refuses(self) -> None:
        self.paths.cleanup_manifest(self.run_id).unlink()

        with self.assertRaises(ActionRequired) as raised:
            self.approve(self.run_id)

        self.assertIn("is gone", str(raised.exception))
        self.assertTrue(self.gone.is_file())


class DiscardTests(MirrorTestCase):
    def setUp(self) -> None:
        super().setUp()
        self.write_photo("2026/08/stranger.jpg", b"not ours")
        self.run_id = self.reconcile().run_id

    def test_discarding_deletes_nothing_and_clears_the_pending_run(self) -> None:
        discarded = self.discard(self.run_id)

        self.assertEqual(discarded.run_id, self.run_id)
        self.assertTrue(self.gone.is_file())
        self.assertTrue(self.kept.is_file())
        self.assertIsNone(self.state.pending_cleanup_run_id)
        self.assertIsNone(self.state.last_mirror_completed_at)

    def test_a_discarded_manifest_stays_as_the_record_of_what_was_rejected(
        self,
    ) -> None:
        discarded = self.discard(self.run_id)

        self.assertTrue(discarded.manifest_path.is_file())
        self.assertEqual(discarded.manifest.run_id, self.run_id)

    def test_a_later_full_export_may_propose_the_deletions_again(self) -> None:
        self.discard(self.run_id)

        outcome = self.reconcile()

        self.assertIs(outcome.status, MirrorStatus.PENDING)
        self.assertNotEqual(outcome.run_id, self.run_id)
        self.assertEqual(len(self.manifests()), 2)

    def test_discarding_another_run_refuses(self) -> None:
        with self.assertRaises(ActionRequired) as raised:
            self.discard("some-other-run")

        self.assertIn(self.run_id, str(raised.exception))

    def test_another_mac_may_discard_because_nothing_is_deleted(self) -> None:
        self.save_state(writer_hostname=OTHER_HOST, pending_cleanup_run_id=self.run_id)

        discarded = self.discard(self.run_id)

        self.assertEqual(discarded.run_id, self.run_id)
        self.assertTrue(self.gone.is_file())


class ManifestTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        self.path = Path(self._tmpdir.name) / "run-7.json"
        self.manifest = CleanupManifest(
            run_id="run-7",
            created_at=THURSDAY,
            hostname=HOSTNAME,
            archive=Path("/Volumes/SanDisk/Media/Apple Photos"),
            reason="11 record(s) left the library",
            assets=(asset("gone"),),
            candidates=(
                CandidateFile(
                    path=Path("/Volumes/SanDisk/Media/Apple Photos/2019/gone.jpg"),
                    uuid="gone",
                    size=17,
                    mtime=1000.5,
                ),
            ),
            changed=(Path("/a/changed.jpg"),),
            unknown=(Path("/a/stranger.jpg"),),
            ambiguous=(Path("/a/shared.jpg"),),
        )

    def test_a_manifest_round_trips(self) -> None:
        write_manifest(self.path, self.manifest)

        self.assertEqual(read_manifest(self.path), self.manifest)

    def test_a_manifest_is_never_rewritten(self) -> None:
        write_manifest(self.path, self.manifest)

        with self.assertRaises(ArchiveUnsafe) as raised:
            write_manifest(self.path, self.manifest)

        self.assertIn("never rewritten", str(raised.exception))

    def test_an_unknown_key_is_refused(self) -> None:
        write_manifest(self.path, self.manifest)
        self.path.write_text(self.path.read_text().replace('"reason"', '"surprise"', 1))

        with self.assertRaises(ArchiveUnsafe) as raised:
            read_manifest(self.path)

        self.assertIn("surprise", str(raised.exception))


class RunIdTests(unittest.TestCase):
    def test_a_run_id_survives_sanitizing_and_names_host_and_time(self) -> None:
        run_id = new_run_id(HOSTNAME, datetime.datetime(2026, 8, 13, 9, 30, 15))

        self.assertEqual(run_id, safe_name(run_id))
        self.assertIn("20260813T093015", run_id)


if __name__ == "__main__":
    unittest.main()
