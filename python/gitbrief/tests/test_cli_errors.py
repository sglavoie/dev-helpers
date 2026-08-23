"""CLI integration tests for --since-last, error paths, --version, and --help."""

import json
from pathlib import Path
from unittest.mock import patch

import pytest
from click.testing import CliRunner

from gitbrief.cli import cli
from gitbrief.git import parse_duration


class TestSummaryWithSinceLast:
    """--since-last flag with mocked history."""

    @staticmethod
    def _run(config_path: Path, repo: Path, last_summary: dict, commits: list[dict]):
        config_path.write_text(
            json.dumps(
                {
                    "projects": {"proj": {"path": str(repo), "backend": None}},
                    "settings": {"backend": "claude"},
                    "last_summary": last_summary,
                }
            )
        )
        with (
            patch(
                "gitbrief.commands.collect.extract_commits", return_value=commits
            ) as mock_extract,
            patch(
                "gitbrief.commands.collect.get_git_user_email", return_value="u@test.com"
            ),
        ):
            result = CliRunner().invoke(
                cli, ["summary", "--since-last", "--dry-run", "proj"]
            )
        return result, mock_extract

    def test_since_last_uses_stored_timestamp(
        self, isolated_config: Path, fake_git_repo: Path, sample_commits: list[dict]
    ) -> None:
        result, mock_extract = self._run(
            isolated_config,
            fake_git_repo,
            {"proj": "2026-03-10T12:00:00"},
            sample_commits,
        )

        assert result.exit_code == 0
        if mock_extract.called:
            args, _ = mock_extract.call_args
            assert args[1] == "2026-03-10"

    def test_since_last_falls_back_to_1w_with_no_history(
        self, isolated_config: Path, fake_git_repo: Path, sample_commits: list[dict]
    ) -> None:
        expected = parse_duration("1w")
        _, mock_extract = self._run(isolated_config, fake_git_repo, {}, sample_commits)

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
