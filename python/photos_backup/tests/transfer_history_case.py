"""Shared fixture for the transfer history test modules."""

import tempfile
import unittest
from pathlib import Path

from photos_backup.summary import BackupSummary
from photos_backup.transfers import TransferHistory
from tests import isolate_transfer_history


class TransferHistoryTestCase(unittest.TestCase):
    def setUp(self):
        self.root = Path(self.enterContext(tempfile.TemporaryDirectory())).resolve()
        self.history_root = isolate_transfer_history(self)
        self.config = self.root / "config.toml"
        self.history = TransferHistory(self.config)
        self.source = self.root / "photos"
        self.destination = self.root / "backup/photos"

    def run_transfer(self, operation=None, *, dry_run=False, destination=None):
        return self.history.run(
            "SSD: All Photos",
            self.source,
            destination or self.destination,
            operation
            or (lambda: BackupSummary("SSD: All Photos", files_transferred=4)),
            dry_run=dry_run,
        )

    def receipt(self):
        receipts, errors = self.history.read()
        self.assertEqual(errors, [])
        self.assertEqual(len(receipts), 1)
        return receipts[0]
