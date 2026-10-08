"""Read a bounded display sample from the latest attempt's download report."""

import json
import stat
from collections import Counter
from pathlib import Path
from typing import Any


def summarize_downloads(
    attempt: dict[str, Any] | None, archive: Path
) -> tuple[dict[str, Any] | None, str | None]:
    if (
        not attempt
        or attempt["status"] == "succeeded"
        or not attempt.get("download_report_path")
    ):
        return None, None
    path = Path(attempt["download_report_path"])
    try:
        # The attempt reader constrains relative paths; also refuse symlink escapes.
        current = archive
        for part in path.relative_to(archive).parts:
            current = current / part
            if stat.S_ISLNK(current.lstat().st_mode):
                raise ValueError("download report must not use symlinks")
        if not stat.S_ISREG(path.lstat().st_mode):
            raise ValueError("download report must be a regular file")
        rows = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(rows, list) or any(
            not isinstance(row, dict)
            or any(
                not isinstance(row.get(field), str) for field in ("filename", "reason")
            )
            for row in rows
        ):
            raise ValueError("invalid download report")
        reasons = Counter(
            "timed out"
            if "timed out" in row["reason"].lower()
            or "timeout" in row["reason"].lower()
            else "retrieval error"
            for row in rows
        )
        return {
            "count": len(rows),
            "reasons": dict(reasons),
            "samples": [
                {"filename": row["filename"], "reason": row["reason"]}
                for row in rows[:3]
            ],
            "report_path": str(path),
        }, None
    except FileNotFoundError:
        return None, None
    except (OSError, ValueError) as error:
        return None, f"Could not read download report '{path}': {error}"
