"""Shared fixture for the copy workflow test modules."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest import mock

from tests import isolate_transfer_history


class CopyWorkflowTestCase(unittest.TestCase):
    def setUp(self) -> None:
        isolate_transfer_history(self)
        self.enterContext(
            mock.patch(
                "photos_backup.cli.backup_all.which", return_value="/test/bin/tool"
            )
        )
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.source = self.root / "Apple Photos' originals"
        self.source.mkdir()
        (self.source / "IMG_0001.JPG").write_text("photo")
        self.destination = self.root / "new parent" / "Backup Photos"
        self.exclude = self.root / "exclude photos.txt"
        self.exclude.write_text("*.tmp\n")
