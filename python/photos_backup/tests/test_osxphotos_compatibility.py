"""Offline contracts for the pinned upstream API; never open a Photos library."""

import inspect
import unittest
from importlib.metadata import requires, version
from pathlib import Path

from osxphotos.cli.export import export_cli, export_photo
from osxphotos.export_db_utils import export_db_migrate_photos_library
from osxphotos.photoexporter import PhotoExporter, StagedFiles

from photos_backup.apple_photos.plan import ExportMode, ExportPlan, export_arguments
from tests.test_export import make_config


class OsxphotosCompatibilityTests(unittest.TestCase):
    def test_installation_uses_the_tested_release(self):
        supported = "0.76.1"
        self.assertEqual(version("osxphotos"), supported)
        self.assertIn(f"osxphotos=={supported}", requires("photos_backup"))

    def test_generated_options_bind_to_upstream_export(self):
        root = Path("/unused-compatibility-fixture")
        arguments = export_arguments(
            make_config(root, root / "archive"),
            ExportPlan(ExportMode.FULL, "compatibility check"),
            dest=root / "archive",
            export_db=root / "export.db",
            report_path=root / "report.csv",
        )
        # bind catches removed/renamed options and new required arguments.
        inspect.signature(export_cli).bind(**arguments, no_progress=True)

    def test_private_hooks_keep_the_call_shapes_used_by_the_adapter(self):
        inspect.signature(PhotoExporter._stage_photo_for_export_with_photokit).bind(
            object(), object()
        )
        self.assertIn("photo", inspect.signature(export_photo).parameters)
        inspect.signature(export_db_migrate_photos_library).bind(
            dbfile="unused.db", photos_library="unused.photoslibrary", dry_run=True
        )

    def test_staging_wire_format_round_trips_all_media_components(self):
        staged = StagedFiles(
            original="original.heic",
            original_live="original.mov",
            edited="edited.heic",
            edited_live="edited.mov",
            raw="original.dng",
            preview="preview.jpeg",
            aae="edited.aae",
            original_aae="original.aae",
            error=["fixture error"],
            update_skipped=True,
        )
        self.assertEqual(StagedFiles(**staged.asdict()).asdict(), staged.asdict())
