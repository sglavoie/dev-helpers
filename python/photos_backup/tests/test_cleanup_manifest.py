from __future__ import annotations

import datetime
import tempfile
import unittest
from pathlib import Path

from photos_backup.apple_photos.cleanup import (
    CleanupManifest,
    new_run_id,
    read_manifest,
    write_manifest,
)
from photos_backup.apple_photos.reconcile import CandidateFile
from photos_backup.archive import ArchiveUnsafe, safe_name
from tests.test_bootstrap import asset
from tests.test_export import HOSTNAME, THURSDAY


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
