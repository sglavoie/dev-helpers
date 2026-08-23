from __future__ import annotations

import dataclasses
import unittest

from photos_backup.apple_photos.adapter import PhotosProbes
from photos_backup.apple_photos.cleanup import (
    INCREMENTAL_EXPORT,
    MIRROR_DISABLED,
    MirrorStatus,
    read_manifest,
)
from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.plan import ExportMode, ExportPlan, ExportResult
from tests.test_bootstrap import asset
from tests.test_cleanup import MirrorTestCase
from tests.test_export import FakeRunner, ReportlessRunner, row


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

    def test_a_planned_export_never_reconciles_or_deletes(self) -> None:
        planned = ExportResult(
            plan=ExportPlan(ExportMode.FULL, "weekly"),
            exit_code=0,
            performed=False,
        )

        outcome = self.reconcile(result=planned)

        self.assert_nothing_happened(outcome)
        self.assertEqual(outcome.reason, "the export was only planned")

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


if __name__ == "__main__":
    unittest.main()
