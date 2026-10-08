from __future__ import annotations

import importlib
import inspect
import unittest
from types import SimpleNamespace
from unittest import mock

from photos_backup.apple_photos.adapter import _export_progress
from photos_backup.progress import ExportProgress, live_terminal


class ProgressTests(unittest.TestCase):
    def reporter(self, terminal=False):
        self.now = 0.0
        self.lines = []
        return ExportProgress(
            clock=lambda: self.now, sink=self.lines.append, terminal=terminal
        )

    def test_a_dumb_terminal_gets_plain_lines(self):
        tty = SimpleNamespace(isatty=lambda: True)
        with mock.patch(
            "photos_backup.progress.click.get_text_stream", return_value=tty
        ):
            for term, expected in (("xterm-256color", True), ("dumb", False)):
                with self.subTest(term), mock.patch.dict("os.environ", TERM=term):
                    self.assertIs(live_terminal("stderr"), expected)

    def test_redirected_heartbeat_and_remaining_budget(self):
        progress = self.reporter()
        progress.selection(3)
        with progress.phase("Retrieving missing files", "video.mov", budget=12):
            self.now = 9
            progress.tick()
            self.assertEqual(self.lines, [])
            self.now = 10
            progress.tick()
            self.assertIn("budget remaining 2s", self.lines[-1])
            self.assertIn("assets processed 0/3", self.lines[-1])
            self.assertNotIn("\033", self.lines[-1])
            self.now = 15
        self.assertIn("budget remaining 0s", self.lines[-1])
        self.assertEqual(progress.timings["Retrieving missing files"], 15)

    def test_redirected_phase_prints_one_closing_line(self):
        progress = self.reporter()
        with progress.phase("Checking archive"):
            self.now = 2
        self.assertEqual(
            self.lines, ["Checking archive | elapsed 2.0s | unresolved downloads 0"]
        )

    def test_terminal_updates_each_second_and_does_not_claim_completion(self):
        progress = self.reporter(terminal=True)
        progress.selection(2)
        with progress.phase("Exporting asset", "a.jpg", announce=False):
            self.now = 1
            progress.tick()
            self.assertIn("assets processed 0/2", self.lines[-1])
            self.assertTrue(self.lines[-1].startswith("\r\033[2K"))
            progress.asset_done()
            self.now = 2
            progress.tick()
            self.assertIn("assets processed 1/2", self.lines[-1])

    def test_nested_phase_restores_asset_and_accumulates_time(self):
        progress = self.reporter()
        with progress.phase("Exporting asset", "a.jpg"):
            self.now = 1
            with progress.phase("Retrieving missing files", "a.jpg", budget=10):
                self.now = 4
            self.now = 5
            progress.tick(force=True)
            self.assertIn("Exporting asset: a.jpg", self.lines[-1])
            self.assertNotIn("budget remaining", self.lines[-1])
        self.assertEqual(progress.timings["Exporting asset"], 5)
        self.assertEqual(progress.timings["Retrieving missing files"], 3)

    def test_context_stops_heartbeat_on_exception(self):
        progress = self.reporter()
        with self.assertRaises(RuntimeError), progress:
            raise RuntimeError("failed")
        self.assertFalse(progress._thread.is_alive())

    def test_export_observer_keeps_upstream_introspection_and_restores_on_error(self):
        module = importlib.import_module("osxphotos.cli.export")
        original = module.export_photo
        original_args = inspect.getfullargspec(original).args
        progress = self.reporter()
        progress.selection(2)

        failing_photo_num = 2

        def export_photo(photo, photo_num=0):
            if photo_num == failing_photo_num:
                raise RuntimeError("failed")
            return "result"

        with mock.patch.object(module, "export_photo", export_photo):
            with self.assertRaises(RuntimeError), _export_progress(progress):
                self.assertEqual(
                    inspect.getfullargspec(module.export_photo).args,
                    ["photo", "photo_num"],
                )
                photo = SimpleNamespace(original_filename="a.jpg")
                self.assertEqual(
                    module.export_photo(photo=photo, photo_num=1), "result"
                )
                module.export_photo(photo=photo, photo_num=2)
            self.assertIs(module.export_photo, export_photo)
        self.assertIs(module.export_photo, original)
        self.assertEqual(progress.processed, 1)
        with _export_progress(progress):
            self.assertEqual(
                inspect.getfullargspec(module.export_photo).args, original_args
            )
