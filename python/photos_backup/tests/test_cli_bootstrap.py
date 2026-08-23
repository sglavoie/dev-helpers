from __future__ import annotations

import unittest
from unittest import mock

from photos_backup.apple_photos.adapter import PhotosProbes
from photos_backup.apple_photos.bootstrap import bootstrap_archive
from photos_backup.cli.cli import cli
from photos_backup.errors import ACTION_REQUIRED_EXIT_CODE
from tests.test_bootstrap import asset
from tests.test_cli import ArchiveCommandTestCase
from tests.test_export import FakeRunner, row

ASSET = asset("a")


class BootstrapTests(ArchiveCommandTestCase):
    def run_bootstrap(self, runner: FakeRunner, *args: str, library=(), recorded=None):
        """Run the real bootstrap against a fake osxphotos and a fake library."""
        written = library if recorded is None else recorded
        probes = PhotosProbes(
            read_library=lambda _: library,
            read_export_db=lambda _: written if runner.arguments is not None else (),
        )

        def bootstrap_with_fakes(config, archive):
            return bootstrap_archive(
                config,
                archive,
                probes=probes,
                runner=runner,
                metadata_reader=lambda path: {},
            )

        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.bootstrap.bootstrap_archive", bootstrap_with_fakes
            ),
        ):
            return self.runner.invoke(
                cli, ["--config", str(self.config_path), "bootstrap", *args]
            )

    def test_a_complete_bootstrap_initializes_and_succeeds(self) -> None:
        result = self.run_bootstrap(FakeRunner([row("a.jpg", new=1)]), library=(ASSET,))

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Bootstrapping (fresh archive)", result.output)
        self.assertIn("1 of 1 library asset(s) recorded", result.output)
        self.assertIn("Archive initialized at", result.output)

    def test_an_incomplete_bootstrap_explains_why_it_did_not_initialize(self) -> None:
        result = self.run_bootstrap(
            FakeRunner([row("a.jpg", new=1)]), library=(ASSET,), recorded=()
        )

        self.assertEqual(result.exit_code, 1)
        self.assertIn("Archive not initialized", result.output)
        self.assertIn("no export-database record", result.output)
        self.assertIn("again to resume", result.output)

    def test_a_dry_run_explains_the_work_and_succeeds(self) -> None:
        runner = FakeRunner([row("a.jpg", new=1)])
        result = self.run_bootstrap(
            runner,
            "--dry-run",
            library=(ASSET,),
            recorded=(),
        )

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIsNone(runner.arguments)
        self.assertIn("Would bootstrap", result.output)
        self.assertIn("Would export", result.output)
        self.assertIn("Status: PLANNED", result.output)
        self.assertIn("Coverage: deferred until the real export", result.output)
        self.assertIn("was not initialized", result.output)
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_bootstrap_refuses_an_already_initialized_archive(self) -> None:
        self.initialized_archive()

        with self.mounted():
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "bootstrap"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("already initialized", result.output)
        self.assertIn("daily", result.output)

    def test_bootstrap_leaves_an_unmounted_volume_untouched(self) -> None:
        result = self.runner.invoke(
            cli, ["--config", str(self.config_path), "bootstrap", "--dry-run"]
        )

        self.assertEqual(list(self.volume.iterdir()), [])
        self.assertNotEqual(result.exit_code, 0)


class VerifyTests(ArchiveCommandTestCase):
    def test_verify_reports_every_failure_and_creates_nothing(self) -> None:
        with self.mounted():
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "verify"]
            )

        self.assertEqual(result.exit_code, 1)
        self.assertIn("[FAIL] export database", result.output)
        self.assertIn("[FAIL] state", result.output)
        self.assertIn("check(s) failed", result.output)
        self.assertEqual(list(self.volume.iterdir()), [])

    def test_verify_names_a_pending_cleanup_without_approving_it(self) -> None:
        paths = self.initialized_archive(pending_cleanup_run_id="run-7")

        with self.mounted():
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "verify"]
            )

        self.assertIn("[FAIL] pending cleanup", result.output)
        self.assertIn("run-7", result.output)
        self.assertEqual(list(paths.cleanup.glob("*.json")), [])


if __name__ == "__main__":
    unittest.main()
