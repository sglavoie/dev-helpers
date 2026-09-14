from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest import mock

import click

from photos_backup.config import (
    DEFAULT_CONFIG_PATH,
    load_apple_photos_config,
    load_rclone_config,
    load_sd_card_config,
    load_ssd_config,
    normalize_volume_override,
    resolve_config_path,
)

FULL_CONFIG = """
[apple_photos]
volume = "/Volumes/Test"
archive = "/Volumes/Test/Media/Apple Photos"
library = "~/Pictures/Photos Library.photoslibrary"
legacy_export = "~/Pictures/export"
limit_export = 100
spouse_device_models = [" iPhone SE (2nd generation) ", "iPhone 17"]
incremental_overlap_days = 14
full_export_weekday = "monday"
full_export_max_age_days = 7
mirror = true
cleanup_max_assets = 10
cleanup_max_fraction = 0.001

[sd_card]
source = "/Volumes/SDSONY/DCIM/100MSDCF"
destination = "~/Pictures/SDSONY"
exclude_file = "~/.osxphotos.sd.exclude"

[ssd]
source = "~/Pictures/export"
destination = "/Volumes/Data/Pictures"

[rclone]
remote = "b2:bucket"
source = "/Volumes/Data/Pictures"
"""

MINIMAL_APPLE_PHOTOS = """
[apple_photos]
volume = "/Volumes/Test"
archive = "/Volumes/Test/Media/Apple Photos"
library = "/Users/tester/Pictures/Photos Library.photoslibrary"
"""


class ConfigTestCase(unittest.TestCase):
    """Base class writing configuration files outside the real home directory."""

    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        self.tmp_path = Path(self._tmpdir.name)

    def write_config(self, content: str) -> Path:
        config_path = self.tmp_path / "photos-backup.toml"
        config_path.write_text(content)
        return config_path

    def assert_usage_error(self, content: str, *expected_fragments: str) -> None:
        config_path = self.write_config(content)
        with self.assertRaises(click.UsageError) as raised:
            load_apple_photos_config(config_path)
        for fragment in expected_fragments:
            self.assertIn(fragment, str(raised.exception))


class ConfigPathTests(ConfigTestCase):
    def test_default_path_is_used_when_none_given(self) -> None:
        self.assertEqual(resolve_config_path(), DEFAULT_CONFIG_PATH.expanduser())

    def test_explicit_path_is_expanded(self) -> None:
        self.assertEqual(
            resolve_config_path(Path("~/somewhere/photos-backup.toml")),
            Path.home() / "somewhere/photos-backup.toml",
        )

    def test_missing_file_is_actionable(self) -> None:
        missing = self.tmp_path / "absent.toml"
        with self.assertRaises(click.UsageError) as raised:
            load_apple_photos_config(missing)
        self.assertIn(str(missing), str(raised.exception))
        self.assertIn("--config", str(raised.exception))

    def test_invalid_toml_is_reported(self) -> None:
        self.assert_usage_error("[apple_photos", "not valid TOML")


