"""Tests for config-file migration and per-project last_summary tracking."""

import json
from pathlib import Path

import gitbrief.config as config_mod


def test_config_migration_old_to_new(tmp_path: Path, use_config_paths) -> None:
    """load_config() should migrate ~/.gitbrief.json -> ~/.gitbrief/config.json."""
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
    use_config_paths(old_path, new_path, new_dir)

    loaded = config_mod.load_config()

    # Old file should be gone, new file should exist
    assert not old_path.exists()
    assert new_path.exists()
    assert loaded["projects"]["myproj"]["path"] == "/some/path"
    assert loaded.get("last_summary") == {}


def test_config_new_path_takes_priority(tmp_path: Path, use_config_paths) -> None:
    """When both old and new config exist, new config is used (no migration)."""
    old_path = tmp_path / ".gitbrief.json"
    new_dir = tmp_path / ".gitbrief"
    new_dir.mkdir()
    new_path = new_dir / "config.json"

    old_path.write_text(
        json.dumps(
            {
                "projects": {"old": {"path": "/old", "backend": None}},
                "settings": {},
                "last_summary": {},
            }
        )
    )
    new_path.write_text(
        json.dumps(
            {
                "projects": {"new": {"path": "/new", "backend": None}},
                "settings": {},
                "last_summary": {},
            }
        )
    )
    use_config_paths(old_path, new_path, new_dir)

    loaded = config_mod.load_config()
    assert "new" in loaded["projects"]
    assert "old" not in loaded["projects"]
    assert old_path.exists()  # not deleted


def test_get_set_last_summary(tmp_path: Path, use_config_paths) -> None:
    new_dir = tmp_path / ".gitbrief"
    new_dir.mkdir()
    use_config_paths(tmp_path / ".gitbrief.json", new_dir / "config.json", new_dir)

    assert config_mod.get_last_summary("proj1") is None

    config_mod.set_last_summary("proj1", "2026-03-15T10:00:00")
    assert config_mod.get_last_summary("proj1") == "2026-03-15T10:00:00"

    # A second project is independent
    assert config_mod.get_last_summary("proj2") is None
    config_mod.set_last_summary("proj2", "2026-03-20T08:00:00")
    assert config_mod.get_last_summary("proj2") == "2026-03-20T08:00:00"
    assert config_mod.get_last_summary("proj1") == "2026-03-15T10:00:00"
