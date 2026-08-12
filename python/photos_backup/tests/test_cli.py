from __future__ import annotations

import contextlib
import datetime
import tempfile
import tomllib
import unittest
from importlib import metadata
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.apple_photos.adapter import PhotosProbes
from photos_backup.apple_photos.bootstrap import bootstrap_archive
from photos_backup.apple_photos.cleanup import (
    CleanupApproval,
    CleanupManifest,
    write_manifest,
)
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.plan import ExportMode, ExportPlan, ExportResult
from photos_backup.apple_photos.verify import Check, VerificationReport
from photos_backup.archive import ArchivePaths, ArchiveState, ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.cli.ssd import load_optional_sd_card_config
from photos_backup.errors import ACTION_REQUIRED_EXIT_CODE, ActionRequired
from photos_backup.config import (
    load_apple_photos_config,
    load_rclone_config,
    load_sd_card_config,
    load_ssd_config,
    resolve_rclone_source,
)
from photos_backup.exclude import exclude_from_arg
from photos_backup.remote.backup import Backup as RemoteBackup
from photos_backup.sd_card.backup import Backup as SdCardBackup
from photos_backup.ssd.backup import Backup as SsdBackup
from tests.test_bootstrap import asset
from tests.test_export import FakeRunner, row

ASSET = asset("a")
UNCHANGED_WRITER = mock.Mock(status=WriterStatus.UNCHANGED)

COMMANDS = (
    "apple-photos",
    "approve-cleanup",
    "backup-all",
    "bootstrap",
    "cleanup-local-export",
    "daily",
    "remote",
    "sd-card",
    "ssd",
    "verify",
)

CONFIG = """
[apple_photos]
volume = "/Volumes/Test"
archive = "/Volumes/Test/Media/Apple Photos"
library = "/Users/tester/Pictures/Photos Library.photoslibrary"
legacy_export = "/Users/tester/Pictures/export"
limit_export = 25
spouse_device_models = ["iPhone SE (2nd generation)"]

[sd_card]
source = "/Volumes/SDSONY/DCIM/100MSDCF"
destination = "/Volumes/Test/SDSONY"
exclude_file = "/Users/tester/.osxphotos.sd.exclude"

[ssd]
source = "/Users/tester/Pictures/export"
destination = "/Volumes/Data/Pictures"

[rclone]
remote = "b2:bucket"
"""

APPLE_PHOTOS_ONLY = """
[apple_photos]
volume = "/Volumes/Test"
archive = "/Volumes/Test/Media/Apple Photos"
library = "/Users/tester/Pictures/Photos Library.photoslibrary"
"""


class HelpTests(unittest.TestCase):
    def setUp(self) -> None:
        self.runner = CliRunner()

    def test_root_help_lists_every_command(self) -> None:
        result = self.runner.invoke(cli, ["--help"])

        self.assertEqual(result.exit_code, 0)
        for command in COMMANDS:
            self.assertIn(command, result.output)

    def test_root_help_documents_config_option(self) -> None:
        result = self.runner.invoke(cli, ["--help"])

        self.assertIn("--config", result.output)
        self.assertIn("photos-backup.toml", result.output)

    def test_each_command_help_succeeds(self) -> None:
        for command in COMMANDS:
            with self.subTest(command=command):
                result = self.runner.invoke(cli, [command, "--help"])
                self.assertEqual(result.exit_code, 0)

    def test_entry_point_aliases_expose_the_same_group(self) -> None:
        pyproject = Path(__file__).parents[1] / "pyproject.toml"
        scripts = tomllib.loads(pyproject.read_text())["project"]["scripts"]

        self.assertEqual(scripts["photos-backup"], scripts["cli"])
        self.assertEqual(scripts["photos-backup"], "photos_backup.cli.cli:cli")

    def test_installed_entry_points_resolve_to_the_same_group(self) -> None:
        entry_points = {
            entry.name: entry
            for entry in metadata.distribution("photos_backup").entry_points
            if entry.group == "console_scripts"
        }
        self.assertEqual(set(entry_points), {"photos-backup", "cli"})
        self.assertIs(entry_points["photos-backup"].load(), entry_points["cli"].load())


class ConstructionTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        self.tmp_path = Path(self._tmpdir.name)

    def write_config(self, content: str) -> Path:
        config_path = self.tmp_path / "photos-backup.toml"
        config_path.write_text(content)
        return config_path

    def test_sd_card_backup_uses_the_sd_card_section(self) -> None:
        config = load_sd_card_config(self.write_config(CONFIG))
        backup = SdCardBackup(config=config, dry_run=True)

        self.assertEqual(backup.src_path, Path("/Volumes/SDSONY/DCIM/100MSDCF"))
        self.assertEqual(backup.dst_path, Path("/Volumes/Test/SDSONY"))

    def test_ssd_backup_uses_the_ssd_and_sd_card_sections(self) -> None:
        config_path = self.write_config(CONFIG)
        backup = SsdBackup(
            config=load_ssd_config(config_path),
            delete_at_destination=False,
            dry_run=True,
            sd_card=load_optional_sd_card_config(config_path),
        )

        self.assertEqual(backup.source, Path("/Users/tester/Pictures/export"))
        self.assertEqual(backup.destination, Path("/Volumes/Data/Pictures"))
        self.assertIsNotNone(backup.sd_card)

    def test_ssd_backup_without_sd_card_section_skips_that_step(self) -> None:
        config_path = self.write_config(
            '[ssd]\nsource = "/Users/tester/Pictures/export"\n'
            'destination = "/Volumes/Data/Pictures"\n'
        )
        backup = SsdBackup(
            config=load_ssd_config(config_path),
            delete_at_destination=False,
            dry_run=True,
            sd_card=load_optional_sd_card_config(config_path),
        )

        self.assertIsNone(backup.sd_card)

    def test_missing_exclude_file_is_dropped_from_the_rsync_command(self) -> None:
        config = load_sd_card_config(self.write_config(CONFIG))
        backup = SdCardBackup(config=config, dry_run=True)

        self.assertEqual(exclude_from_arg(backup.exclude_file), "")

    def test_existing_exclude_file_is_passed_to_rsync(self) -> None:
        exclude_file = self.tmp_path / "exclude.txt"
        exclude_file.write_text("*.tmp\n")

        self.assertEqual(
            exclude_from_arg(exclude_file), f"--exclude-from={exclude_file}"
        )

    def test_rclone_source_falls_back_to_the_ssd_destination(self) -> None:
        config_path = self.write_config(CONFIG)
        config = load_rclone_config(config_path)

        self.assertEqual(
            resolve_rclone_source(config, config_path), Path("/Volumes/Data/Pictures")
        )

    def test_remote_backup_uses_the_resolved_source(self) -> None:
        config_path = self.write_config(CONFIG)
        config = load_rclone_config(config_path)
        backup = RemoteBackup(
            config=config,
            source=resolve_rclone_source(config, config_path),
            dry_run=True,
        )

        self.assertEqual(backup.remote, "b2:bucket")
        self.assertEqual(backup.src_path, Path("/Volumes/Data/Pictures"))

    def test_apple_photos_only_configuration_does_not_need_other_sections(self) -> None:
        config_path = self.write_config(APPLE_PHOTOS_ONLY)

        self.assertIsNone(load_apple_photos_config(config_path).legacy_export)
        self.assertIsNone(load_optional_sd_card_config(config_path))

    def test_apple_photos_export_does_not_need_a_legacy_export(self) -> None:
        config = load_apple_photos_config(self.write_config(APPLE_PHOTOS_ONLY))

        self.assertIsNone(config.legacy_export)


class ArchiveCommandTestCase(unittest.TestCase):
    """Gives every test a fake mounted volume and a matching configuration file."""

    def setUp(self) -> None:
        self.runner = CliRunner()
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        # Resolved because macOS temporary directories live under symlinks.
        self.root = Path(self._tmpdir.name).resolve()
        self.volume = self.root / "SanDisk"
        self.volume.mkdir()
        self.config_path = self.root / "photos-backup.toml"
        self.config_path.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\n'
            f'archive = "{self.volume}/Media/Apple Photos"\n'
            'library = "/Users/tester/Pictures/Photos Library.photoslibrary"\n'
        )

    def mounted(self):
        return mock.patch(
            "photos_backup.archive.probes.os.path.ismount",
            lambda path: Path(path) == self.volume,
        )

    def initialized_archive(self, **changes) -> ArchivePaths:
        paths = ArchivePaths(
            volume=self.volume, archive=self.volume / "Media" / "Apple Photos"
        )
        paths.metadata.mkdir(parents=True)
        ArchiveStateStore(paths).save(
            ArchiveState(
                initialized_at=datetime.datetime(
                    2026, 8, 11, 9, 30, tzinfo=datetime.UTC
                ),
                **changes,
            )
        )
        return paths


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


