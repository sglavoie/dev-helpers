"""Tests for gitbrief.config module."""

import json

import pytest

import gitbrief.config as config_module
from gitbrief.config import (
    _normalize_projects,
    add_project,
    get_setting,
    get_setting_int,
    load_config,
    remove_project,
    save_config,
    set_setting,
)


@pytest.fixture(autouse=True)
def patch_config_path(tmp_path, monkeypatch):
    """Redirect all config paths to a temp directory for every test."""
    gitbrief_dir = tmp_path / ".gitbrief"
    gitbrief_dir.mkdir(exist_ok=True)
    new_cfg = gitbrief_dir / "config.json"
    old_cfg = tmp_path / ".gitbrief.json"
    monkeypatch.setattr(config_module, "GITBRIEF_DIR", gitbrief_dir)
    monkeypatch.setattr(config_module, "NEW_CONFIG_PATH", new_cfg)
    monkeypatch.setattr(config_module, "OLD_CONFIG_PATH", old_cfg)
    monkeypatch.setattr(config_module, "CONFIG_PATH", new_cfg)
    return new_cfg


# ---------------------------------------------------------------------------
# _normalize_projects
# ---------------------------------------------------------------------------


class TestNormalizeProjects:
    def test_string_value_legacy(self):
        result = _normalize_projects({"myrepo": "/some/path"})
        assert result == {"myrepo": {"path": "/some/path", "backend": None}}

    def test_dict_value_passthrough(self):
        entry = {"path": "/some/path", "backend": "claude"}
        result = _normalize_projects({"myrepo": entry})
        assert result == {"myrepo": entry}

    def test_empty_dict(self):
        assert _normalize_projects({}) == {}

    def test_mixed_values(self):
        result = _normalize_projects(
            {
                "a": "/path/a",
                "b": {"path": "/path/b", "backend": None},
            }
        )
        assert result["a"] == {"path": "/path/a", "backend": None}
        assert result["b"] == {"path": "/path/b", "backend": None}


# ---------------------------------------------------------------------------
# load_config / save_config
# ---------------------------------------------------------------------------


class TestLoadConfig:
    def test_returns_default_when_no_file(self):
        cfg = load_config()
        assert cfg["projects"] == {}
        assert cfg["settings"]["backend"] == "claude"

    def test_loads_existing_config(self, tmp_path):
        data = {
            "projects": {"repo": {"path": "/tmp/repo", "backend": None}},
            "settings": {
                "backend": "copilot",
                "timeout": 60,
                "retries": 1,
                "max_commits": 50,
            },
        }
        # autouse fixture already points NEW_CONFIG_PATH to tmp_path/.gitbrief/config.json
        config_module.NEW_CONFIG_PATH.write_text(json.dumps(data))
        cfg = load_config()
        assert cfg["settings"]["backend"] == "copilot"
        assert cfg["projects"]["repo"]["path"] == "/tmp/repo"

    def test_normalizes_legacy_projects(self, tmp_path):
        data = {"projects": {"old": "/legacy/path"}, "settings": {}}
        config_module.NEW_CONFIG_PATH.write_text(json.dumps(data))
        cfg = load_config()
        assert cfg["projects"]["old"] == {"path": "/legacy/path", "backend": None}


class TestSaveConfig:
    def test_writes_json(self):
        cfg = {"projects": {}, "settings": {"backend": "claude"}}
        save_config(cfg)
        loaded = json.loads(config_module.CONFIG_PATH.read_text())
        assert loaded["settings"]["backend"] == "claude"

    def test_trailing_newline(self):
        save_config({"projects": {}, "settings": {}})
        raw = config_module.CONFIG_PATH.read_text()
        assert raw.endswith("\n")


# ---------------------------------------------------------------------------
# add_project / remove_project
# ---------------------------------------------------------------------------


class TestAddProject:
    def test_adds_project(self, tmp_path):
        git_dir = tmp_path / "repo" / ".git"
        git_dir.mkdir(parents=True)
        add_project("myrepo", str(tmp_path / "repo"))
        cfg = load_config()
        assert "myrepo" in cfg["projects"]

    def test_raises_if_not_git_repo(self, tmp_path):
        import click

        with pytest.raises(click.ClickException, match="Not a git repository"):
            add_project("bad", str(tmp_path))

    def test_raises_if_alias_exists(self, tmp_path):
        import click

        git_dir = tmp_path / "repo" / ".git"
        git_dir.mkdir(parents=True)
        add_project("dup", str(tmp_path / "repo"))
        with pytest.raises(click.ClickException, match="already exists"):
            add_project("dup", str(tmp_path / "repo"))

    def test_raises_on_invalid_backend(self, tmp_path):
        import click

        git_dir = tmp_path / "repo" / ".git"
        git_dir.mkdir(parents=True)
        with pytest.raises(click.ClickException, match="Invalid backend"):
            add_project("myrepo", str(tmp_path / "repo"), backend="unknown")


class TestRemoveProject:
    def test_removes_existing_project(self, tmp_path):
        git_dir = tmp_path / "repo" / ".git"
        git_dir.mkdir(parents=True)
        add_project("todel", str(tmp_path / "repo"))
        remove_project("todel")
        cfg = load_config()
        assert "todel" not in cfg["projects"]

    def test_raises_if_not_found(self):
        import click

        with pytest.raises(click.ClickException, match="not found"):
            remove_project("ghost")


# ---------------------------------------------------------------------------
# set_setting
# ---------------------------------------------------------------------------


class TestSetSetting:
    def test_set_valid_backend(self):
        set_setting("backend", "copilot")
        assert get_setting("backend") == "copilot"

    def test_set_invalid_backend(self):
        import click

        with pytest.raises(click.ClickException, match="Invalid backend"):
            set_setting("backend", "gpt4")

    def test_set_timeout_valid(self):
        set_setting("timeout", "300")
        assert get_setting_int("timeout", 120) == 300

    def test_set_timeout_invalid_string(self):
        import click

        with pytest.raises(click.ClickException, match="positive integer"):
            set_setting("timeout", "abc")

    def test_set_timeout_zero(self):
        import click

        with pytest.raises(click.ClickException, match="positive integer"):
            set_setting("timeout", "0")

    def test_set_retries_valid(self):
        set_setting("retries", "3")
        assert get_setting_int("retries", 2) == 3

    def test_set_retries_too_high(self):
        import click

        with pytest.raises(click.ClickException, match="between 0 and 5"):
            set_setting("retries", "6")

    def test_set_retries_invalid_string(self):
        import click

        with pytest.raises(click.ClickException, match="between 0 and 5"):
            set_setting("retries", "nope")

    def test_set_max_commits_valid(self):
        set_setting("max_commits", "250")
        assert get_setting_int("max_commits", 100) == 250

    def test_set_max_commits_too_low(self):
        import click

        with pytest.raises(click.ClickException, match="between 10 and 1000"):
            set_setting("max_commits", "5")

    def test_set_max_commits_too_high(self):
        import click

        with pytest.raises(click.ClickException, match="between 10 and 1000"):
            set_setting("max_commits", "1001")

    def test_set_max_commits_invalid_string(self):
        import click

        with pytest.raises(click.ClickException, match="between 10 and 1000"):
            set_setting("max_commits", "lots")

    def test_set_max_commits_boundary_low(self):
        set_setting("max_commits", "10")
        assert get_setting_int("max_commits", 100) == 10

    def test_set_max_commits_boundary_high(self):
        set_setting("max_commits", "1000")
        assert get_setting_int("max_commits", 100) == 1000
