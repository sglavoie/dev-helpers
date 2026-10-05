from __future__ import annotations

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.process import stream_command, transfer_failure


class StreamCommandTests(unittest.TestCase):
    def test_failure_excerpt_prefers_diagnostics_over_later_progress(self):
        output = (
            "\033[31mERROR: permission denied\033[0m\n" + "Transferred: 0 / 2\n" * 20
        )
        detail = transfer_failure("rclone", 5, output)
        self.assertIn("ERROR: permission denied", detail)
        self.assertNotIn("Transferred", detail)
        self.assertNotIn("\033", detail)

    def test_failure_excerpt_is_bounded_and_handles_absent_or_bytes_output(self):
        detail = transfer_failure("rsync", 23, "ERROR: " + "x" * 10000)
        self.assertLess(len(detail), 300)
        self.assertIn("denied", transfer_failure("rsync", 23, b"denied\xff"))
        self.assertIn("See the transfer output", transfer_failure("rsync", 23, None))

    def test_output_is_visible_before_child_finishes_and_stderr_is_drained(self):
        with tempfile.TemporaryDirectory() as directory:
            acknowledgement = Path(directory) / "seen"
            script = """
import pathlib, sys, time
print('started', flush=True)
deadline = time.monotonic() + 5
while not pathlib.Path(sys.argv[1]).exists():
    if time.monotonic() > deadline:
        sys.exit(7)
    time.sleep(0.01)
print('progress\\rfinished', file=sys.stderr, flush=True)
"""

            def show(line, *, nl):
                if "started" in line:
                    acknowledgement.touch()

            with mock.patch(
                "photos_backup.process.click.echo", side_effect=show
            ) as echo:
                result = stream_command(
                    [sys.executable, "-c", script, str(acknowledgement)], check=True
                )
            self.assertIn("started\nprogress\nfinished\n", result.stdout)
            self.assertEqual(
                "".join(call.args[0] for call in echo.call_args_list), result.stdout
            )

    def test_nonzero_exit_is_preserved_or_raised_as_requested(self):
        command = [sys.executable, "-c", "import sys; print('failed'); sys.exit(4)"]
        with mock.patch("photos_backup.process.click.echo"):
            self.assertEqual(stream_command(command).returncode, 4)
            with self.assertRaises(subprocess.CalledProcessError) as raised:
                stream_command(command, check=True)
        self.assertIn("failed", raised.exception.output)

    def test_large_output_is_streamed_but_only_the_tail_is_retained(self):
        with mock.patch("photos_backup.process.click.echo") as echo:
            result = stream_command(
                [sys.executable, "-c", "for i in range(1000): print(i)"]
            )
        self.assertEqual(echo.call_count, 1000)
        self.assertEqual(len(result.stdout.splitlines()), 200)
        self.assertTrue(result.stdout.endswith("999\n"))

    def test_interruption_kills_the_child(self):
        with mock.patch("photos_backup.process.subprocess.Popen") as popen:
            process = popen.return_value.__enter__.return_value
            process.stdout.__iter__.side_effect = KeyboardInterrupt
            with self.assertRaises(KeyboardInterrupt):
                stream_command(["rsync"])
            process.kill.assert_called_once()
