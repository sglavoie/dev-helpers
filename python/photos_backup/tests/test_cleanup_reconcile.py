from __future__ import annotations

import unittest
from pathlib import Path

from photos_backup.apple_photos.adapter import ExportedFile
from photos_backup.apple_photos.identity import AssetIdentity, LibraryComparison
from photos_backup.apple_photos.reconcile import (
    ArchiveFile,
    approval_reason,
    plan_reconciliation,
)
from tests.test_bootstrap import asset


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


if __name__ == "__main__":
    unittest.main()
