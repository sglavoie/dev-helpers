"""CLI integration tests for the add/list/summary, config, and doctor flows."""

import json
from pathlib import Path
from unittest.mock import patch

from click.testing import CliRunner

from gitbrief.cli import cli


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
        self, isolated_config: Path, fake_git_repo: Path, sample_commits: list[dict]
    ) -> None:
        runner = CliRunner()
        alias = "my-repo"
        runner.invoke(cli, ["add", alias, str(fake_git_repo)])

        with (
            patch(
                "gitbrief.commands.collect.extract_commits", return_value=sample_commits
            ),
            patch(
                "gitbrief.commands.collect.get_git_user_email",
                return_value="user@test.com",
            ),
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
            patch("gitbrief.commands.collect.extract_commits", return_value=[]),
            patch(
                "gitbrief.commands.collect.get_git_user_email",
                return_value="user@test.com",
            ),
        ):
            result = runner.invoke(cli, ["summary", "--last", "1d", alias])

        assert result.exit_code == 0
        assert "No activity found" in result.output

    def test_summary_all_projects_when_none_specified(
        self,
        isolated_config: Path,
        fake_git_repo: Path,
        tmp_path: Path,
        sample_commits: list[dict],
    ) -> None:
        """When no project argument is given, all registered projects are used."""
        runner = CliRunner()
        repo2 = tmp_path / "repo2"
        repo2.mkdir()
        (repo2 / ".git").mkdir()

        runner.invoke(cli, ["add", "proj-a", str(fake_git_repo)])
        runner.invoke(cli, ["add", "proj-b", str(repo2)])

        with (
            patch(
                "gitbrief.commands.collect.extract_commits", return_value=sample_commits
            ),
            patch(
                "gitbrief.commands.collect.get_git_user_email",
                return_value="user@test.com",
            ),
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
            "gitbrief.commands.doctor.extract_commits",
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
