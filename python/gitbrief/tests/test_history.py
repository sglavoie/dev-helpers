"""Tests for history storage, retrieval, clearing, and --since-last integration."""

import json
from pathlib import Path
from unittest.mock import patch

import pytest
from click.testing import CliRunner

import gitbrief.history as history_mod
from gitbrief.history import (
    clear_history,
    get_history_entry,
    list_history,
    parse_older_than,
    save_summary,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _patch_history_dir(tmp_path: Path):
    """Context manager that redirects history to a tmp directory."""
    hist_dir = tmp_path / "history"
    return patch.multiple(
        history_mod,
        GITBRIEF_DIR=tmp_path,
        HISTORY_DIR=hist_dir,
    )


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


def test_save_summary_creates_file(tmp_path: Path) -> None:
    with _patch_history_dir(tmp_path):
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


def test_save_summary_filename_format(tmp_path: Path) -> None:
    with _patch_history_dir(tmp_path):
        path = save_summary(
            projects=["p"],
            since="2026-01-01",
            until=None,
            backend="claude",
            commit_count=1,
            summary="x",
        )
    # Filename should be YYYY-MM-DD_HHMMSS.json
    import re

    assert re.match(r"\d{4}-\d{2}-\d{2}_\d{6}\.json", path.name)


def test_list_history_empty(tmp_path: Path) -> None:
    with _patch_history_dir(tmp_path):
        assert list_history() == []


def test_list_history_newest_first(tmp_path: Path) -> None:
    with _patch_history_dir(tmp_path):
        # Create two records with slightly different timestamps
        hist_dir = tmp_path / "history"
        hist_dir.mkdir(parents=True)
        older = hist_dir / "2026-01-01_100000.json"
        newer = hist_dir / "2026-03-01_100000.json"
        older.write_text(
            json.dumps(
                {
                    "timestamp": "2026-01-01T10:00:00",
                    "projects": ["a"],
                    "since": "2025-12-01",
                    "until": None,
                    "backend": "claude",
                    "commit_count": 1,
                    "summary": "old",
                }
            )
        )
        newer.write_text(
            json.dumps(
                {
                    "timestamp": "2026-03-01T10:00:00",
                    "projects": ["b"],
                    "since": "2026-02-01",
                    "until": None,
                    "backend": "claude",
                    "commit_count": 2,
                    "summary": "new",
                }
            )
        )

        entries = list_history()

    assert len(entries) == 2
    assert entries[0][0] == "2026-03-01_100000"  # newest first
    assert entries[1][0] == "2026-01-01_100000"


def test_list_history_skips_corrupt_files(tmp_path: Path) -> None:
    with _patch_history_dir(tmp_path):
        hist_dir = tmp_path / "history"
        hist_dir.mkdir(parents=True)
        (hist_dir / "2026-01-01_100000.json").write_text("not json")
        (hist_dir / "2026-02-01_100000.json").write_text(
            json.dumps(
                {
                    "timestamp": "2026-02-01T10:00:00",
                    "projects": ["ok"],
                    "since": "2026-01-01",
                    "until": None,
                    "backend": "claude",
                    "commit_count": 3,
                    "summary": "good",
                }
            )
        )
        entries = list_history()

    assert len(entries) == 1
    assert entries[0][0] == "2026-02-01_100000"


# ---------------------------------------------------------------------------
# get_history_entry
# ---------------------------------------------------------------------------


def _populate_history(tmp_path: Path) -> tuple[str, str]:
    """Create two history entries and return their stems (newest first)."""
    hist_dir = tmp_path / "history"
    hist_dir.mkdir(parents=True)
    stem_old = "2026-01-01_100000"
    stem_new = "2026-03-01_100000"
    for stem, ts, proj in [
        (stem_old, "2026-01-01T10:00:00", "alpha"),
        (stem_new, "2026-03-01T10:00:00", "beta"),
    ]:
        (hist_dir / f"{stem}.json").write_text(
            json.dumps(
                {
                    "timestamp": ts,
                    "projects": [proj],
                    "since": "2025-12-01",
                    "until": None,
                    "backend": "claude",
                    "commit_count": 1,
                    "summary": f"summary {proj}",
                }
            )
        )
    return stem_new, stem_old  # newest first (index 1, 2)


def test_get_history_entry_by_index(tmp_path: Path) -> None:
    stem_new, stem_old = _populate_history(tmp_path)
    with _patch_history_dir(tmp_path):
        r1 = get_history_entry("1")
        r2 = get_history_entry("2")

    assert r1 is not None and r1["projects"] == ["beta"]  # index 1 = newest
    assert r2 is not None and r2["projects"] == ["alpha"]


def test_get_history_entry_by_stem(tmp_path: Path) -> None:
    stem_new, stem_old = _populate_history(tmp_path)
    with _patch_history_dir(tmp_path):
        r = get_history_entry(stem_old)

    assert r is not None
    assert r["projects"] == ["alpha"]


def test_get_history_entry_not_found(tmp_path: Path) -> None:
    _populate_history(tmp_path)
    with _patch_history_dir(tmp_path):
        assert get_history_entry("999") is None
        assert get_history_entry("nonexistent-stem") is None


# ---------------------------------------------------------------------------
# clear_history
# ---------------------------------------------------------------------------


def test_clear_history_all(tmp_path: Path) -> None:
    _populate_history(tmp_path)
    with _patch_history_dir(tmp_path):
        count = clear_history()
        remaining = list_history()

    assert count == 2
    assert remaining == []


def test_clear_history_older_than(tmp_path: Path) -> None:
    """Only entries older than the cutoff should be deleted."""
    hist_dir = tmp_path / "history"
    hist_dir.mkdir(parents=True)
    # One old entry (60 days ago), one recent (1 day ago)
    from datetime import datetime, timedelta

    now = datetime.now()
    old_ts = (now - timedelta(days=60)).isoformat(timespec="seconds")
    new_ts = (now - timedelta(days=1)).isoformat(timespec="seconds")
    old_stem = (now - timedelta(days=60)).strftime("%Y-%m-%d_%H%M%S")
    new_stem = (now - timedelta(days=1)).strftime("%Y-%m-%d_%H%M%S")

    for stem, ts in [(old_stem, old_ts), (new_stem, new_ts)]:
        (hist_dir / f"{stem}.json").write_text(
            json.dumps(
                {
                    "timestamp": ts,
                    "projects": ["x"],
                    "since": "2026-01-01",
                    "until": None,
                    "backend": "claude",
                    "commit_count": 1,
                    "summary": "s",
                }
            )
        )

    with _patch_history_dir(tmp_path):
        count = clear_history(older_than_days=30)
        entries = list_history()

    assert count == 1
    assert len(entries) == 1
    assert entries[0][1]["timestamp"] == new_ts


def test_clear_history_no_dir(tmp_path: Path) -> None:
    with _patch_history_dir(tmp_path):
        # History dir doesn't exist yet
        assert clear_history() == 0


# ---------------------------------------------------------------------------
# Config migration
# ---------------------------------------------------------------------------


def test_config_migration_old_to_new(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """load_config() should migrate ~/.gitbrief.json -> ~/.gitbrief/config.json."""
    import gitbrief.config as config_mod

    old_path = tmp_path / ".gitbrief.json"
    new_dir = tmp_path / ".gitbrief"
    new_path = new_dir / "config.json"

    old_config = {
        "projects": {"myproj": {"path": "/some/path", "backend": None}},
        "settings": {
            "backend": "claude",
            "timeout": 120,
            "retries": 2,
            "max_commits": 100,
        },
    }
    old_path.write_text(json.dumps(old_config))

    monkeypatch.setattr(config_mod, "OLD_CONFIG_PATH", old_path)
    monkeypatch.setattr(config_mod, "NEW_CONFIG_PATH", new_path)
    monkeypatch.setattr(config_mod, "GITBRIEF_DIR", new_dir)
    monkeypatch.setattr(config_mod, "CONFIG_PATH", new_path)

    loaded = config_mod.load_config()

    # Old file should be gone, new file should exist
    assert not old_path.exists()
    assert new_path.exists()
    assert loaded["projects"]["myproj"]["path"] == "/some/path"
    assert loaded.get("last_summary") == {}


def test_config_new_path_takes_priority(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """When both old and new config exist, new config is used (no migration)."""
    import gitbrief.config as config_mod

    old_path = tmp_path / ".gitbrief.json"
    new_dir = tmp_path / ".gitbrief"
    new_dir.mkdir()
    new_path = new_dir / "config.json"

    old_config = {
        "projects": {"old": {"path": "/old", "backend": None}},
        "settings": {},
        "last_summary": {},
    }
    new_config = {
        "projects": {"new": {"path": "/new", "backend": None}},
        "settings": {},
        "last_summary": {},
    }
    old_path.write_text(json.dumps(old_config))
    new_path.write_text(json.dumps(new_config))

    monkeypatch.setattr(config_mod, "OLD_CONFIG_PATH", old_path)
    monkeypatch.setattr(config_mod, "NEW_CONFIG_PATH", new_path)
    monkeypatch.setattr(config_mod, "GITBRIEF_DIR", new_dir)
    monkeypatch.setattr(config_mod, "CONFIG_PATH", new_path)

    loaded = config_mod.load_config()
    assert "new" in loaded["projects"]
    assert "old" not in loaded["projects"]
    assert old_path.exists()  # not deleted


# ---------------------------------------------------------------------------
# last_summary tracking
# ---------------------------------------------------------------------------


def test_get_set_last_summary(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    import gitbrief.config as config_mod

    new_dir = tmp_path / ".gitbrief"
    new_dir.mkdir()
    new_path = new_dir / "config.json"
    old_path = tmp_path / ".gitbrief.json"

    monkeypatch.setattr(config_mod, "OLD_CONFIG_PATH", old_path)
    monkeypatch.setattr(config_mod, "NEW_CONFIG_PATH", new_path)
    monkeypatch.setattr(config_mod, "GITBRIEF_DIR", new_dir)
    monkeypatch.setattr(config_mod, "CONFIG_PATH", new_path)

    assert config_mod.get_last_summary("proj1") is None

    config_mod.set_last_summary("proj1", "2026-03-15T10:00:00")
    assert config_mod.get_last_summary("proj1") == "2026-03-15T10:00:00"

    # A second project is independent
    assert config_mod.get_last_summary("proj2") is None
    config_mod.set_last_summary("proj2", "2026-03-20T08:00:00")
    assert config_mod.get_last_summary("proj2") == "2026-03-20T08:00:00"
    assert config_mod.get_last_summary("proj1") == "2026-03-15T10:00:00"


# ---------------------------------------------------------------------------
# --since-last CLI flag
# ---------------------------------------------------------------------------


def _make_config(tmp_path: Path, last_summary: dict | None = None) -> Path:
    """Write a minimal config with optional last_summary entries."""
    config = {
        "projects": {"myproj": {"path": str(tmp_path / "repo"), "backend": None}},
        "settings": {
            "backend": "claude",
            "timeout": 120,
            "retries": 2,
            "max_commits": 100,
        },
        "last_summary": last_summary or {},
    }
    cfg_path = tmp_path / ".gitbrief" / "config.json"
    cfg_path.parent.mkdir(parents=True, exist_ok=True)
    cfg_path.write_text(json.dumps(config))
    return cfg_path


def test_since_last_requires_no_last_or_since() -> None:
    from gitbrief.cli import cli

    runner = CliRunner()
    result = runner.invoke(cli, ["summary", "--since-last", "--last", "1w", "myproj"])
    assert result.exit_code != 0
    assert "--since-last cannot be combined" in result.output


def test_since_last_needs_one_of_three() -> None:
    from gitbrief.cli import cli

    runner = CliRunner()
    result = runner.invoke(cli, ["summary", "myproj"])
    assert result.exit_code != 0
    assert "Specify --last, --since, or --since-last" in result.output


def test_since_last_uses_history_timestamp(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """--since-last should use the stored timestamp as the since date."""
    import gitbrief.config as config_mod
    from gitbrief.cli import cli

    # Set up a repo dir (just needs to look like a git repo for validate_repo)
    repo_dir = tmp_path / "repo"
    repo_dir.mkdir()
    (repo_dir / ".git").mkdir()

    new_dir = tmp_path / ".gitbrief"
    cfg_path = _make_config(tmp_path, last_summary={"myproj": "2026-03-10T12:00:00"})

    monkeypatch.setattr(config_mod, "OLD_CONFIG_PATH", tmp_path / ".gitbrief.json")
    monkeypatch.setattr(config_mod, "NEW_CONFIG_PATH", cfg_path)
    monkeypatch.setattr(config_mod, "GITBRIEF_DIR", new_dir)
    monkeypatch.setattr(config_mod, "CONFIG_PATH", cfg_path)

    runner = CliRunner()
    with patch("gitbrief.git.extract_commits", return_value=[]) as mock_extract:
        runner.invoke(cli, ["summary", "--since-last", "--dry-run", "myproj"])

    # Even with no commits, extract_commits should have been called with the stored date
    if mock_extract.called:
        args, kwargs = mock_extract.call_args
        assert args[1] == "2026-03-10"  # date portion of the stored timestamp


def test_since_last_falls_back_to_1w_when_no_history(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """When no history exists for a project, --since-last falls back to 1w."""
    import gitbrief.config as config_mod
    from gitbrief.cli import cli

    repo_dir = tmp_path / "repo"
    repo_dir.mkdir()
    (repo_dir / ".git").mkdir()

    new_dir = tmp_path / ".gitbrief"
    cfg_path = _make_config(tmp_path, last_summary={})  # no history

    monkeypatch.setattr(config_mod, "OLD_CONFIG_PATH", tmp_path / ".gitbrief.json")
    monkeypatch.setattr(config_mod, "NEW_CONFIG_PATH", cfg_path)
    monkeypatch.setattr(config_mod, "GITBRIEF_DIR", new_dir)
    monkeypatch.setattr(config_mod, "CONFIG_PATH", cfg_path)

    from gitbrief.git import parse_duration

    expected_fallback = parse_duration("1w")

    runner = CliRunner()
    with patch("gitbrief.git.extract_commits", return_value=[]) as mock_extract:
        runner.invoke(cli, ["summary", "--since-last", "--dry-run", "myproj"])

    if mock_extract.called:
        args, _ = mock_extract.call_args
        assert args[1] == expected_fallback


# ---------------------------------------------------------------------------
# history CLI commands
# ---------------------------------------------------------------------------


def test_history_list_empty(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    from gitbrief.cli import cli

    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", tmp_path)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", tmp_path / "history")

    runner = CliRunner()
    result = runner.invoke(cli, ["history", "list"])
    assert result.exit_code == 0
    assert "No history found" in result.output


def test_history_list_shows_entries(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from gitbrief.cli import cli

    hist_dir = tmp_path / "history"
    hist_dir.mkdir(parents=True)
    (hist_dir / "2026-03-01_100000.json").write_text(
        json.dumps(
            {
                "timestamp": "2026-03-01T10:00:00",
                "projects": ["proj1"],
                "since": "2026-02-01",
                "until": None,
                "backend": "claude",
                "commit_count": 7,
                "summary": "## Summary\n- did stuff",
            }
        )
    )

    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", tmp_path)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", hist_dir)

    runner = CliRunner()
    result = runner.invoke(cli, ["history", "list"])
    assert result.exit_code == 0
    assert "proj1" in result.output
    assert "7 commits" in result.output


def test_history_show_by_index(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    from gitbrief.cli import cli

    hist_dir = tmp_path / "history"
    hist_dir.mkdir(parents=True)
    (hist_dir / "2026-03-01_100000.json").write_text(
        json.dumps(
            {
                "timestamp": "2026-03-01T10:00:00",
                "projects": ["proj1"],
                "since": "2026-02-01",
                "until": None,
                "backend": "claude",
                "commit_count": 3,
                "summary": "# The Summary\n- shipped feature",
            }
        )
    )

    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", tmp_path)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", hist_dir)

    runner = CliRunner()
    result = runner.invoke(cli, ["history", "show", "1"])
    assert result.exit_code == 0
    assert "shipped feature" in result.output


def test_history_show_not_found(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from gitbrief.cli import cli

    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", tmp_path)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", tmp_path / "history")

    runner = CliRunner()
    result = runner.invoke(cli, ["history", "show", "99"])
    assert result.exit_code != 0
    assert "No history entry found" in result.output


def test_history_clear_with_yes_flag(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from gitbrief.cli import cli

    hist_dir = tmp_path / "history"
    hist_dir.mkdir(parents=True)
    (hist_dir / "2026-03-01_100000.json").write_text(
        json.dumps(
            {
                "timestamp": "2026-03-01T10:00:00",
                "projects": ["p"],
                "since": "2026-02-01",
                "until": None,
                "backend": "claude",
                "commit_count": 1,
                "summary": "s",
            }
        )
    )

    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", tmp_path)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", hist_dir)

    runner = CliRunner()
    result = runner.invoke(cli, ["history", "clear", "--yes"])
    assert result.exit_code == 0
    assert "Deleted 1 history entry" in result.output
    assert list(hist_dir.glob("*.json")) == []


def test_history_clear_older_than(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from datetime import datetime, timedelta
    from gitbrief.cli import cli

    hist_dir = tmp_path / "history"
    hist_dir.mkdir(parents=True)
    now = datetime.now()
    old_ts = (now - timedelta(days=60)).isoformat(timespec="seconds")
    new_ts = (now - timedelta(days=1)).isoformat(timespec="seconds")
    old_stem = (now - timedelta(days=60)).strftime("%Y-%m-%d_%H%M%S")
    new_stem = (now - timedelta(days=1)).strftime("%Y-%m-%d_%H%M%S")

    for stem, ts in [(old_stem, old_ts), (new_stem, new_ts)]:
        (hist_dir / f"{stem}.json").write_text(
            json.dumps(
                {
                    "timestamp": ts,
                    "projects": ["p"],
                    "since": "2026-01-01",
                    "until": None,
                    "backend": "claude",
                    "commit_count": 1,
                    "summary": "s",
                }
            )
        )

    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", tmp_path)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", hist_dir)

    runner = CliRunner()
    result = runner.invoke(cli, ["history", "clear", "--older-than", "30d", "--yes"])
    assert result.exit_code == 0
    assert "Deleted 1" in result.output
    remaining = list(hist_dir.glob("*.json"))
    assert len(remaining) == 1


def test_history_diff_shows_both(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from gitbrief.cli import cli

    hist_dir = tmp_path / "history"
    hist_dir.mkdir(parents=True)
    for stem, ts, proj, txt in [
        ("2026-01-01_100000", "2026-01-01T10:00:00", "alpha", "old summary"),
        ("2026-03-01_100000", "2026-03-01T10:00:00", "beta", "new summary"),
    ]:
        (hist_dir / f"{stem}.json").write_text(
            json.dumps(
                {
                    "timestamp": ts,
                    "projects": [proj],
                    "since": "2025-12-01",
                    "until": None,
                    "backend": "claude",
                    "commit_count": 1,
                    "summary": txt,
                }
            )
        )

    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", tmp_path)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", hist_dir)

    runner = CliRunner()
    result = runner.invoke(cli, ["history", "diff", "1", "2"])
    assert result.exit_code == 0
    assert "new summary" in result.output
    assert "old summary" in result.output
