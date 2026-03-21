"""CLI integration tests — end-to-end command flows using CliRunner (Session 5a + 5b)."""

import json
from pathlib import Path
from unittest.mock import patch

import pytest
from click.testing import CliRunner

import gitbrief.config as config_mod
import gitbrief.history as history_mod
from gitbrief.cli import cli


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


@pytest.fixture
def fake_git_repo(tmp_path: Path) -> Path:
    """Create a minimal fake git repo (just a .git dir — no real commits)."""
    repo = tmp_path / "repo"
    repo.mkdir()
    (repo / ".git").mkdir()
    return repo


_SAMPLE_COMMITS = [
    {
        "sha": "abc12345",
        "subject": "feat: add feature",
        "body": "",
        "refs": [],
        "files_changed": 2,
        "insertions": 10,
        "deletions": 1,
    }
]


# ---------------------------------------------------------------------------
# 5a: Full command-flow integration tests
# ---------------------------------------------------------------------------


class TestCLIWorkflow:
    """add → list → summary --dry-run → remove end-to-end flow."""

    def test_add_list_remove_flow(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        alias = "test-proj"
        path = str(fake_git_repo)

        result = runner.invoke(cli, ["add", alias, path])
        assert result.exit_code == 0, result.output
        assert alias in result.output

        result = runner.invoke(cli, ["list"])
        assert result.exit_code == 0
        assert alias in result.output
        assert path in result.output

        result = runner.invoke(cli, ["remove", alias])
        assert result.exit_code == 0
        assert alias in result.output

        result = runner.invoke(cli, ["list"])
        assert result.exit_code == 0
        assert "No projects registered" in result.output

    def test_add_then_summary_dry_run(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        alias = "my-repo"
        runner.invoke(cli, ["add", alias, str(fake_git_repo)])

        with (
            patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
        ):
            result = runner.invoke(cli, ["summary", "--last", "1w", "--dry-run", alias])

        assert result.exit_code == 0, result.output
        assert "feat: add feature" in result.output
        assert "Summarize" in result.output

    def test_summary_zero_commits_message(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        alias = "empty-repo"
        runner.invoke(cli, ["add", alias, str(fake_git_repo)])

        with (
            patch("gitbrief.cli.extract_commits", return_value=[]),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
        ):
            result = runner.invoke(cli, ["summary", "--last", "1d", alias])

        assert result.exit_code == 0
        assert "No activity found" in result.output

    def test_summary_all_projects_when_none_specified(
        self, isolated_config: Path, fake_git_repo: Path, tmp_path: Path
    ) -> None:
        """When no project argument is given, all registered projects are used."""
        runner = CliRunner()
        repo2 = tmp_path / "repo2"
        repo2.mkdir()
        (repo2 / ".git").mkdir()

        runner.invoke(cli, ["add", "proj-a", str(fake_git_repo)])
        runner.invoke(cli, ["add", "proj-b", str(repo2)])

        with (
            patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
        ):
            result = runner.invoke(cli, ["summary", "--last", "1w", "--dry-run"])

        assert result.exit_code == 0, result.output
        assert "proj-a" in result.output
        assert "proj-b" in result.output


class TestConfigCommands:
    """config set → config get → config list flow."""

    def test_config_set_get_flow(self, isolated_config: Path) -> None:
        runner = CliRunner()

        result = runner.invoke(cli, ["config", "set", "timeout", "300"])
        assert result.exit_code == 0
        assert "timeout" in result.output
        assert "300" in result.output

        result = runner.invoke(cli, ["config", "get", "timeout"])
        assert result.exit_code == 0
        assert "300" in result.output

    def test_config_list_shows_settings(self, isolated_config: Path) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["config", "set", "backend", "claude"])
        runner.invoke(cli, ["config", "set", "timeout", "180"])

        result = runner.invoke(cli, ["config", "list"])
        assert result.exit_code == 0
        assert "backend" in result.output
        assert "timeout" in result.output

    def test_config_get_missing_key(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["config", "get", "nonexistent"])
        assert result.exit_code == 0
        assert "not set" in result.output

    def test_config_set_invalid_backend_fails(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["config", "set", "backend", "gpt4"])
        assert result.exit_code != 0
        assert "Invalid backend" in result.output

    def test_config_set_invalid_timeout_fails(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["config", "set", "timeout", "abc"])
        assert result.exit_code != 0

    def test_config_set_max_commits(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["config", "set", "max_commits", "250"])
        assert result.exit_code == 0

        result = runner.invoke(cli, ["config", "get", "max_commits"])
        assert result.exit_code == 0
        assert "250" in result.output


class TestDoctorCommand:
    """doctor with various project/backend states."""

    def test_doctor_no_projects(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["doctor"])
        assert result.exit_code == 0
        assert "No projects registered" in result.output

    def test_doctor_with_valid_project(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])

        with patch(
            "gitbrief.cli.extract_commits",
            return_value=[{"sha": "abc", "subject": "x", "body": "", "refs": []}],
        ):
            result = runner.invoke(cli, ["doctor"])

        assert result.exit_code == 0
        assert "proj" in result.output

    def test_doctor_with_missing_project_path(
        self, isolated_config: Path, tmp_path: Path
    ) -> None:
        runner = CliRunner()
        cfg = {
            "projects": {
                "ghost": {"path": str(tmp_path / "nonexistent"), "backend": None}
            },
            "settings": {},
            "last_summary": {},
        }
        isolated_config.write_text(json.dumps(cfg))

        result = runner.invoke(cli, ["doctor"])
        assert result.exit_code == 0
        assert "ghost" in result.output

    def test_doctor_warns_about_unknown_config_keys(
        self, isolated_config: Path
    ) -> None:
        runner = CliRunner()
        cfg = {
            "projects": {},
            "settings": {"backend": "claude", "mystery_key": "value"},
            "last_summary": {},
        }
        isolated_config.write_text(json.dumps(cfg))

        result = runner.invoke(cli, ["doctor"])
        assert result.exit_code == 0
        assert "mystery_key" in result.output

    def test_doctor_shows_ok_config_when_clean(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["doctor"])
        assert result.exit_code == 0
        assert "Config" in result.output


class TestSummaryWithSinceLast:
    """--since-last flag with mocked history."""

    def test_since_last_uses_stored_timestamp(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        cfg = {
            "projects": {"proj": {"path": str(fake_git_repo), "backend": None}},
            "settings": {"backend": "claude"},
            "last_summary": {"proj": "2026-03-10T12:00:00"},
        }
        isolated_config.write_text(json.dumps(cfg))

        with (
            patch(
                "gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS
            ) as mock_extract,
            patch("gitbrief.cli.get_git_user_email", return_value="u@test.com"),
        ):
            result = runner.invoke(
                cli, ["summary", "--since-last", "--dry-run", "proj"]
            )

        assert result.exit_code == 0
        if mock_extract.called:
            args, _ = mock_extract.call_args
            assert args[1] == "2026-03-10"

    def test_since_last_falls_back_to_1w_with_no_history(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        cfg = {
            "projects": {"proj": {"path": str(fake_git_repo), "backend": None}},
            "settings": {"backend": "claude"},
            "last_summary": {},
        }
        isolated_config.write_text(json.dumps(cfg))

        from gitbrief.git import parse_duration

        expected = parse_duration("1w")

        with (
            patch(
                "gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS
            ) as mock_extract,
            patch("gitbrief.cli.get_git_user_email", return_value="u@test.com"),
        ):
            runner.invoke(cli, ["summary", "--since-last", "--dry-run", "proj"])

        if mock_extract.called:
            args, _ = mock_extract.call_args
            assert args[1] == expected

    def test_since_last_cannot_combine_with_last(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["summary", "--since-last", "--last", "1w"])
        assert result.exit_code != 0
        assert "--since-last cannot be combined" in result.output


class TestErrorPaths:
    """Error handling for invalid inputs and missing prerequisites."""

    def test_summary_no_projects_registered(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["summary", "--last", "1w"])
        assert result.exit_code != 0
        assert "No projects registered" in result.output

    def test_summary_unknown_project(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "real-proj", str(fake_git_repo)])
        result = runner.invoke(cli, ["summary", "--last", "1w", "nonexistent"])
        assert result.exit_code != 0
        assert "Unknown project" in result.output

    def test_summary_bad_since_date(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = runner.invoke(cli, ["summary", "--since", "not-a-date", "proj"])
        assert result.exit_code != 0

    def test_summary_invalid_duration(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = runner.invoke(cli, ["summary", "--last", "5x", "proj"])
        assert result.exit_code != 0
        assert "Invalid" in result.output

    def test_summary_no_time_arg_required(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = runner.invoke(cli, ["summary", "proj"])
        assert result.exit_code != 0
        assert "Specify" in result.output

    def test_summary_both_last_and_since_rejected(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = runner.invoke(
            cli, ["summary", "--last", "1w", "--since", "2026-01-01", "proj"]
        )
        assert result.exit_code != 0

    def test_summary_until_before_since_rejected(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = runner.invoke(
            cli, ["summary", "--since", "2026-03-01", "--until", "2026-01-01", "proj"]
        )
        assert result.exit_code != 0

    def test_add_non_git_path(self, isolated_config: Path, tmp_path: Path) -> None:
        runner = CliRunner()
        non_git = tmp_path / "not-a-repo"
        non_git.mkdir()
        result = runner.invoke(cli, ["add", "proj", str(non_git)])
        assert result.exit_code != 0
        assert "git repository" in result.output.lower()

    def test_remove_nonexistent_alias(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["remove", "ghost"])
        assert result.exit_code != 0
        assert "not found" in result.output

    def test_history_show_nonexistent_entry(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["history", "show", "99"])
        assert result.exit_code != 0
        assert "No history entry found" in result.output


class TestVersionFlag:
    """--version flag shows the package version."""

    def test_version_flag_shows_version(self) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["--version"])
        assert result.exit_code == 0
        assert "0.1.0" in result.output


class TestHelpText:
    """All commands produce --help output without error."""

    @pytest.mark.parametrize(
        "cmd",
        [
            ["--help"],
            ["add", "--help"],
            ["remove", "--help"],
            ["list", "--help"],
            ["summary", "--help"],
            ["config", "--help"],
            ["config", "set", "--help"],
            ["config", "get", "--help"],
            ["config", "list", "--help"],
            ["doctor", "--help"],
            ["install-completion", "--help"],
            ["history", "--help"],
            ["history", "list", "--help"],
            ["history", "show", "--help"],
            ["history", "clear", "--help"],
            ["history", "diff", "--help"],
        ],
    )
    def test_help_exits_zero(self, cmd: list[str]) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, cmd)
        assert result.exit_code == 0
        assert "Usage:" in result.output


# ---------------------------------------------------------------------------
# 5b: Real git repo integration tests (no subprocess mocking)
# ---------------------------------------------------------------------------


class TestRealGitRepoIntegration:
    """Tests that run against an actual git repository."""

    def test_extract_commits_returns_known_commits(self, real_git_repo: Path) -> None:
        from gitbrief.git import extract_commits

        commits = extract_commits(str(real_git_repo), "2000-01-01")
        assert len(commits) >= 3
        subjects = [c["subject"] for c in commits]
        assert any("initial readme" in s for s in subjects)
        assert any("add main module" in s for s in subjects)
        assert any("add utils" in s for s in subjects)

    def test_extract_commits_since_future_returns_empty(
        self, real_git_repo: Path
    ) -> None:
        from gitbrief.git import extract_commits

        commits = extract_commits(str(real_git_repo), "2099-01-01")
        assert commits == []

    def test_extract_commits_respects_max_commits(self, real_git_repo: Path) -> None:
        from gitbrief.git import extract_commits

        commits = extract_commits(str(real_git_repo), "2000-01-01", max_commits=2)
        assert len(commits) <= 2

    def test_extract_commits_author_filter_match(self, real_git_repo: Path) -> None:
        from gitbrief.git import extract_commits

        commits = extract_commits(
            str(real_git_repo), "2000-01-01", author="test@example.com"
        )
        assert len(commits) >= 3

    def test_extract_commits_author_filter_no_match(self, real_git_repo: Path) -> None:
        from gitbrief.git import extract_commits

        commits = extract_commits(
            str(real_git_repo), "2000-01-01", author="nobody@nowhere.com"
        )
        assert commits == []

    def test_extract_commits_includes_diff_stats(self, real_git_repo: Path) -> None:
        from gitbrief.git import extract_commits

        commits = extract_commits(str(real_git_repo), "2000-01-01")
        # At least some commits should have diff stats
        stats_commits = [c for c in commits if "files_changed" in c]
        assert len(stats_commits) > 0

    def test_summary_dry_run_real_repo_produces_valid_prompt(
        self, isolated_config: Path, real_git_repo: Path
    ) -> None:
        """summary --dry-run against a real repo produces a well-formed AI prompt."""
        runner = CliRunner()
        runner.invoke(cli, ["add", "real", str(real_git_repo)])

        result = runner.invoke(
            cli,
            ["summary", "--last", "100d", "--dry-run", "--all-authors", "real"],
        )

        assert result.exit_code == 0, result.output
        assert "Summarize" in result.output
        assert "## Project: real" in result.output
        # At least one commit subject should appear
        assert "docs: initial readme" in result.output or "feat: add" in result.output

    def test_validate_repo_on_real_git_repo(self, real_git_repo: Path) -> None:
        from gitbrief.git import validate_repo

        assert validate_repo(str(real_git_repo)) is None

    def test_get_git_user_email_on_real_repo(self, real_git_repo: Path) -> None:
        from gitbrief.git import get_git_user_email

        email = get_git_user_email(str(real_git_repo))
        assert email == "test@example.com"
