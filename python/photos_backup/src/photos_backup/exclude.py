from __future__ import annotations

from pathlib import Path


def exclude_from_arg(exclude_file: Path | None) -> str:
    """Build the rsync exclude flag, ignoring a file that is not on disk."""
    if exclude_file is None or not exclude_file.is_file():
        return ""
    return f"--exclude-from={exclude_file}"
