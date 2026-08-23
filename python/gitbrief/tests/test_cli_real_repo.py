"""Integration tests that run against an actual git repository (no mocking)."""

from pathlib import Path

from click.testing import CliRunner

from gitbrief.cli import cli
from gitbrief.git import extract_commits, get_git_user_email, validate_repo


class TestRealGitRepoIntegration:
    """Tests that run against an actual git repository."""

    def test_extract_commits_returns_known_commits(self, real_git_repo: Path) -> None:
        commits = extract_commits(str(real_git_repo), "2000-01-01")
        assert len(commits) >= 3
        subjects = [c["subject"] for c in commits]
        assert any("initial readme" in s for s in subjects)
        assert any("add main module" in s for s in subjects)
        assert any("add utils" in s for s in subjects)

    def test_extract_commits_since_future_returns_empty(
        self, real_git_repo: Path
    ) -> None:
        assert extract_commits(str(real_git_repo), "2099-01-01") == []

    def test_extract_commits_respects_max_commits(self, real_git_repo: Path) -> None:
        commits = extract_commits(str(real_git_repo), "2000-01-01", max_commits=2)
        assert len(commits) <= 2

    def test_extract_commits_author_filter_match(self, real_git_repo: Path) -> None:
        commits = extract_commits(
            str(real_git_repo), "2000-01-01", author="test@example.com"
        )
        assert len(commits) >= 3

    def test_extract_commits_author_filter_no_match(self, real_git_repo: Path) -> None:
        commits = extract_commits(
            str(real_git_repo), "2000-01-01", author="nobody@nowhere.com"
        )
        assert commits == []

    def test_extract_commits_includes_diff_stats(self, real_git_repo: Path) -> None:
        commits = extract_commits(str(real_git_repo), "2000-01-01")
        # At least some commits should have diff stats
        assert [c for c in commits if "files_changed" in c]

    def test_summary_dry_run_real_repo_produces_valid_prompt(
        self, isolated_config: Path, real_git_repo: Path
    ) -> None:
        """summary --dry-run against a real repo produces a well-formed AI prompt."""
        runner = CliRunner()
        runner.invoke(cli, ["add", "real", str(real_git_repo)])

        result = runner.invoke(
            cli, ["summary", "--last", "100d", "--dry-run", "--all-authors", "real"]
        )

        assert result.exit_code == 0, result.output
        assert "Summarize" in result.output
        assert "## Project: real" in result.output
        # At least one commit subject should appear
        assert "docs: initial readme" in result.output or "feat: add" in result.output

    def test_validate_repo_on_real_git_repo(self, real_git_repo: Path) -> None:
        assert validate_repo(str(real_git_repo)) is None

    def test_get_git_user_email_on_real_repo(self, real_git_repo: Path) -> None:
        assert get_git_user_email(str(real_git_repo)) == "test@example.com"