class DailyTests(ArchiveCommandTestCase):
    def test_daily_refuses_an_uninitialized_archive(self) -> None:
        with mock.patch(
            "photos_backup.archive.probes.os.path.ismount",
            lambda path: Path(path) == self.volume,
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily", "--dry-run"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("not initialized", result.output)
        self.assertIn("bootstrap", result.output)

    def test_daily_stops_before_exporting_when_a_takeover_is_blocked(self) -> None:
        paths = ArchivePaths(
            volume=self.volume, archive=self.volume / "Media" / "Apple Photos"
        )
        paths.metadata.mkdir(parents=True)
        ArchiveStateStore(paths).save(
            ArchiveState(
                initialized_at=datetime.datetime(
                    2026, 8, 11, 9, 30, tzinfo=datetime.UTC
                ),
                writer_hostname="another Mac.local",
            )
        )

        with (
            mock.patch(
                "photos_backup.archive.probes.os.path.ismount",
                lambda path: Path(path) == self.volume,
            ),
            mock.patch(
                "photos_backup.cli.daily.ensure_writer",
                side_effect=ActionRequired("that is a different library"),
            ),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("different library", result.output)
        self.assertEqual(list(paths.reports.glob("*.csv")), [])

    def test_daily_leaves_an_unmounted_volume_untouched(self) -> None:
        result = self.runner.invoke(
            cli, ["--config", str(self.config_path), "daily", "--dry-run"]
        )

        self.assertEqual(list(self.volume.iterdir()), [])
        self.assertNotEqual(result.exit_code, 0)

    def test_daily_asks_for_approval_while_a_cleanup_is_pending(self) -> None:
        self.initialized_archive(pending_cleanup_run_id="run-7")
        export = ExportResult(
            plan=ExportPlan(ExportMode.FULL, "a weekly full export"),
            exit_code=0,
            counts={"new": 1},
            state_advanced=True,
        )

        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.daily.ensure_writer",
                return_value=UNCHANGED_WRITER,
            ),
            mock.patch(
                "photos_backup.cli.daily.ApplePhotosExport",
                return_value=mock.Mock(export=lambda: export),
            ),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "daily"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE, result.output)
        self.assertIn("Mirror (pending)", result.output)
        self.assertIn("approve-cleanup run-7", result.output)


class ManualExportTests(ArchiveCommandTestCase):
    def run_manual(self, result: ExportResult, *args: str):
        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.apple_photos.ApplePhotosExport",
                return_value=mock.Mock(export=lambda: result),
            ),
        ):
            return self.runner.invoke(
                cli, ["--config", str(self.config_path), "apple-photos", *args]
            )

    def test_a_clean_manual_export_succeeds(self) -> None:
        result = self.run_manual(
            ExportResult(
                plan=ExportPlan(ExportMode.FULL, "a weekly full export"),
                exit_code=0,
                counts={"new": 1},
                state_advanced=True,
            )
        )

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Exported (full)", result.output)

    def test_a_failed_manual_export_reports_the_reason_and_fails(self) -> None:
        result = self.run_manual(
            ExportResult(
                plan=ExportPlan(
                    ExportMode.INCREMENTAL, "photos created since 2026-08-01"
                ),
                exit_code=1,
            )
        )

        self.assertEqual(result.exit_code, 1)
        self.assertIn("osxphotos exited with status 1", result.output)

    def test_a_manual_export_with_error_rows_fails(self) -> None:
        result = self.run_manual(
            ExportResult(
                plan=ExportPlan(ExportMode.FULL, "a weekly full export"),
                exit_code=0,
                counts={"new": 1, "error": 2},
            )
        )

        self.assertEqual(result.exit_code, 1)
        self.assertIn("2 export error(s)", result.output)


