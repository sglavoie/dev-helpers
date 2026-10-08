from __future__ import annotations

import os
import signal
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import _interrupt, cli, install_termination_handlers
from photos_backup.process import stream_command


class TerminationSignalTests(unittest.TestCase):
    def setUp(self) -> None:
        for signum in (signal.SIGTERM, signal.SIGHUP):
            self.addCleanup(signal.signal, signum, signal.getsignal(signum))

    def test_termination_signals_raise_keyboard_interrupt(self):
        install_termination_handlers()
        for signum in (signal.SIGTERM, signal.SIGHUP):
            with self.subTest(signum=signum):
                self.assertIs(signal.getsignal(signum), _interrupt)
                with self.assertRaises(KeyboardInterrupt):
                    _interrupt(signum, None)

    def test_cli_installs_the_handlers(self):
        signal.signal(signal.SIGTERM, signal.SIG_DFL)
        with tempfile.TemporaryDirectory() as directory:
            CliRunner().invoke(
                cli, ["--config", str(Path(directory) / "missing.toml"), "status"]
            )
        self.assertIs(signal.getsignal(signal.SIGTERM), _interrupt)

    def test_sigterm_stops_the_transfer_child(self):
        install_termination_handlers()
        child = [
            sys.executable,
            "-c",
            "import os, time; print(os.getpid(), flush=True); time.sleep(60)",
        ]
        pids: list[int] = []

        def echo(line: str, nl: bool = True) -> None:
            pids.append(int(line))
            os.kill(os.getpid(), signal.SIGTERM)

        with mock.patch("photos_backup.process.click.echo", side_effect=echo):
            with self.assertRaises(KeyboardInterrupt):
                stream_command(child)
        # Popen's context manager reaped the killed child before re-raising.
        with self.assertRaises(ProcessLookupError):
            os.kill(pids[0], 0)


if __name__ == "__main__":
    unittest.main()
