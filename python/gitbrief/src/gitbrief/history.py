from __future__ import annotations

import json
import re
from datetime import datetime, timedelta
from pathlib import Path

GITBRIEF_DIR = Path.home() / ".gitbrief"
HISTORY_DIR = GITBRIEF_DIR / "history"


def _ensure_history_dir() -> None:
    HISTORY_DIR.mkdir(parents=True, exist_ok=True)


def save_summary(
    projects: list[str],
    since: str,
    until: str | None,
    backend: str,
    commit_count: int,
    summary: str,
) -> Path:
    """Save a summary record to history and return the file path."""
    _ensure_history_dir()
    ts = datetime.now()
    filename = ts.strftime("%Y-%m-%d_%H%M%S") + ".json"
    record = {
        "timestamp": ts.isoformat(timespec="seconds"),
        "projects": projects,
        "since": since,
        "until": until,
        "backend": backend,
        "commit_count": commit_count,
        "summary": summary,
    }
    path = HISTORY_DIR / filename
    path.write_text(json.dumps(record, indent=2) + "\n")
    return path


def list_history() -> list[tuple[str, dict]]:
    """Return list of (stem, record) sorted newest-first."""
    if not HISTORY_DIR.exists():
        return []
    entries = []
    for f in sorted(HISTORY_DIR.glob("*.json"), reverse=True):
        try:
            record = json.loads(f.read_text())
            entries.append((f.stem, record))
        except (json.JSONDecodeError, OSError):
            continue
    return entries


def get_history_entry(id_: str) -> dict | None:
    """Get a history entry by 1-based index or filename stem."""
    entries = list_history()
    try:
        idx = int(id_)
        if 1 <= idx <= len(entries):
            return entries[idx - 1][1]
        return None
    except ValueError:
        pass
    for stem, record in entries:
        if stem == id_:
            return record
    return None


def clear_history(older_than_days: int | None = None) -> int:
    """Delete history entries, optionally only those older than N days.

    Returns the number of entries deleted.
    """
    if not HISTORY_DIR.exists():
        return 0
    cutoff: datetime | None = None
    if older_than_days is not None:
        cutoff = datetime.now() - timedelta(days=older_than_days)
    count = 0
    for f in HISTORY_DIR.glob("*.json"):
        if cutoff is not None:
            try:
                record = json.loads(f.read_text())
                ts = datetime.fromisoformat(record["timestamp"])
                if ts >= cutoff:
                    continue
            except (json.JSONDecodeError, KeyError, ValueError, OSError):
                pass
        f.unlink()
        count += 1
    return count


def parse_older_than(value: str) -> int:
    """Parse '30d', '4w', '2m', '1y' into approximate number of days."""
    m = re.fullmatch(r"(\d+)([dwmy])", value)
    if not m:
        raise ValueError(
            f"Invalid --older-than value: {value!r}. Use e.g. 30d, 4w, 2m, 1y"
        )
    n, unit = int(m.group(1)), m.group(2)
    multipliers = {"d": 1, "w": 7, "m": 30, "y": 365}
    return n * multipliers[unit]
