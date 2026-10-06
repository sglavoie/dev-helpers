"""Hidden exclusions affect export coverage, never deletion detection."""

import dataclasses
import inspect
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from osxphotos.cli.export import export_cli

from photos_backup.apple_photos.adapter import read_photos_library
from photos_backup.apple_photos.cleanup import reconcile_mirror
from tests.test_bootstrap import BootstrapTestCase, asset
from tests.test_cleanup import FULL_EXPORT, MirrorTestCase, UNTOUCHED
from tests.test_export import FakeRunner, row


class HiddenBootstrapTests(BootstrapTestCase):
    def test_excluded_hidden_assets_do_not_block_initialization(self):
        self.config = dataclasses.replace(self.config, exclude_hidden=True)
        runner = FakeRunner([row("visible.jpg", new=1)])
        result = self.bootstrap(
            runner,
            library=(
                asset("visible"),
                dataclasses.replace(asset("hidden"), hidden=True),
            ),
            after=(asset("visible"),),
        )
        self.assertTrue(result.initialized)
        self.assertEqual(result.coverage.library, 1)
        self.assertEqual(result.excluded_hidden, 1)
        self.assertTrue(runner.arguments["not_hidden"])
        inspect.signature(export_cli).bind(**runner.arguments)

    def test_hidden_assets_still_block_initialization_by_default(self):
        result = self.bootstrap(
            FakeRunner(),
            library=(dataclasses.replace(asset("hidden"), hidden=True),),
            after=(),
        )
        self.assertFalse(result.initialized)
        self.assertEqual(result.excluded_hidden, 0)

    def test_visible_missing_components_still_block_initialization(self):
        self.config = dataclasses.replace(self.config, exclude_hidden=True)
        result = self.bootstrap(
            FakeRunner([row("visible.mov", missing=1)]),
            library=(
                asset("visible"),
                dataclasses.replace(asset("hidden"), hidden=True),
            ),
            after=(asset("visible"),),
        )
        self.assertFalse(result.initialized)
        self.assertEqual(result.excluded_hidden, 1)


class HiddenLibraryTests(unittest.TestCase):
    def test_library_reader_keeps_hidden_assets_with_their_visibility(self):
        with patch("photos_backup.apple_photos.adapter.PhotosDB") as database:
            database.return_value.photos.return_value = [
                SimpleNamespace(
                    uuid="hidden",
                    cloud_guid="cloud",
                    original_filename="h.jpg",
                    hidden=True,
                ),
                SimpleNamespace(
                    uuid="visible",
                    cloud_guid="other",
                    original_filename="v.jpg",
                    hidden=False,
                ),
            ]
            assets = read_photos_library(Path("/unused.photoslibrary"))
        self.assertEqual(
            [(a.uuid, a.hidden) for a in assets], [("hidden", True), ("visible", False)]
        )


class HiddenMirrorTests(MirrorTestCase):
    def test_hiding_an_exported_asset_does_not_delete_its_archive_copy(self):
        self.config = dataclasses.replace(self.config, exclude_hidden=True)
        probes = self.probes(
            library=(dataclasses.replace(asset("kept"), hidden=True), *UNTOUCHED),
            recorded=(asset("kept"), asset("gone"), *UNTOUCHED),
            files=self.records,
        )
        with self.archive() as opened:
            reconcile_mirror(self.config, opened, FULL_EXPORT, probes=probes)
        self.assertTrue(self.kept.exists())
        self.assertFalse(self.gone.exists())
