"""Step summaries, rsync statistics, and byte formatting."""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import TypedDict


_SCALE = 1024


@dataclass
class BackupSummary:
    step_name: str
    files_transferred: int | None = None
    total_size: str = ""
    elapsed_seconds: float = 0.0
    skipped: bool = False
    planned: bool = False
    error: str | None = None
    dry_run: bool = False
    action_required: bool = False
    skip_reason: str | None = None


class TransferStats(TypedDict):
    files_transferred: int | None
    total_size: str


def parse_rsync_stats(output: str) -> TransferStats:
    """Parse rsync --stats output for file count and total size.

    GNU rsync reports "regular files"; macOS openrsync (/usr/bin/rsync) reports
    "files". Both count regular files only.
    """
    result: TransferStats = {"files_transferred": None, "total_size": ""}

    files_match = re.search(
        r"Number of (?:regular )?files transferred:\s*([\d,]+)", output
    )
    if files_match:
        result["files_transferred"] = int(files_match.group(1).replace(",", ""))

    # GNU -h prints "1.50M bytes", plain GNU "1,500,006 bytes", openrsync "1500 kB".
    size_match = re.search(
        r"Total transferred file size:\s*([\d,.]+\s*\S+(?: bytes)?)", output
    )
    if size_match:
        result["total_size"] = size_match.group(1)

    return result


def human_size(total: int) -> str:
    """Render a byte count the way a person reading a deletion prompt needs it."""
    size = float(total)
    for unit in ("B", "KB", "MB", "GB"):
        if size < _SCALE:
            return f"{size:.0f} {unit}" if unit == "B" else f"{size:.1f} {unit}"
        size /= _SCALE
    return f"{size:.1f} TB"
