from __future__ import annotations

import os
import unittest
from unittest import mock

from click.testing import CliRunner

from photos_backup.apple_photos.cleanup import MirrorStatus, approve_cleanup
from photos_backup.archive import ArchiveUnsafe
from photos_backup.cli.cli import cli
from photos_backup.errors import ActionRequired
from tests.test_bootstrap import asset
from tests.test_cleanup import UNTOUCHED, MirrorTestCase
from tests.test_export import THURSDAY

OTHER_HOST = "Sebastiens Mac mini.local"


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
        assert discarded.manifest is not None
        self.assertEqual(discarded.manifest.run_id, self.run_id)

    def test_a_missing_or_malformed_manifest_can_still_be_discarded(self) -> None:
        for name, damage in (
            ("missing", lambda path: path.unlink()),
            ("malformed", lambda path: path.write_text("{")),
        ):
            with self.subTest(name):
                self.save_state(pending_cleanup_run_id=self.run_id)
                manifest = self.paths.cleanup_manifest(self.run_id)
                damage(manifest)

                discarded = self.discard(self.run_id)

                self.assertIsNone(discarded.manifest)
                self.assertIsNotNone(discarded.manifest_problem)
                self.assertIsNone(self.state.pending_cleanup_run_id)
                self.assertTrue(self.gone.is_file())

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


class CleanupPreviewTests(MirrorTestCase):
    def setUp(self):
        super().setUp()
        self.write_photo("stranger.jpg", b"unrecorded")
        self.run_id = self.reconcile().run_id
        self.paths.lock_file.unlink()
        self.config_path = self.root / "config.toml"
        self.config_path.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\n'
            f'archive = "{self.archive_root}"\nlibrary = "{self.config.library}"\n'
        )

    def snapshot(self):
        return {
            path: (
                path.stat().st_mtime_ns,
                path.read_bytes() if path.is_file() else None,
            )
            for path in self.archive_root.rglob("*")
        }

    def preview(self, *args, probes=None, run_id=None):
        with (
            mock.patch(
                "photos_backup.archive.SystemProbes", return_value=self.system_probes
            ),
            mock.patch(
                "photos_backup.apple_photos.cleanup.PhotosProbes",
                return_value=probes or self.default_probes(),
            ),
        ):
            return CliRunner().invoke(
                cli,
                [
                    "--config",
                    str(self.config_path),
                    "approve-cleanup",
                    run_id or self.run_id,
                    "--dry-run",
                    *args,
                ],
            )

    def test_preview_revalidates_and_lists_deletions_without_any_writes(self):
        before = self.snapshot()
        result = self.preview()
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn(f"Would delete: {self.gone}", result.output)
        self.assertIn(f"({self.gone.stat().st_size} bytes)", result.output)
        self.assertIn("cleanup remains pending", result.output)
        self.assertNotIn("Approved cleanup", result.output)
        self.assertEqual(self.snapshot(), before)
        self.assertFalse(self.paths.lock_file.exists())

        # Approval revalidates again; a preview never authorizes a later edit.
        self.gone.write_bytes(b"edited after preview")
        approval = self.approve(self.run_id)
        self.assertEqual(approval.deleted, ())
        self.assertEqual(approval.stale, (self.gone,))

    def test_preview_explains_changed_and_unreviewed_files_that_will_be_kept(self):
        self.gone.write_bytes(b"changed since manifest")
        probes = self.probes(
            library=(), recorded=(asset("kept"), asset("gone")), files=self.records
        )
        before = self.snapshot()
        result = self.preview(probes=probes)
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Would delete: 0 archive file(s)", result.output)
        self.assertIn("(0 bytes)", result.output)
        self.assertIn("No longer deletable, so kept: 1", result.output)
        self.assertIn("Never reviewed, so kept: 1", result.output)
        self.assertIn(f"Kept: {self.gone}", result.output)
        self.assertIn(f"Kept: {self.kept}", result.output)
        self.assertEqual(self.snapshot(), before)

    def test_preview_keeps_a_photo_restored_to_the_library(self):
        probes = self.probes(
            library=(asset("kept"), asset("gone")),
            recorded=(asset("kept"), asset("gone")),
            files=self.records,
        )
        result = self.preview(probes=probes)
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn(f"Kept: {self.gone}", result.output)
        self.assertTrue(self.gone.exists())

    def test_preview_refusals_write_nothing(self):
        for refusal in ("wrong-run", "wrong-owner", "malformed", "missing", "discard"):
            with self.subTest(refusal=refusal):
                manifest = self.paths.cleanup_manifest(self.run_id)
                original = manifest.read_bytes()
                if refusal == "wrong-owner":
                    self.save_state(
                        writer_hostname="other.local",
                        pending_cleanup_run_id=self.run_id,
                    )
                elif refusal == "malformed":
                    manifest.write_text("{}")
                elif refusal == "missing":
                    manifest.unlink()
                before = self.snapshot()
                result = self.preview(
                    *(["--discard"] if refusal == "discard" else []),
                    run_id="another-run" if refusal == "wrong-run" else self.run_id,
                )
                self.assertEqual(
                    result.exit_code,
                    2 if refusal in ("discard", "malformed") else 3,
                    result.output,
                )
                self.assertEqual(self.snapshot(), before)
                manifest.write_bytes(original)
                self.save_state(pending_cleanup_run_id=self.run_id)

    def test_read_only_archive_cannot_be_used_for_actual_approval(self):
        before = self.snapshot()
        with self.archive(dry_run=True) as archive:
            with self.assertRaises(ArchiveUnsafe):
                approve_cleanup(self.config, archive, self.run_id)
        self.assertEqual(self.snapshot(), before)


if __name__ == "__main__":
    unittest.main()
