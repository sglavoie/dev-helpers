from __future__ import annotations

import datetime
import tempfile
import tomllib
import unittest
from importlib import metadata
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.archive import ArchivePaths, ArchiveState, ArchiveStateStore
from photos_backup.cli.cli import cli
from photos_backup.cli.ssd import load_optional_sd_card_config
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

    def test_root_help_documents_volume_option(self) -> None:
        result = self.runner.invoke(cli, ["--help"])

        self.assertIn("--volume", result.output)
        self.assertIn("apple_photos", result.output)

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


class VolumeOverrideCommandTests(ArchiveCommandTestCase):
    """`--volume` re-points a run at a plain directory, without a mount point."""

    def run_verify(self, *arguments: str):
        return self.runner.invoke(
            cli, ["--config", str(self.config_path), *arguments, "verify"]
        )

    def test_an_unmounted_configured_volume_still_fails_closed(self) -> None:
        result = self.run_verify()

        self.assertEqual(result.exit_code, 2)
        self.assertIn("is not a mount point", result.output)

    def test_override_accepts_a_local_directory(self) -> None:
        local = self.root / "some" / "path"
        (local / "Media" / "Apple Photos" / ".photos-backup").mkdir(parents=True)

        result = self.run_verify("--volume", str(local))

        # The archive opened, so verification reached its per-check report
        # instead of stopping at the volume.
        self.assertNotIn("[apple_photos] volume", result.output)
        self.assertIn("Archive verification", result.output)
        self.assertIn(str(local / "Media" / "Apple Photos"), result.output)

    def test_override_reports_a_missing_directory(self) -> None:
        result = self.run_verify("--volume", str(self.root / "absent"))

        self.assertEqual(result.exit_code, 2)
        self.assertIn("create it and retry", result.output)

    def test_override_rejects_a_relative_path(self) -> None:
        result = self.run_verify("--volume", "some/path")

        self.assertEqual(result.exit_code, 2)
        self.assertIn("absolute path", result.output)


if __name__ == "__main__":
    unittest.main()
