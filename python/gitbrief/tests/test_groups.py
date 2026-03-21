"""Tests for project groups — config helpers and CLI commands."""

import json
from pathlib import Path

import pytest
from click.testing import CliRunner

import gitbrief.config as config_mod
import gitbrief.history as history_mod
from gitbrief.cli import cli
from gitbrief.config import (
    add_to_group,
    create_group,
    delete_group,
    list_groups,
    load_config,
    remove_from_group,
    resolve_group,
    save_config,
)


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(autouse=True)
def patch_config_path(tmp_path, monkeypatch):
    """Redirect all config paths to a temp directory for every test."""
    gitbrief_dir = tmp_path / ".gitbrief"
    gitbrief_dir.mkdir(exist_ok=True)
    new_cfg = gitbrief_dir / "config.json"
    old_cfg = tmp_path / ".gitbrief.json"
    monkeypatch.setattr(config_mod, "GITBRIEF_DIR", gitbrief_dir)
    monkeypatch.setattr(config_mod, "NEW_CONFIG_PATH", new_cfg)
    monkeypatch.setattr(config_mod, "OLD_CONFIG_PATH", old_cfg)
    monkeypatch.setattr(config_mod, "CONFIG_PATH", new_cfg)
    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", gitbrief_dir)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", gitbrief_dir / "history")
    return new_cfg


@pytest.fixture
def config_with_projects(tmp_path):
    """Return a config dict with two registered projects."""
    project_a = tmp_path / "proj-a"
    project_b = tmp_path / "proj-b"
    project_a.mkdir()
    project_b.mkdir()
    config = {
        "projects": {
            "proj-a": {"path": str(project_a), "backend": None},
            "proj-b": {"path": str(project_b), "backend": None},
        },
        "groups": {},
        "settings": {},
        "last_summary": {},
    }
    save_config(config)
    return config


@pytest.fixture
def fake_git_repo(tmp_path) -> Path:
    """Create a minimal fake git repo (just a .git dir)."""
    repo = tmp_path / "repo"
    repo.mkdir()
    (repo / ".git").mkdir()
    return repo


# ---------------------------------------------------------------------------
# Unit tests: config helper functions
# ---------------------------------------------------------------------------


class TestCreateGroup:
    def test_creates_group(self, config_with_projects):
        config = load_config()
        create_group(config, "frontend", ["proj-a"])
        assert config["groups"]["frontend"] == ["proj-a"]

    def test_creates_group_with_multiple_aliases(self, config_with_projects):
        config = load_config()
        create_group(config, "all", ["proj-a", "proj-b"])
        assert config["groups"]["all"] == ["proj-a", "proj-b"]

    def test_raises_if_group_exists(self, config_with_projects):
        import click

        config = load_config()
        create_group(config, "frontend", ["proj-a"])
        with pytest.raises(click.ClickException, match="already exists"):
            create_group(config, "frontend", ["proj-b"])

    def test_raises_if_alias_unknown(self, config_with_projects):
        import click

        config = load_config()
        with pytest.raises(click.ClickException, match="Unknown project alias"):
            create_group(config, "bad", ["nonexistent"])

    def test_empty_aliases_list(self, config_with_projects):
        """Creating a group with no aliases is allowed (empty group)."""
        config = load_config()
        create_group(config, "empty", [])
        assert config["groups"]["empty"] == []


class TestDeleteGroup:
    def test_deletes_group(self, config_with_projects):
        config = load_config()
        create_group(config, "frontend", ["proj-a"])
        delete_group(config, "frontend")
        assert "frontend" not in config["groups"]

    def test_raises_if_not_found(self, config_with_projects):
        import click

        config = load_config()
        with pytest.raises(click.ClickException, match="not found"):
            delete_group(config, "ghost")