class ApplePhotosConfigTests(ConfigTestCase):
    def test_full_section_is_parsed_and_expanded(self) -> None:
        config = load_apple_photos_config(self.write_config(FULL_CONFIG))

        self.assertEqual(config.volume, Path("/Volumes/Test"))
        self.assertEqual(config.archive, Path("/Volumes/Test/Media/Apple Photos"))
        self.assertEqual(
            config.library, Path.home() / "Pictures/Photos Library.photoslibrary"
        )
        self.assertEqual(config.legacy_export, Path.home() / "Pictures/export")
        self.assertEqual(config.limit_export, 100)
        self.assertEqual(
            config.spouse_device_models,
            ("iPhone SE (2nd generation)", "iPhone 17"),
        )
        self.assertEqual(config.incremental_overlap_days, 14)
        self.assertEqual(config.full_export_weekday, 0)
        self.assertEqual(config.full_export_max_age_days, 7)
        self.assertTrue(config.mirror)
        self.assertEqual(config.cleanup_max_assets, 10)
        self.assertEqual(config.cleanup_max_fraction, 0.001)

    def test_optional_values_fall_back_to_defaults(self) -> None:
        config = load_apple_photos_config(self.write_config(MINIMAL_APPLE_PHOTOS))

        self.assertIsNone(config.legacy_export)
        self.assertEqual(config.limit_export, 0)
        self.assertEqual(config.spouse_device_models, ())
        self.assertEqual(config.incremental_overlap_days, 14)
        self.assertEqual(config.full_export_weekday, 0)
        self.assertEqual(config.full_export_max_age_days, 7)
        self.assertTrue(config.mirror)
        self.assertEqual(config.cleanup_max_assets, 10)
        self.assertEqual(config.cleanup_max_fraction, 0.001)

    def test_environment_variables_are_expanded(self) -> None:
        content = """
        [apple_photos]
        volume = "$PHOTOS_TEST_VOLUME"
        archive = "$PHOTOS_TEST_VOLUME/Media/Apple Photos"
        library = "$PHOTOS_TEST_VOLUME/Photos Library.photoslibrary"
        """
        with mock.patch.dict("os.environ", {"PHOTOS_TEST_VOLUME": "/Volumes/FromEnv"}):
            config = load_apple_photos_config(self.write_config(content))

        self.assertEqual(config.archive, Path("/Volumes/FromEnv/Media/Apple Photos"))

    def test_missing_section_is_reported(self) -> None:
        self.assert_usage_error("[sd_card]\n", "missing the required [apple_photos]")

    def test_missing_required_value_is_reported(self) -> None:
        self.assert_usage_error(
            '[apple_photos]\nvolume = "/Volumes/Test"\n',
            "archive",
            "required",
        )

    def test_unknown_key_is_rejected(self) -> None:
        self.assert_usage_error(
            MINIMAL_APPLE_PHOTOS + 'archive_path = "/Volumes/Test/Other"\n',
            "unknown key(s) archive_path",
        )

    def test_relative_path_is_rejected(self) -> None:
        self.assert_usage_error(
            '[apple_photos]\nvolume = "Volumes/Test"\n',
            "volume",
            "absolute path",
        )

    def test_archive_must_be_inside_volume(self) -> None:
        content = """
        [apple_photos]
        volume = "/Volumes/Test"
        archive = "/Volumes/Other/Media/Apple Photos"
        library = "/Users/tester/Pictures/Photos Library.photoslibrary"
        """
        self.assert_usage_error(content, "archive", "must be inside volume")

    def test_archive_equal_to_volume_is_rejected(self) -> None:
        content = """
        [apple_photos]
        volume = "/Volumes/Test"
        archive = "/Volumes/Test"
        library = "/Users/tester/Pictures/Photos Library.photoslibrary"
        """
        self.assert_usage_error(content, "archive", "must be inside volume")

    def test_legacy_export_inside_archive_is_rejected(self) -> None:
        content = MINIMAL_APPLE_PHOTOS + (
            'legacy_export = "/Volumes/Test/Media/Apple Photos/export"\n'
        )
        self.assert_usage_error(content, "legacy_export", "must not be inside archive")

    def test_volume_override_reroots_the_archive_sub_path(self) -> None:
        config = load_apple_photos_config(
            self.write_config(MINIMAL_APPLE_PHOTOS),
            volume=Path("/Users/tester/some/path"),
        )

        self.assertEqual(config.volume, Path("/Users/tester/some/path"))
        self.assertEqual(
            config.archive, Path("/Users/tester/some/path/Media/Apple Photos")
        )
        self.assertFalse(config.require_mounted_volume)

    def test_without_an_override_the_volume_must_be_mounted(self) -> None:
        config = load_apple_photos_config(self.write_config(MINIMAL_APPLE_PHOTOS))

        self.assertEqual(config.volume, Path("/Volumes/Test"))
        self.assertTrue(config.require_mounted_volume)

    def test_volume_override_still_validates_the_configuration_file(self) -> None:
        content = """
        [apple_photos]
        volume = "/Volumes/Test"
        archive = "/Volumes/Other/Media/Apple Photos"
        library = "/Users/tester/Pictures/Photos Library.photoslibrary"
        """
        config_path = self.write_config(content)
        with self.assertRaises(click.UsageError) as raised:
            load_apple_photos_config(
                config_path, volume=Path("/Users/tester/elsewhere")
            )
        self.assertIn("must be inside volume", str(raised.exception))

    def test_legacy_export_inside_the_overridden_archive_is_rejected(self) -> None:
        content = MINIMAL_APPLE_PHOTOS + (
            'legacy_export = "/Users/tester/elsewhere/Media/Apple Photos/export"\n'
        )
        config_path = self.write_config(content)
        with self.assertRaises(click.UsageError) as raised:
            load_apple_photos_config(
                config_path, volume=Path("/Users/tester/elsewhere")
            )
        self.assertIn("must not be inside archive", str(raised.exception))

    def test_overlap_days_out_of_range_is_rejected(self) -> None:
        self.assert_usage_error(
            MINIMAL_APPLE_PHOTOS + "incremental_overlap_days = 400\n",
            "incremental_overlap_days",
            "between 0 and 365",
        )

    def test_wrong_type_is_rejected(self) -> None:
        self.assert_usage_error(
            MINIMAL_APPLE_PHOTOS + 'incremental_overlap_days = "14"\n',
            "incremental_overlap_days",
            "must be an integer",
        )

    def test_boolean_is_not_accepted_as_integer(self) -> None:
        self.assert_usage_error(
            MINIMAL_APPLE_PHOTOS + "limit_export = true\n",
            "limit_export",
            "must be an integer",
        )

    def test_unknown_weekday_is_rejected(self) -> None:
        self.assert_usage_error(
            MINIMAL_APPLE_PHOTOS + 'full_export_weekday = "caturday"\n',
            "full_export_weekday",
            "monday",
        )

    def test_weekday_is_case_insensitive(self) -> None:
        config = load_apple_photos_config(
            self.write_config(MINIMAL_APPLE_PHOTOS + 'full_export_weekday = "Sunday"\n')
        )
        self.assertEqual(config.full_export_weekday, 6)

    def test_cleanup_fraction_above_one_is_rejected(self) -> None:
        self.assert_usage_error(
            MINIMAL_APPLE_PHOTOS + "cleanup_max_fraction = 1.5\n",
            "cleanup_max_fraction",
            "between 0.0 and 1.0",
        )

    def test_negative_cleanup_limit_is_rejected(self) -> None:
        self.assert_usage_error(
            MINIMAL_APPLE_PHOTOS + "cleanup_max_assets = -1\n",
            "cleanup_max_assets",
            "must be between",
        )

    def test_non_string_device_model_is_rejected(self) -> None:
        self.assert_usage_error(
            MINIMAL_APPLE_PHOTOS + "spouse_device_models = [1]\n",
            "spouse_device_models",
            "non-empty strings",
        )

    def test_mirror_must_be_boolean(self) -> None:
        self.assert_usage_error(
            MINIMAL_APPLE_PHOTOS + 'mirror = "yes"\n',
            "mirror",
            "true or false",
        )


