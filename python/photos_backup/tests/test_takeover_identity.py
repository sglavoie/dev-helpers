from __future__ import annotations

import unittest

from photos_backup.apple_photos.identity import assess_library, compare_library
from tests.test_takeover import asset


class ComparisonTests(unittest.TestCase):
    def test_records_matched_by_cloud_guid_with_a_new_uuid_are_migratable(self) -> None:
        comparison = compare_library(
            [asset("old-1", "guid-1"), asset("old-2", "guid-2")],
            [asset("new-1", "guid-1"), asset("new-2", "guid-2")],
        )

        self.assertEqual(comparison.recorded, 2)
        self.assertEqual(comparison.matched, 2)
        self.assertEqual(comparison.uuid_changed, 2)
        self.assertEqual(comparison.absent, ())

    def test_the_same_uuid_on_both_sides_needs_no_migration(self) -> None:
        comparison = compare_library(
            [asset("same-1", "guid-1")], [asset("same-1", "guid-1")]
        )

        self.assertEqual(comparison.uuid_changed, 0)

    def test_a_record_whose_cloud_guid_is_gone_is_absent(self) -> None:
        comparison = compare_library(
            [asset("old-1", "guid-1"), asset("old-2", "guid-2")],
            [asset("new-1", "guid-1")],
        )

        self.assertEqual([item.uuid for item in comparison.absent], ["old-2"])
        self.assertEqual(comparison.absent_fraction, 0.5)

    def test_a_matching_cloud_guid_under_another_filename_does_not_match(self) -> None:
        comparison = compare_library(
            [asset("old-1", "guid-1", "IMG_0001.HEIC")],
            [asset("new-1", "guid-1", "IMG_9999.HEIC")],
        )

        self.assertEqual(comparison.matched, 0)
        self.assertEqual(comparison.absent_count, 1)

    def test_records_without_a_cloud_guid_are_unidentifiable_not_absent(self) -> None:
        comparison = compare_library(
            [asset("old-1", None), asset("old-2", "guid-2")],
            [asset("new-2", "guid-2")],
        )

        self.assertEqual(comparison.unidentifiable, 1)
        self.assertEqual(comparison.comparable, 1)
        self.assertEqual(comparison.absent, ())


class VerdictTests(unittest.TestCase):
    def assess(self, recorded, library, **overrides):
        limits = {"max_absent_assets": 10, "max_absent_fraction": 0.001}
        limits.update(overrides)
        return assess_library(compare_library(recorded, library), **limits)

    def test_an_empty_export_database_has_nothing_to_protect(self) -> None:
        verdict = self.assess([], [asset("new-1", "guid-1")])

        self.assertFalse(verdict.blocked)
        self.assertFalse(verdict.migration_required)

    def test_a_library_without_one_shared_asset_is_the_wrong_library(self) -> None:
        verdict = self.assess([asset("old-1", "guid-1")], [asset("new-9", "guid-9")])

        self.assertIn("different library", str(verdict.blocked_reason))

    def test_records_without_cloud_guids_cannot_identify_a_library(self) -> None:
        verdict = self.assess([asset("old-1", None)], [asset("new-1", "guid-1")])

        self.assertIn("cannot be identified", str(verdict.blocked_reason))

    def test_absence_over_the_asset_cap_blocks_below_the_fraction(self) -> None:
        recorded = [asset(f"old-{index}", f"guid-{index}") for index in range(12000)]
        library = [asset(f"new-{index}", f"guid-{index}") for index in range(11989)]

        verdict = self.assess(recorded, library)

        self.assertIn("over the limit", str(verdict.blocked_reason))

    def test_absence_over_the_fraction_blocks_below_the_asset_cap(self) -> None:
        recorded = [asset(f"old-{index}", f"guid-{index}") for index in range(1000)]
        library = [asset(f"new-{index}", f"guid-{index}") for index in range(998)]

        verdict = self.assess(recorded, library)

        self.assertIn("over the limit", str(verdict.blocked_reason))

    def test_a_small_absent_set_becomes_deletion_candidates(self) -> None:
        recorded = [asset(f"old-{index}", f"guid-{index}") for index in range(5000)]
        library = [asset(f"new-{index}", f"guid-{index}") for index in range(4995)]

        verdict = self.assess(recorded, library)

        self.assertFalse(verdict.blocked)
        self.assertTrue(verdict.migration_required)
        self.assertEqual(len(verdict.deletion_candidates), 5)


if __name__ == "__main__":
    unittest.main()