class TestAddToGroup:
    def test_adds_alias(self, config_with_projects):
        config = load_config()
        create_group(config, "fe", ["proj-a"])
        add_to_group(config, "fe", "proj-b")
        assert "proj-b" in config["groups"]["fe"]

    def test_raises_if_group_not_found(self, config_with_projects):
        import click

        config = load_config()
        with pytest.raises(click.ClickException, match="not found"):
            add_to_group(config, "ghost", "proj-a")

    def test_raises_if_alias_unknown(self, config_with_projects):
        import click

        config = load_config()
        create_group(config, "fe", [])
        with pytest.raises(click.ClickException, match="Unknown project alias"):
            add_to_group(config, "fe", "nope")

    def test_raises_on_duplicate(self, config_with_projects):
        import click

        config = load_config()
        create_group(config, "fe", ["proj-a"])
        with pytest.raises(click.ClickException, match="already in group"):
            add_to_group(config, "fe", "proj-a")


class TestRemoveFromGroup:
    def test_removes_alias(self, config_with_projects):
        config = load_config()
        create_group(config, "fe", ["proj-a", "proj-b"])
        remove_from_group(config, "fe", "proj-a")
        assert config["groups"]["fe"] == ["proj-b"]

    def test_raises_if_group_not_found(self, config_with_projects):
        import click

        config = load_config()
        with pytest.raises(click.ClickException, match="not found"):
            remove_from_group(config, "ghost", "proj-a")

    def test_raises_if_alias_not_in_group(self, config_with_projects):
        import click

        config = load_config()
        create_group(config, "fe", ["proj-a"])
        with pytest.raises(click.ClickException, match="not in group"):
            remove_from_group(config, "fe", "proj-b")


class TestResolveGroup:
    def test_returns_aliases(self, config_with_projects):
        config = load_config()
        create_group(config, "fe", ["proj-a", "proj-b"])
        assert resolve_group(config, "fe") == ["proj-a", "proj-b"]

    def test_returns_empty_list(self, config_with_projects):
        config = load_config()
        create_group(config, "fe", [])
        assert resolve_group(config, "fe") == []

    def test_raises_if_not_found(self, config_with_projects):
        import click

        config = load_config()
        with pytest.raises(click.ClickException, match="not found"):
            resolve_group(config, "ghost")


class TestListGroups:
    def test_returns_empty_dict(self, config_with_projects):
        config = load_config()
        assert list_groups(config) == {}

    def test_returns_groups(self, config_with_projects):
        config = load_config()
        create_group(config, "fe", ["proj-a"])
        create_group(config, "be", ["proj-b"])
        groups = list_groups(config)
        assert groups == {"fe": ["proj-a"], "be": ["proj-b"]}


class TestLoadConfigInitializesGroups:
    def test_groups_initialized_if_missing(self, tmp_path):
        """load_config() should default 'groups' to {} if key is absent."""
        config = {
            "projects": {},
            "settings": {},
            "last_summary": {},
        }
        save_config(config)
        loaded = load_config()
        assert "groups" in loaded
        assert loaded["groups"] == {}


# ---------------------------------------------------------------------------
# Unit tests: @group expansion logic
# ---------------------------------------------------------------------------