class VolumeOverrideTests(unittest.TestCase):
    def test_absolute_path_is_kept(self) -> None:
        self.assertEqual(
            normalize_volume_override("/Users/tester/some/path"),
            Path("/Users/tester/some/path"),
        )

    def test_home_and_environment_variables_are_expanded(self) -> None:
        with mock.patch.dict("os.environ", {"PHOTOS_TEST_VOLUME": "some"}):
            self.assertEqual(
                normalize_volume_override("~/$PHOTOS_TEST_VOLUME/path"),
                Path.home() / "some/path",
            )

    def test_relative_path_is_rejected(self) -> None:
        with self.assertRaises(click.BadParameter) as raised:
            normalize_volume_override("some/path")
        self.assertIn("absolute path", str(raised.exception))


class OtherSectionTests(ConfigTestCase):
    def test_sd_card_section(self) -> None:
        config = load_sd_card_config(self.write_config(FULL_CONFIG))

        self.assertEqual(config.source, Path("/Volumes/SDSONY/DCIM/100MSDCF"))
        self.assertEqual(config.destination, Path.home() / "Pictures/SDSONY")
        self.assertEqual(config.exclude_file, Path.home() / ".osxphotos.sd.exclude")

    def test_ssd_section_without_exclude_file(self) -> None:
        config = load_ssd_config(self.write_config(FULL_CONFIG))

        self.assertEqual(config.source, Path.home() / "Pictures/export")
        self.assertEqual(config.destination, Path("/Volumes/Data/Pictures"))
        self.assertIsNone(config.exclude_file)

    def test_rclone_section(self) -> None:
        config = load_rclone_config(self.write_config(FULL_CONFIG))

        self.assertEqual(config.remote, "b2:bucket")
        self.assertEqual(config.source, Path("/Volumes/Data/Pictures"))

    def test_rclone_requires_remote(self) -> None:
        config_path = self.write_config('[rclone]\nsource = "/Volumes/Data"\n')
        with self.assertRaises(click.UsageError) as raised:
            load_rclone_config(config_path)
        self.assertIn("remote", str(raised.exception))

    def test_shipped_example_file_is_valid(self) -> None:
        example = Path(__file__).parents[1] / "photos-backup.example.toml"

        load_apple_photos_config(example)
        load_sd_card_config(example)
        load_ssd_config(example)
        load_rclone_config(example)

    def test_apple_photos_section_is_not_required_by_other_loaders(self) -> None:
        config_path = self.write_config(
            '[sd_card]\nsource = "/Volumes/SD"\ndestination = "/Volumes/Data/SD"\n'
        )
        config = load_sd_card_config(config_path)
        self.assertEqual(config.destination, Path("/Volumes/Data/SD"))


if __name__ == "__main__":
    unittest.main()
