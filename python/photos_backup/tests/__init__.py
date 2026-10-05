import tempfile
from pathlib import Path
from unittest import mock


def isolate_transfer_history(test):
    """Keep command tests from reading or writing the user's transfer receipts."""
    root = Path(test.enterContext(tempfile.TemporaryDirectory())) / "history"
    test.enterContext(
        mock.patch("photos_backup.transfers.history_root", return_value=root)
    )
    return root