class TestGroupExpansion:
    """Test @group resolution in summary via dry-run."""

    @pytest.fixture
    def runner(self):
        return CliRunner()

    @pytest.fixture
    def two_repo_config(self, tmp_path):
        """Write a config with two fake repos and one group."""
        repo_a = tmp_path / "repo-a"
        repo_b = tmp_path / "repo-b"
        repo_a.mkdir()
        repo_b.mkdir()
        (repo_a / ".git").mkdir()
        (repo_b / ".git").mkdir()
        config = {
            "projects": {
                "proj-a": {"path": str(repo_a), "backend": None},
                "proj-b": {"path": str(repo_b), "backend": None},
            },
            "groups": {"fe": ["proj-a"], "all": ["proj-a", "proj-b"]},
            "settings": {"backend": "claude", "timeout": 120, "retries": 2, "max_commits": 100},
            "last_summary": {},
        }
        save_config(config)
        return config

    def test_at_all_expands(self, runner, two_repo_config):
        result = runner.invoke(cli, ["summary", "--last", "1w", "--raw", "@all"])
        # No error about unknown project
        assert "Unknown group" not in (result.output or "")
        assert result.exit_code == 0 or "No activity" in result.output

    def test_at_group_expands(self, runner, two_repo_config):
        result = runner.invoke(cli, ["summary", "--last", "1w", "--raw", "@fe"])
        assert "Unknown group" not in (result.output or "")

    def test_unknown_group_error(self, runner, two_repo_config):
        result = runner.invoke(cli, ["summary", "--last", "1w", "--raw", "@ghost"])
        assert result.exit_code != 0
        assert "Unknown group" in result.output

    def test_deduplication(self, runner, two_repo_config):
        """@all and proj-a together should not duplicate proj-a."""
        result = runner.invoke(
            cli, ["summary", "--last", "1w", "--raw", "@all", "proj-a"]
        )
        # Should not crash or error
        assert "Traceback" not in (result.output or "")
        assert "Unknown" not in (result.output or "")
        # proj-a header should appear at most once in raw output
        assert (result.output or "").count("=== proj-a ===") <= 1

    def test_regular_alias_passthrough(self, runner, two_repo_config):
        result = runner.invoke(
            cli, ["summary", "--last", "1w", "--raw", "proj-a"]
        )
        assert "Unknown project" not in (result.output or "")


# ---------------------------------------------------------------------------
# CLI integration tests: group commands
# ---------------------------------------------------------------------------


