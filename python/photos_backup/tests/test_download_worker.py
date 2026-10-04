"""Exercise the real control channel with a child that cannot access Photos."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.downloads import DownloadWorker

# Use the production server and replace only the native staging operation.
FAKE_WORKER = """
import json, os, sys, time
from pathlib import Path
from photos_backup.apple_photos import downloads

def stage(request, response):
    mode = request.read_text()
    if mode == "block":
        time.sleep(30)
    if mode == "crash":
        os._exit(7)
    if mode == "error":
        raise ValueError("native staging failed")
    print("native stdout noise", flush=True)
    print("native stderr noise", file=sys.stderr, flush=True)
    response.write_text(json.dumps({"pid": os.getpid()}))

downloads._worker = stage
downloads._serve(int(sys.argv[-1]))
"""


class WorkerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.request = self.root / "request"
        self.response = self.root / "response"
        self.worker = DownloadWorker()
        self.addCleanup(self.worker.close)
        self.real_popen = subprocess.Popen

    def spawn(self, args, **kwargs):
        return self.real_popen([sys.executable, "-c", FAKE_WORKER, args[-1]], **kwargs)

    def run_request(self, mode="ok", timeout=10):
        self.request.write_text(mode)
        self.worker.run(self.request, self.response, timeout)
        return json.loads(self.response.read_text())["pid"]

    def test_many_requests_share_one_process_and_shutdown_reaps_it(self):
        with mock.patch(
            "photos_backup.apple_photos.downloads.subprocess.Popen",
            side_effect=self.spawn,
        ) as spawn:
            pids = [self.run_request() for _ in range(4)]
        self.assertEqual(len(set(pids)), 1)
        spawn.assert_called_once()
        process = self.worker.process
        self.worker.close()
        self.assertIsNotNone(process.returncode)
        with self.assertRaises(ProcessLookupError):
            os.kill(pids[0], 0)

    def test_timeout_reaps_worker_and_next_request_uses_replacement(self):
        with mock.patch(
            "photos_backup.apple_photos.downloads.subprocess.Popen",
            side_effect=self.spawn,
        ):
            pid = self.run_request()
            with self.assertRaises(subprocess.TimeoutExpired):
                self.run_request("block", timeout=0.05)
            with self.assertRaises(ProcessLookupError):
                os.kill(pid, 0)
            self.assertNotEqual(self.run_request(), pid)

    def test_crash_or_reported_error_restarts_worker(self):
        with mock.patch(
            "photos_backup.apple_photos.downloads.subprocess.Popen",
            side_effect=self.spawn,
        ):
            for mode in ("crash", "error"):
                pid = self.run_request()
                with self.assertRaises(OSError):
                    self.run_request(mode)
                self.assertIsNone(self.worker.process)
                self.assertNotEqual(self.run_request(), pid)

    def test_bad_protocol_is_rejected_and_process_reaped(self):
        for reply in (b"not json\n", b'{"id": 999, "ok": true}\n', b'{"id": 1}\n'):
            script = (
                "import socket,sys,time; "
                "s=socket.socket(fileno=int(sys.argv[-1])); s.recv(4096); "
                f"s.sendall({reply!r}); time.sleep(30)"
            )
            processes = []

            def spawn(args, **kwargs):
                process = self.real_popen(
                    [sys.executable, "-c", script, args[-1]], **kwargs
                )
                processes.append(process)
                return process

            with mock.patch(
                "photos_backup.apple_photos.downloads.subprocess.Popen",
                side_effect=spawn,
            ):
                with self.assertRaises(ValueError):
                    self.run_request()
            self.assertIsNotNone(processes[0].returncode)
            self.assertIsNone(self.worker.process)

    def test_keyboard_interrupt_reaps_active_worker(self):
        with mock.patch(
            "photos_backup.apple_photos.downloads.subprocess.Popen",
            side_effect=self.spawn,
        ):
            pid = self.run_request()
            with mock.patch(
                "photos_backup.apple_photos.downloads.select.select",
                side_effect=KeyboardInterrupt,
            ):
                with self.assertRaises(KeyboardInterrupt):
                    self.run_request("block")
            with self.assertRaises(ProcessLookupError):
                os.kill(pid, 0)
