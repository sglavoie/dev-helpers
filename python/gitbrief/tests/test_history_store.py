"""Tests for history storage: saving, listing, lookup, and clearing."""

import json
import re
from datetime import datetime, timedelta
from pathlib import Path

import pytest

from gitbrief.history import (
    clear_history,
    get_history_entry,
    list_history,
    parse_older_than,
    save_summary,
)

STEM_OLD = "2026-01-01_100000"
STEM_NEW = "2026-03-01_100000"


@pytest.fixture
def two_entries(write_history) -> None:
    """Two history entries: alpha (older) and beta (newer)."""
    write_history(STEM_OLD, projects=["alpha"], summary="summary alpha")
    write_history(STEM_NEW, projects=["beta"], summary="summary beta")


# ---------------------------------------------------------------------------
# parse_older_than
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    "value,expected",
    [
        ("1d", 1),
        ("7d", 7),
        ("2w", 14),
        ("1m", 30),
        ("1y", 365),
        ("3m", 90),
    ],
)
def test_parse_older_than_valid(value: str, expected: int) -> None:
    assert parse_older_than(value) == expected


@pytest.mark.parametrize("bad", ["", "bad", "1x", "0", "d1", "1.5d"])
def test_parse_older_than_invalid(bad: str) -> None:
    with pytest.raises(ValueError):
        parse_older_than(bad)


# ---------------------------------------------------------------------------
# save_summary / list_history
# ---------------------------------------------------------------------------


def test_save_summary_creates_file(history_dir: Path) -> None:
    path = save_summary(
        projects=["proj1"],
        since="2026-03-01",
        until=None,
        backend="claude",
        commit_count=5,
        summary="## Summary\n- did stuff",
    )
    assert path.exists()
    record = json.loads(path.read_text())
    assert record["projects"] == ["proj1"]
    assert record["since"] == "2026-03-01"
    assert record["until"] is None
    assert record["commit_count"] == 5
    assert "## Summary" in record["summary"]
    assert "timestamp" in record


def test_save_summary_filename_format(history_dir: Path) -> None:
    path = save_summary(
        projects=["p"],
        since="2026-01-01",
        until=None,
        backend="claude",
        commit_count=1,
        summary="x",
    )
    # Filename should be YYYY-MM-DD_HHMMSS.json
    assert re.match(r"\d{4}-\d{2}-\d{2}_\d{6}\.json", path.name)


def test_list_history_empty(history_dir: Path) -> None:
    assert list_history() == []


def test_list_history_newest_first(write_history) -> None:
    write_history(STEM_OLD, projects=["a"], summary="old")
    write_history(STEM_NEW, projects=["b"], commit_count=2, summary="new")

    entries = list_history()

    assert len(entries) == 2
    assert entries[0][0] == STEM_NEW  # newest first
    assert entries[1][0] == STEM_OLD


def test_list_history_skips_corrupt_files(history_dir: Path, write_history) -> None:
    write_history("2026-02-01_100000", projects=["ok"], commit_count=3, summary="good")
    (history_dir / "2026-01-01_100000.json").write_text("not json")

    entries = list_history()

    assert len(entries) == 1
    assert entries[0][0] == "2026-02-01_100000"


# ---------------------------------------------------------------------------
# get_history_entry
# ---------------------------------------------------------------------------


def test_get_history_entry_by_index(two_entries) -> None:
    r1 = get_history_entry("1")
    r2 = get_history_entry("2")

    assert r1 is not None and r1["projects"] == ["beta"]  # index 1 = newest
    assert r2 is not None and r2["projects"] == ["alpha"]


def test_get_history_entry_by_stem(two_entries) -> None:
    r = get_history_entry(STEM_OLD)

    assert r is not None
    assert r["projects"] == ["alpha"]


def test_get_history_entry_not_found(two_entries) -> None:
    assert get_history_entry("999") is None
    assert get_history_entry("nonexistent-stem") is None


# ---------------------------------------------------------------------------
# clear_history
# ---------------------------------------------------------------------------


def test_clear_history_all(two_entries) -> None:
    count = clear_history()

    assert count == 2
    assert list_history() == []


def test_clear_history_older_than(write_history) -> None:
    """Only entries older than the cutoff should be deleted."""
    now = datetime.now()
    old, new = now - timedelta(days=60), now - timedelta(days=1)
    new_ts = new.isoformat(timespec="seconds")
    write_history(
        old.strftime("%Y-%m-%d_%H%M%S"), timestamp=old.isoformat(timespec="seconds")
    )
    write_history(new.strftime("%Y-%m-%d_%H%M%S"), timestamp=new_ts)

    count = clear_history(older_than_days=30)
    entries = list_history()

    assert count == 1
    assert len(entries) == 1
    assert entries[0][1]["timestamp"] == new_ts


def test_clear_history_no_dir(history_dir: Path) -> None:
    # History dir doesn't exist yet
    assert clear_history() == 0