class ApproveCleanupTests(ArchiveCommandTestCase):
    def test_approving_with_nothing_pending_asks_for_a_daily_run(self) -> None:
        self.initialized_archive()

        with self.mounted():
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "approve-cleanup", "run-7"]
            )

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("No cleanup is waiting", result.output)

    def test_an_approval_reports_what_it_deleted_and_kept(self) -> None:
        self.initialized_archive(pending_cleanup_run_id="run-7")
        approval = CleanupApproval(
            run_id="run-7",
            manifest=None,
            deleted=(Path("/a/gone.jpg"),),
            stale=(Path("/a/edited.jpg"),),
            unreviewed=(),
            completed_at=datetime.datetime(2026, 8, 13, 9, 30, tzinfo=datetime.UTC),
        )

        with (
            self.mounted(),
            mock.patch(
                "photos_backup.cli.approve_cleanup.run_cleanup_approval",
                return_value=approval,
            ),
        ):
            result = self.runner.invoke(
                cli, ["--config", str(self.config_path), "approve-cleanup", "run-7"]
            )

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Approved cleanup run 'run-7'", result.output)
        self.assertIn("Deleted: 1 archive file(s)", result.output)
        self.assertIn("No longer deletable, so kept: 1", result.output)
        self.assertIn("Mirror completed at", result.output)

    def test_discarding_clears_the_pending_run_without_deleting(self) -> None:
        paths = self.initialized_archive(pending_cleanup_run_id="run-7")
        write_manifest(
            paths.cleanup_manifest("run-7"),
            CleanupManifest(
                run_id="run-7",
                created_at=datetime.datetime(2026, 8, 13, 9, 30, tzinfo=datetime.UTC),
                hostname="tester",
                archive=paths.archive,
                reason="one archive file has no export-database record",
                assets=(),
                candidates=(),
                changed=(),
                unknown=(Path("/a/stranger.jpg"),),
                ambiguous=(),
            ),
        )

        with self.mounted():
            result = self.runner.invoke(
                cli,
                [
                    "--config",
                    str(self.config_path),
                    "approve-cleanup",
                    "run-7",
                    "--discard",
                ],
            )

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("Discarded cleanup run 'run-7'", result.output)
        self.assertIn("nothing was deleted", result.output)
        self.assertIsNone(ArchiveStateStore(paths).load().pending_cleanup_run_id)


class LocalExportCleanupTests(ArchiveCommandTestCase):
    """The destructive branch is never reached here: no test has a terminal."""

    def setUp(self) -> None:
        super().setUp()
        self.export = self.root / "export"
        self.photo = self.export / "2022" / "03" / "a.JPG"
        self.photo.parent.mkdir(parents=True)
        self.photo.write_text("12345")
        self.config_path.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\n'
            f'archive = "{self.volume}/Media/Apple Photos"\n'
            'library = "/Users/tester/Pictures/Photos Library.photoslibrary"\n'
            f'legacy_export = "{self.export}"\n'
        )

    def run_cleanup(self, *args: str, healthy: bool = False):
        report = VerificationReport(checks=(Check("state", True, "healthy"),))
        with contextlib.ExitStack() as stack:
            stack.enter_context(self.mounted())
            if healthy:
                stack.enter_context(
                    mock.patch(
                        "photos_backup.cli.cleanup_local_export.verify_archive",
                        return_value=report,
                    )
                )
            return self.runner.invoke(
                cli,
                ["--config", str(self.config_path), "cleanup-local-export", *args],
            )

    def test_a_run_without_a_terminal_is_refused(self) -> None:
        result = self.run_cleanup()

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("never from a script", result.output)
        self.assertTrue(self.photo.is_file())

    def test_an_uninitialized_archive_is_refused(self) -> None:
        result = self.run_cleanup("--dry-run")

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("is not initialized", result.output)
        self.assertTrue(self.photo.is_file())

    def test_a_failing_verification_stops_the_cleanup(self) -> None:
        self.initialized_archive()

        result = self.run_cleanup("--dry-run")

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("Archive check(s) failed", result.output)
        self.assertTrue(self.photo.is_file())

    def test_a_dry_run_shows_the_path_count_and_size_it_would_delete(self) -> None:
        self.initialized_archive()

        result = self.run_cleanup("--dry-run", healthy=True)

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn(str(self.export), result.output)
        self.assertIn("Files: 1", result.output)
        self.assertIn("Size: 5 B", result.output)
        self.assertIn("nothing was deleted", result.output)
        self.assertTrue(self.photo.is_file())

    def test_unrecognized_contents_stop_the_cleanup(self) -> None:
        self.initialized_archive()
        (self.export / "taxes.pdf").write_text("not a photo")

        result = self.run_cleanup("--dry-run", healthy=True)

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("not something an Apple Photos export writes", result.output)
        self.assertTrue(self.photo.is_file())

    def test_an_export_configured_on_the_archive_volume_is_refused(self) -> None:
        self.initialized_archive()
        self.config_path.write_text(
            f'[apple_photos]\nvolume = "{self.volume}"\n'
            f'archive = "{self.volume}/Media/Apple Photos"\n'
            'library = "/Users/tester/Pictures/Photos Library.photoslibrary"\n'
            f'legacy_export = "{self.volume}"\n'
        )

        result = self.run_cleanup("--dry-run", healthy=True)

        self.assertEqual(result.exit_code, ACTION_REQUIRED_EXIT_CODE)
        self.assertIn("archive volume", result.output)
        self.assertTrue(self.photo.is_file())


if __name__ == "__main__":
    unittest.main()
