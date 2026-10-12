"""Recorded archive, transfer, and verification status for `status`."""

import click  # noqa: F401  (tests patch `photos_backup.status_report.click.echo`)

from photos_backup.status_report.archive import (
    print_archive_status,
    print_export_attempt,
)
from photos_backup.status_report.downloads import (
    print_download_summary,
    print_verification_history,
)
from photos_backup.status_report.short import print_short_status
from photos_backup.status_report.transfer_history import (
    _transfer_suggestions,
    print_transfer_history,
)

__all__ = [
    "_transfer_suggestions",
    "print_archive_status",
    "print_download_summary",
    "print_export_attempt",
    "print_short_status",
    "print_transfer_history",
    "print_verification_history",
]
