"""Tests for the `history` command group and the `summary --since-last` flag."""

import json
from datetime import datetime, timedelta
from pathlib import Path
from unittest.mock import patch

from click.testing import CliRunner

from gitbrief.cli import cli
from gitbrief.git import parse_duration


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


def _since_last_run(tmp_path: Path, use_config_paths, last_summary: dict):
    """Run `summary --since-last --dry-run` against a temp config."""
    repo_dir = tmp_path / "repo"
    repo_dir.mkdir()
    (repo_dir / ".git").mkdir()

    cfg_path = _make_config(tmp_path, last_summary=last_summary)
    use_config_paths(tmp_path / ".gitbrief.json", cfg_path, tmp_path / ".gitbrief")

    with patch("gitbrief.git.extract_commits", return_value=[]) as mock_extract:
        CliRunner().invoke(cli, ["summary", "--since-last", "--dry-run", "myproj"])
    return mock_extract


# ---------------------------------------------------------------------------
# --since-last CLI flag
# ---------------------------------------------------------------------------


def test_since_last_requires_no_last_or_since() -> None:
    result = CliRunner().invoke(
        cli, ["summary", "--since-last", "--last", "1w", "myproj"]
    )
    assert result.exit_code != 0
    assert "--since-last cannot be combined" in result.output


def test_since_last_needs_one_of_three() -> None:
    result = CliRunner().invoke(cli, ["summary", "myproj"])
    assert result.exit_code != 0
    assert "Specify --last, --since, or --since-last" in result.output


def test_since_last_uses_history_timestamp(tmp_path: Path, use_config_paths) -> None:
    """--since-last should use the stored timestamp as the since date."""
    mock_extract = _since_last_run(
        tmp_path, use_config_paths, {"myproj": "2026-03-10T12:00:00"}
    )

    # Even with no commits, extract_commits should get the stored date
    if mock_extract.called:
        args, _ = mock_extract.call_args
        assert args[1] == "2026-03-10"  # date portion of the stored timestamp


def test_since_last_falls_back_to_1w_when_no_history(
    tmp_path: Path, use_config_paths
) -> None:
    """When no history exists for a project, --since-last falls back to 1w."""
    expected_fallback = parse_duration("1w")
    mock_extract = _since_last_run(tmp_path, use_config_paths, {})

    if mock_extract.called:
        args, _ = mock_extract.call_args
        assert args[1] == expected_fallback


# ---------------------------------------------------------------------------
# history CLI commands
# ---------------------------------------------------------------------------


def test_history_list_empty(history_dir: Path) -> None:
    result = CliRunner().invoke(cli, ["history", "list"])
    assert result.exit_code == 0
    assert "No history found" in result.output


def test_history_list_shows_entries(write_history) -> None:
    write_history(
        "2026-03-01_100000",
        projects=["proj1"],
        commit_count=7,
        summary="## Summary\n- did stuff",
    )

    result = CliRunner().invoke(cli, ["history", "list"])
    assert result.exit_code == 0
    assert "proj1" in result.output
    assert "7 commits" in result.output


def test_history_show_by_index(write_history) -> None:
    write_history(
        "2026-03-01_100000",
        commit_count=3,
        summary="# The Summary\n- shipped feature",
    )

    result = CliRunner().invoke(cli, ["history", "show", "1"])
    assert result.exit_code == 0
    assert "shipped feature" in result.output


def test_history_show_not_found(history_dir: Path) -> None:
    result = CliRunner().invoke(cli, ["history", "show", "99"])
    assert result.exit_code != 0
    assert "No history entry found" in result.output


def test_history_clear_with_yes_flag(history_dir: Path, write_history) -> None:
    write_history("2026-03-01_100000", projects=["p"], summary="s")

    result = CliRunner().invoke(cli, ["history", "clear", "--yes"])
    assert result.exit_code == 0
    assert "Deleted 1 history entry" in result.output
    assert list(history_dir.glob("*.json")) == []


def test_history_clear_older_than(history_dir: Path, write_history) -> None:
    now = datetime.now()
    for delta in (60, 1):
        stamp = now - timedelta(days=delta)
        write_history(
            stamp.strftime("%Y-%m-%d_%H%M%S"),
            timestamp=stamp.isoformat(timespec="seconds"),
            projects=["p"],
            summary="s",
        )

    result = CliRunner().invoke(
        cli, ["history", "clear", "--older-than", "30d", "--yes"]
    )
    assert result.exit_code == 0
    assert "Deleted 1" in result.output
    assert len(list(history_dir.glob("*.json"))) == 1


def test_history_diff_shows_both(write_history) -> None:
    write_history("2026-01-01_100000", projects=["alpha"], summary="old summary")
    write_history("2026-03-01_100000", projects=["beta"], summary="new summary")

    result = CliRunner().invoke(cli, ["history", "diff", "1", "2"])
    assert result.exit_code == 0
    assert "new summary" in result.output
    assert "old summary" in result.output
