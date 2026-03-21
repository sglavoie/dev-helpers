"""Tests for Session 4: project discovery (discover_repos + scan command)."""

import json
import subprocess
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest
from click.testing import CliRunner

import gitbrief.config as config_mod
import gitbrief.history as history_mod
from gitbrief.cli import cli
from gitbrief.git import discover_repos


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def isolated_config(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Redirect config and history paths to a tmp directory."""
    gitbrief_dir = tmp_path / ".gitbrief"
    gitbrief_dir.mkdir()
    cfg_path = gitbrief_dir / "config.json"
    old_path = tmp_path / ".gitbrief.json"

    monkeypatch.setattr(config_mod, "OLD_CONFIG_PATH", old_path)
    monkeypatch.setattr(config_mod, "NEW_CONFIG_PATH", cfg_path)
    monkeypatch.setattr(config_mod, "GITBRIEF_DIR", gitbrief_dir)
    monkeypatch.setattr(config_mod, "CONFIG_PATH", cfg_path)
    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", gitbrief_dir)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", gitbrief_dir / "history")

    return cfg_path


def _make_git_repo(path: Path) -> None:
    """Create a minimal fake git repo (just a .git dir, no real commits)."""
    path.mkdir(parents=True, exist_ok=True)
    (path / ".git").mkdir()


def _make_real_git_repo(path: Path) -> None:
    """Create a real git repo with one commit so last_commit is populated."""
    path.mkdir(parents=True, exist_ok=True)
    subprocess.run(["git", "init", str(path)], check=True, capture_output=True)
    subprocess.run(
        ["git", "-C", str(path), "config", "user.email", "test@example.com"],
        check=True,
        capture_output=True,
    )
    subprocess.run(
        ["git", "-C", str(path), "config", "user.name", "Test"],
        check=True,
        capture_output=True,
    )
    subprocess.run(
        ["git", "-C", str(path), "config", "commit.gpgsign", "false"],
        check=True,
        capture_output=True,
    )
    readme = path / "README.md"
    readme.write_text("hello")
    subprocess.run(
        ["git", "-C", str(path), "add", "README.md"], check=True, capture_output=True
    )
    subprocess.run(
        ["git", "-C", str(path), "commit", "-m", "init"],
        check=True,
        capture_output=True,
    )


# ---------------------------------------------------------------------------
# discover_repos unit tests
# ---------------------------------------------------------------------------


class TestDiscoverRepos:
    def test_finds_single_git_repo(self, tmp_path: Path) -> None:
        _make_git_repo(tmp_path / "myrepo")
        repos = discover_repos(str(tmp_path))
        assert len(repos) == 1
        assert repos[0]["name"] == "myrepo"
        assert repos[0]["path"] == str(tmp_path / "myrepo")

    def test_finds_multiple_repos(self, tmp_path: Path) -> None:
        for name in ("alpha", "beta", "gamma"):
            _make_git_repo(tmp_path / name)
        repos = discover_repos(str(tmp_path))
        names = {r["name"] for r in repos}
        assert names == {"alpha", "beta", "gamma"}

    def test_returns_empty_when_no_repos(self, tmp_path: Path) -> None:
        (tmp_path / "not-a-repo").mkdir()
        repos = discover_repos(str(tmp_path))
        assert repos == []

    def test_depth_limiting(self, tmp_path: Path) -> None:
        # repo at depth 2 — reachable with max_depth=2
        deep = tmp_path / "level1" / "level2" / "repo"
        _make_git_repo(deep)
        # depth=1: only direct children of tmp_path are scanned -> nothing found
        assert discover_repos(str(tmp_path), max_depth=1) == []
        # depth=2: level1/level2 is at depth 2 from tmp_path -> found
        repos = discover_repos(str(tmp_path), max_depth=2)
        assert len(repos) == 0  # level1/level2/repo is at depth 3 from tmp_path

    def test_depth_limiting_exact_depth(self, tmp_path: Path) -> None:
        # repo directly inside level1 (depth 2)
        repo = tmp_path / "level1" / "repo"
        _make_git_repo(repo)
        assert discover_repos(str(tmp_path), max_depth=1) == []
        repos = discover_repos(str(tmp_path), max_depth=2)
        assert len(repos) == 1
        assert repos[0]["name"] == "repo"

    def test_skips_hidden_directories(self, tmp_path: Path) -> None:
        _make_git_repo(tmp_path / ".hidden_repo")
        repos = discover_repos(str(tmp_path))
        assert repos == []

    def test_skips_node_modules(self, tmp_path: Path) -> None:
        _make_git_repo(tmp_path / "node_modules" / "some-pkg")
        repos = discover_repos(str(tmp_path))
        assert repos == []

    def test_skips_venv(self, tmp_path: Path) -> None:
        _make_git_repo(tmp_path / "venv" / "sub")
        _make_git_repo(tmp_path / ".venv" / "sub")
        repos = discover_repos(str(tmp_path))
        assert repos == []

    def test_skips_pycache(self, tmp_path: Path) -> None:
        _make_git_repo(tmp_path / "__pycache__" / "cached")
        repos = discover_repos(str(tmp_path))
        assert repos == []

    def test_result_has_required_keys(self, tmp_path: Path) -> None:
        _make_git_repo(tmp_path / "repo")
        repos = discover_repos(str(tmp_path))
        assert set(repos[0].keys()) == {"path", "name", "last_commit"}

    def test_path_is_absolute(self, tmp_path: Path) -> None:
        _make_git_repo(tmp_path / "repo")
        repos = discover_repos(str(tmp_path))
        assert Path(repos[0]["path"]).is_absolute()

    def test_does_not_recurse_into_git_repos(self, tmp_path: Path) -> None:
        """Repos nested inside other repos should not be discovered."""
        outer = tmp_path / "outer"
        _make_git_repo(outer)
        inner = outer / "inner"
        _make_git_repo(inner)
        repos = discover_repos(str(tmp_path))
        names = [r["name"] for r in repos]
        assert "outer" in names
        assert "inner" not in names

    def test_sorts_by_last_commit_most_recent_first(self, tmp_path: Path) -> None:
        """Repos with later last_commit dates appear first."""
        old_repo = tmp_path / "old_project"
        new_repo = tmp_path / "new_project"
        _make_git_repo(old_repo)
        _make_git_repo(new_repo)

        def mock_last_commit(repo_path: str) -> str:
            if "old" in repo_path:
                return "2024-01-01"
            return "2026-03-20"

        with patch("gitbrief.git._get_last_commit_date", side_effect=mock_last_commit):
            repos = discover_repos(str(tmp_path))

        assert repos[0]["name"] == "new_project"
        assert repos[1]["name"] == "old_project"

    def test_last_commit_empty_on_failure(self, tmp_path: Path) -> None:
        _make_git_repo(tmp_path / "repo")
        with patch("gitbrief.git._get_last_commit_date", return_value=""):
            repos = discover_repos(str(tmp_path))
        assert repos[0]["last_commit"] == ""

    def test_real_repo_has_last_commit_date(self, tmp_path: Path) -> None:
        repo_path = tmp_path / "real_repo"
        _make_real_git_repo(repo_path)
        repos = discover_repos(str(tmp_path))
        assert len(repos) == 1
        # Should be a valid ISO date
        assert len(repos[0]["last_commit"]) == 10
        assert repos[0]["last_commit"][4] == "-"


# ---------------------------------------------------------------------------
# _get_last_commit_date unit tests
# ---------------------------------------------------------------------------


class TestGetLastCommitDate:
    def test_returns_date_for_real_repo(self, tmp_path: Path) -> None:
        from gitbrief.git import _get_last_commit_date

        repo_path = tmp_path / "repo"
        _make_real_git_repo(repo_path)
        date = _get_last_commit_date(str(repo_path))
        assert len(date) == 10
        assert date[4] == "-"

    def test_returns_empty_on_failure(self) -> None:
        from gitbrief.git import _get_last_commit_date

        result = _get_last_commit_date("/nonexistent/path")
        assert result == ""

    def test_returns_empty_when_git_missing(self) -> None:
        from gitbrief.git import _get_last_commit_date

        with patch("gitbrief.git.subprocess.run", side_effect=FileNotFoundError):
            result = _get_last_commit_date("/some/repo")
        assert result == ""

    def test_returns_empty_on_nonzero_exit(self) -> None:
        from gitbrief.git import _get_last_commit_date

        mock_result = MagicMock()
        mock_result.returncode = 1
        mock_result.stdout = ""
        with patch("gitbrief.git.subprocess.run", return_value=mock_result):
            result = _get_last_commit_date("/some/repo")
        assert result == ""


# ---------------------------------------------------------------------------
# _resolve_alias unit tests
# ---------------------------------------------------------------------------


class TestResolveAlias:
    def test_returns_name_when_free(self) -> None:
        from gitbrief.cli import _resolve_alias

        assert _resolve_alias("myproject", {}) == "myproject"

    def test_appends_2_when_taken(self) -> None:
        from gitbrief.cli import _resolve_alias

        existing = {"myproject": {}}
        assert _resolve_alias("myproject", existing) == "myproject-2"

    def test_increments_further_when_needed(self) -> None:
        from gitbrief.cli import _resolve_alias

        existing = {"myproject": {}, "myproject-2": {}, "myproject-3": {}}
        assert _resolve_alias("myproject", existing) == "myproject-4"


# ---------------------------------------------------------------------------
# scan CLI integration tests
# ---------------------------------------------------------------------------


class TestScanCommand:
    def test_scan_no_repos_found(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        runner = CliRunner()
        scan_dir = tmp_path / "empty"
        scan_dir.mkdir()
        result = runner.invoke(cli, ["scan", str(scan_dir)])
        assert result.exit_code == 0, result.output
        assert "No git repositories found" in result.output

    def test_scan_auto_registers_new_repos(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        repo_a = scan_dir / "alpha"
        repo_b = scan_dir / "beta"
        _make_git_repo(repo_a)
        _make_git_repo(repo_b)

        runner = CliRunner()
        with patch("gitbrief.git._get_last_commit_date", return_value="2026-03-20"):
            result = runner.invoke(cli, ["scan", str(scan_dir), "--auto"])

        assert result.exit_code == 0, result.output
        assert "alpha" in result.output
        assert "beta" in result.output
        assert "Added 2 new repositories" in result.output

        # Verify saved to config
        config = json.loads(isolated_config.read_text())
        registered_paths = {p["path"] for p in config["projects"].values()}
        assert str(repo_a) in registered_paths
        assert str(repo_b) in registered_paths

    def test_scan_auto_skips_already_registered(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        repo_a = scan_dir / "alpha"
        _make_git_repo(repo_a)

        # Pre-register repo_a
        config = {"projects": {"alpha": {"path": str(repo_a), "backend": None}}, "groups": {}, "settings": {}, "last_summary": {}}
        isolated_config.write_text(json.dumps(config))

        runner = CliRunner()
        with patch("gitbrief.git._get_last_commit_date", return_value="2026-03-20"):
            result = runner.invoke(cli, ["scan", str(scan_dir), "--auto"])

        assert result.exit_code == 0, result.output
        assert "All discovered repositories are already registered" in result.output

    def test_scan_interactive_yes_adds_all(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        repo_a = scan_dir / "repo-a"
        _make_git_repo(repo_a)

        runner = CliRunner()
        with patch("gitbrief.git._get_last_commit_date", return_value="2026-03-20"):
            result = runner.invoke(cli, ["scan", str(scan_dir)], input="y\n")

        assert result.exit_code == 0, result.output
        assert "Added 1 new repository" in result.output

        config = json.loads(isolated_config.read_text())
        assert str(repo_a) in {p["path"] for p in config["projects"].values()}

    def test_scan_interactive_no_adds_nothing(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        _make_git_repo(scan_dir / "repo-a")

        # Write an initial empty config so we can verify it's unchanged
        initial = {"projects": {}, "groups": {}, "settings": {}, "last_summary": {}}
        isolated_config.write_text(json.dumps(initial))

        runner = CliRunner()
        with patch("gitbrief.git._get_last_commit_date", return_value="2026-03-20"):
            result = runner.invoke(cli, ["scan", str(scan_dir)], input="N\n")

        assert result.exit_code == 0, result.output
        assert "No repositories added" in result.output

        config = json.loads(isolated_config.read_text())
        assert config["projects"] == {}

    def test_scan_interactive_select_specific_repos(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        repo_a = scan_dir / "aaa"
        repo_b = scan_dir / "bbb"
        _make_git_repo(repo_a)
        _make_git_repo(repo_b)

        runner = CliRunner()
        # Select only the first repo
        with patch("gitbrief.git._get_last_commit_date", return_value=""):
            result = runner.invoke(cli, ["scan", str(scan_dir)], input="select\n1\n")

        assert result.exit_code == 0, result.output
        assert "Added 1 new repository" in result.output

        config = json.loads(isolated_config.read_text())
        assert len(config["projects"]) == 1

    def test_scan_interactive_select_empty_input(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        _make_git_repo(scan_dir / "repo-a")

        runner = CliRunner()
        with patch("gitbrief.git._get_last_commit_date", return_value=""):
            result = runner.invoke(cli, ["scan", str(scan_dir)], input="select\n\n")

        assert result.exit_code == 0, result.output
        assert "No repositories selected" in result.output

    def test_scan_alias_conflict_resolved_with_suffix(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        repo = scan_dir / "myapp"
        _make_git_repo(repo)

        # Pre-register another project with the same alias "myapp"
        config = {"projects": {"myapp": {"path": "/some/other/path", "backend": None}}, "groups": {}, "settings": {}, "last_summary": {}}
        isolated_config.write_text(json.dumps(config))

        runner = CliRunner()
        with patch("gitbrief.git._get_last_commit_date", return_value="2026-03-20"):
            result = runner.invoke(cli, ["scan", str(scan_dir), "--auto"])

        assert result.exit_code == 0, result.output
        assert "myapp-2" in result.output

        config = json.loads(isolated_config.read_text())
        assert "myapp-2" in config["projects"]

    def test_scan_respects_depth_option(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        deep_repo = scan_dir / "level1" / "level2" / "repo"
        _make_git_repo(deep_repo)

        runner = CliRunner()
        # With depth=1, shouldn't find the deep repo
        result = runner.invoke(cli, ["scan", str(scan_dir), "--depth", "1"])
        assert result.exit_code == 0, result.output
        assert "No git repositories found" in result.output

        # With depth=3, should find it (but not at depth=2 since it's 3 levels deep)
        with patch("gitbrief.git._get_last_commit_date", return_value=""):
            result = runner.invoke(
                cli, ["scan", str(scan_dir), "--depth", "3", "--auto"]
            )
        assert result.exit_code == 0, result.output
        assert "Added" in result.output

    def test_scan_shows_registered_status_in_table(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        repo_a = scan_dir / "alpha"
        _make_git_repo(repo_a)

        # Pre-register it
        config = {"projects": {"alpha": {"path": str(repo_a), "backend": None}}, "groups": {}, "settings": {}, "last_summary": {}}
        isolated_config.write_text(json.dumps(config))

        runner = CliRunner()
        with patch("gitbrief.git._get_last_commit_date", return_value=""):
            result = runner.invoke(cli, ["scan", str(scan_dir)])

        assert result.exit_code == 0, result.output
        assert "registered" in result.output

    def test_scan_select_invalid_index_skipped(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        scan_dir = tmp_path / "projects"
        _make_git_repo(scan_dir / "repo-a")

        runner = CliRunner()
        # Index 99 is out of range; should warn and add nothing
        with patch("gitbrief.git._get_last_commit_date", return_value=""):
            result = runner.invoke(cli, ["scan", str(scan_dir)], input="select\n99\n")

        assert result.exit_code == 0, result.output
        assert "No valid repositories selected" in result.output
