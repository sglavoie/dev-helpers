from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

from osxphotos.exportoptions import ExportOptions
from osxphotos.photoexporter import PhotoExporter, StagedFiles

from photos_backup.apple_photos.adapter import run_osxphotos_export
from photos_backup.apple_photos.downloads import (
    DownloadBudget,
    _run_worker,
    _worker,
    bounded_downloads,
)
from tests.test_export import FakeRunner


def photo(**overrides):
    fields = dict(
        uuid="asset-1",
        original_filename="photo.jpg",
        hasadjustments=False,
        live_photo=False,
        shared=False,
        has_raw=False,
        uti_edited=None,
        uti="public.jpeg",
        burst=False,
        path_derivatives=[],
        _info={},
        _verbose=lambda _: None,
        path="/local/photo.jpg",
        path_edited=None,
        path_live_photo=None,
        path_edited_live_photo=None,
        path_raw=None,
    )
    fields.update(overrides)
    return SimpleNamespace(**fields)


class DownloadTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.exporter = PhotoExporter(photo())
        self.exporter._temp_dir_path = self.root

    @staticmethod
    def downloaded(request, response, timeout):
        image = request.parent / "photo.jpg"
        image.write_bytes(b"downloaded original")
        response.write_text(json.dumps(StagedFiles(original=str(image)).asdict()))

    def test_timeout_is_shared_across_versions_and_retries(self):
        budget = DownloadBudget(120)
        with mock.patch(
            "photos_backup.apple_photos.downloads._run_worker",
            side_effect=subprocess.TimeoutExpired("worker", 120),
        ) as run:
            for options in (
                ExportOptions(),
                ExportOptions(),
                ExportOptions(edited=True),
            ):
                result = budget.stage(self.exporter, options)
                self.assertIsNone(result.original)

        run.assert_called_once()
        self.assertIn("timed out", budget.failures["asset-1"]["reason"])
        self.assertEqual(list(self.root.iterdir()), [])

    def test_success_retains_staging_until_parent_finishes_and_no_total_budget(self):
        budget = DownloadBudget(120)
        with (
            mock.patch(
                "photos_backup.apple_photos.downloads._run_worker",
                side_effect=self.downloaded,
            ) as run,
            mock.patch(
                "photos_backup.apple_photos.downloads.time.monotonic",
                side_effect=[0, 100, 500, 600],
            ),
        ):
            first = budget.stage(self.exporter, ExportOptions())
            self.exporter.photo.uuid = "asset-2"
            second = budget.stage(self.exporter, ExportOptions())

        self.assertEqual(Path(first.original).read_bytes(), b"downloaded original")
        self.assertTrue(Path(second.original).exists())
        self.assertEqual([call.args[2] for call in run.call_args_list], [120, 120])
        self.assertFalse(budget.failures)

    def test_successful_attempt_time_is_deducted_before_another_version(self):
        budget = DownloadBudget(120)
        with (
            mock.patch(
                "photos_backup.apple_photos.downloads._run_worker",
                side_effect=self.downloaded,
            ) as run,
            mock.patch(
                "photos_backup.apple_photos.downloads.time.monotonic",
                side_effect=[0, 100, 500, 510],
            ),
        ):
            budget.stage(self.exporter, ExportOptions())
            budget.stage(self.exporter, ExportOptions(edited=True))

        self.assertEqual([call.args[2] for call in run.call_args_list], [120, 20])

    def test_worker_failure_is_reported_and_staging_removed(self):
        budget = DownloadBudget(120)
        with mock.patch(
            "photos_backup.apple_photos.downloads._run_worker",
            side_effect=subprocess.CalledProcessError(1, "worker"),
        ):
            budget.stage(self.exporter, ExportOptions())
        budget.write_failures(self.root / "export.csv")
        failures = json.loads((self.root / "export.downloads.json").read_text())
        self.assertEqual(failures[0]["uuid"], "asset-1")
        self.assertIn("download failed", failures[0]["reason"])
        self.assertEqual(
            [p.name for p in self.root.iterdir()], ["export.downloads.json"]
        )

    def test_fully_local_export_does_not_start_a_download_worker(self):
        with (
            bounded_downloads(1),
            mock.patch("photos_backup.apple_photos.downloads._run_worker") as run,
        ):
            staged = self.exporter._stage_photos_for_export(
                ExportOptions(
                    download_missing=True, use_photokit=True, export_aae=False
                )
            )
        self.assertEqual(staged.original, "/local/photo.jpg")
        run.assert_not_called()

    def test_adapter_is_restored_after_an_exception(self):
        original = PhotoExporter._stage_photo_for_export_with_photokit
        with self.assertRaises(RuntimeError), bounded_downloads(1):
            raise RuntimeError("export failed")
        self.assertIs(PhotoExporter._stage_photo_for_export_with_photokit, original)

    def test_dry_run_never_downloads(self):
        with (
            bounded_downloads(1),
            mock.patch("photos_backup.apple_photos.downloads._run_worker") as run,
        ):
            staged = self.exporter._stage_photo_for_export_with_photokit(
                ExportOptions(dry_run=True)
            )
        self.assertTrue(staged.original.endswith("photo.jpeg"))
        run.assert_not_called()

    def test_real_subprocess_timeout_kills_and_reaps_blocked_child(self):
        executable = self.root / "blocked_worker"
        pid_file = self.root / "pid"
        executable.write_text(
            f"#!{sys.executable}\nimport os, time\nfrom pathlib import Path\n"
            f"Path({str(pid_file)!r}).write_text(str(os.getpid()))\ntime.sleep(30)\n"
        )
        executable.chmod(0o700)
        with (
            mock.patch(
                "photos_backup.apple_photos.downloads.sys.executable", str(executable)
            ),
            self.assertRaises(subprocess.TimeoutExpired),
        ):
            _run_worker(self.root / "request", self.root / "response", 1)
        pid = int(pid_file.read_text())
        with self.assertRaises(ProcessLookupError):
            os.kill(pid, 0)

    def test_worker_uses_upstream_staging_for_original_edited_live_and_video(self):
        # Exercise the real upstream staging method with only Apple's I/O mocked.
        # This catches changes to the private method's required photo properties.
        for edited, live, video in (
            (False, False, False),
            (True, False, False),
            (False, True, False),
            (True, True, False),
            (False, False, True),
        ):
            with self.subTest(edited=edited, live=live, video=video):
                item = photo(
                    hasadjustments=edited,
                    live_photo=live,
                    uti="com.apple.quicktime-movie" if video else "public.jpeg",
                )
                request = self.root / "request.json"
                response = self.root / "response.json"
                fields = {k: v for k, v in vars(item).items() if k != "_verbose"}
                request.write_text(
                    json.dumps(
                        {
                            "photo": fields,
                            "options": {"edited": edited, "live_photo": live},
                        }
                    )
                )
                exported = [str(self.root / ("photo.mov" if video else "photo.jpeg"))]
                if live:
                    exported.append(str(self.root / "photo.mov"))
                with mock.patch("osxphotos.photoexporter.PhotoLibrary") as library:
                    library.return_value.fetch_uuid.return_value.export.return_value = (
                        exported
                    )
                    _worker(request, response)
                result = json.loads(response.read_text())
                self.assertEqual(
                    result["edited" if edited else "original"], exported[0]
                )
                if live:
                    self.assertEqual(
                        result["edited_live" if edited else "original_live"],
                        exported[1],
                    )
                self.assertFalse(result["error"])

    def test_download_failure_makes_export_fail_even_if_upstream_returns_zero(self):
        report = self.root / "export.csv"

        def export(**arguments):
            self.exporter._stage_photo_for_export_with_photokit(ExportOptions())
            return FakeRunner()(arguments)

        with (
            mock.patch(
                "photos_backup.apple_photos.adapter.export_cli", side_effect=export
            ),
            mock.patch(
                "photos_backup.apple_photos.downloads._run_worker",
                side_effect=subprocess.TimeoutExpired("worker", 120),
            ),
        ):
            code = run_osxphotos_export({"report": str(report)})

        self.assertEqual(code, 1)
        self.assertTrue(report.with_suffix(".downloads.json").exists())

    def test_local_assets_are_processed_before_missing_ones(self):
        missing = photo(uuid="cloud", path=None)
        local = photo(uuid="local")
        edited_missing = photo(uuid="edited-cloud", hasadjustments=True)
        with (
            mock.patch("photos_backup.apple_photos.adapter.PhotosDB") as database,
            mock.patch("photos_backup.apple_photos.adapter.export_cli") as export,
        ):
            database.return_value.query.return_value = [missing, local, edited_missing]
            export.side_effect = lambda **args: (
                self.assertEqual(
                    [p.uuid for p in args["db"].query(None)],
                    ["local", "cloud", "edited-cloud"],
                )
                or 0
            )
            result = run_osxphotos_export(
                {"db": "library", "report": str(self.root / "report.csv")},
                local_first=True,
            )
        self.assertEqual(result, 0)


if __name__ == "__main__":
    unittest.main()