class TestGroupCLI:
    @pytest.fixture
    def runner(self):
        return CliRunner()

    @pytest.fixture
    def config_with_two_projects(self, tmp_path):
        repo_a = tmp_path / "repo-a"
        repo_b = tmp_path / "repo-b"
        repo_a.mkdir()
        repo_b.mkdir()
        (repo_a / ".git").mkdir()
        (repo_b / ".git").mkdir()
        config = {
            "projects": {
                "proj-a": {"path": str(repo_a), "backend": None},
                "proj-b": {"path": str(repo_b), "backend": None},
            },
            "groups": {},
            "settings": {},
            "last_summary": {},
        }
        save_config(config)
        return config

    def test_group_create(self, runner, config_with_two_projects):
        result = runner.invoke(cli, ["group", "create", "fe", "proj-a"])
        assert result.exit_code == 0, result.output
        assert "fe" in result.output
        config = load_config()
        assert config["groups"]["fe"] == ["proj-a"]

    def test_group_create_multiple_aliases(self, runner, config_with_two_projects):
        result = runner.invoke(cli, ["group", "create", "all", "proj-a", "proj-b"])
        assert result.exit_code == 0, result.output
        config = load_config()
        assert set(config["groups"]["all"]) == {"proj-a", "proj-b"}

    def test_group_create_duplicate_error(self, runner, config_with_two_projects):
        runner.invoke(cli, ["group", "create", "fe", "proj-a"])
        result = runner.invoke(cli, ["group", "create", "fe", "proj-b"])
        assert result.exit_code != 0
        assert "already exists" in result.output

    def test_group_create_unknown_alias_error(self, runner, config_with_two_projects):
        result = runner.invoke(cli, ["group", "create", "fe", "nope"])
        assert result.exit_code != 0
        assert "Unknown project alias" in result.output

    def test_group_delete(self, runner, config_with_two_projects):
        runner.invoke(cli, ["group", "create", "fe", "proj-a"])
        result = runner.invoke(cli, ["group", "delete", "fe"])
        assert result.exit_code == 0, result.output
        config = load_config()
        assert "fe" not in config["groups"]

    def test_group_delete_not_found(self, runner, config_with_two_projects):
        result = runner.invoke(cli, ["group", "delete", "ghost"])
        assert result.exit_code != 0
        assert "not found" in result.output

    def test_group_add(self, runner, config_with_two_projects):
        runner.invoke(cli, ["group", "create", "fe", "proj-a"])
        result = runner.invoke(cli, ["group", "add", "fe", "proj-b"])
        assert result.exit_code == 0, result.output
        config = load_config()
        assert "proj-b" in config["groups"]["fe"]

    def test_group_add_duplicate_error(self, runner, config_with_two_projects):
        runner.invoke(cli, ["group", "create", "fe", "proj-a"])
        result = runner.invoke(cli, ["group", "add", "fe", "proj-a"])
        assert result.exit_code != 0
        assert "already in group" in result.output

    def test_group_remove(self, runner, config_with_two_projects):
        runner.invoke(cli, ["group", "create", "fe", "proj-a", "proj-b"])
        result = runner.invoke(cli, ["group", "remove", "fe", "proj-a"])
        assert result.exit_code == 0, result.output
        config = load_config()
        assert "proj-a" not in config["groups"]["fe"]

    def test_group_remove_not_in_group(self, runner, config_with_two_projects):
        runner.invoke(cli, ["group", "create", "fe", "proj-a"])
        result = runner.invoke(cli, ["group", "remove", "fe", "proj-b"])
        assert result.exit_code != 0
        assert "not in group" in result.output

    def test_group_list_empty(self, runner, config_with_two_projects):
        result = runner.invoke(cli, ["group", "list"])
        assert result.exit_code == 0, result.output
        assert "No groups" in result.output

    def test_group_list_shows_groups(self, runner, config_with_two_projects):
        runner.invoke(cli, ["group", "create", "fe", "proj-a"])
        runner.invoke(cli, ["group", "create", "be", "proj-b"])
        result = runner.invoke(cli, ["group", "list"])
        assert result.exit_code == 0, result.output
        assert "fe" in result.output
        assert "be" in result.output

    def test_group_list_shows_members(self, runner, config_with_two_projects):
        runner.invoke(cli, ["group", "create", "fe", "proj-a", "proj-b"])
        result = runner.invoke(cli, ["group", "list"])
        assert "proj-a" in result.output
        assert "proj-b" in result.output

    def test_group_list_empty_group(self, runner, config_with_two_projects):
        """An empty group shows '(empty)' placeholder."""
        config = load_config()
        config["groups"]["empty-group"] = []
        save_config(config)
        result = runner.invoke(cli, ["group", "list"])
        assert result.exit_code == 0
        assert "(empty)" in result.output


# ---------------------------------------------------------------------------
# Doctor command: group health check
# ---------------------------------------------------------------------------


class TestDoctorGroupHealth:
    @pytest.fixture
    def runner(self):
        return CliRunner()

    def test_doctor_warns_stale_group(self, runner, tmp_path):
        repo_a = tmp_path / "repo-a"
        repo_a.mkdir()
        (repo_a / ".git").mkdir()
        config = {
            "projects": {"proj-a": {"path": str(repo_a), "backend": None}},
            "groups": {"fe": ["proj-a", "deleted-proj"]},
            "settings": {"backend": "claude", "timeout": 120, "retries": 2, "max_commits": 100},
            "last_summary": {},
        }
        save_config(config)
        result = runner.invoke(cli, ["doctor"])
        assert result.exit_code == 0
        assert "WARN" in result.output
        assert "deleted-proj" in result.output

    def test_doctor_ok_healthy_group(self, runner, tmp_path):
        repo_a = tmp_path / "repo-a"
        repo_a.mkdir()
        (repo_a / ".git").mkdir()
        config = {
            "projects": {"proj-a": {"path": str(repo_a), "backend": None}},
            "groups": {"fe": ["proj-a"]},
            "settings": {"backend": "claude", "timeout": 120, "retries": 2, "max_commits": 100},
            "last_summary": {},
        }
        save_config(config)
        result = runner.invoke(cli, ["doctor"])
        assert result.exit_code == 0
        assert "Group: fe" in result.output
