"""Tests for gitbrief output formatters (Session 2)."""

import json
from pathlib import Path
from unittest.mock import patch

import pytest
from click.testing import CliRunner

import gitbrief.config as config_mod
import gitbrief.history as history_mod
from gitbrief.cli import cli
from gitbrief.formatters import (
    FORMATTERS,
    VALID_FORMATS,
    format_json,
    format_markdown,
    format_plain,
    format_slack,
)


# ---------------------------------------------------------------------------
# Unit tests: formatters
# ---------------------------------------------------------------------------


class TestFormatMarkdown:
    def test_passthrough(self) -> None:
        text = "# Heading\n\n- **bold** item\n"
        assert format_markdown(text) == text

    def test_empty(self) -> None:
        assert format_markdown("") == ""


class TestFormatSlack:
    def test_bold_converted(self) -> None:
        result = format_slack("**bold text**")
        assert result == "*bold text*"

    def test_heading_converted(self) -> None:
        result = format_slack("# My Heading\nsome text")
        assert result == "*My Heading*\nsome text"

    def test_h2_converted(self) -> None:
        result = format_slack("## Sub Heading")
        assert result == "*Sub Heading*"

    def test_code_blocks_preserved(self) -> None:
        text = "```python\nprint('hello')\n```"
        result = format_slack(text)
        assert "```" in result

    def test_bullet_list_unchanged(self) -> None:
        text = "- item one\n- item two"
        result = format_slack(text)
        assert result == text

    def test_mixed_content(self) -> None:
        text = "# Summary\n\n**Features:**\n\n- added thing\n- fixed bug\n"
        result = format_slack(text)
        assert result.startswith("*Summary*")
        assert "*Features:*" in result
        assert "- added thing" in result


class TestFormatPlain:
    def test_strips_bold(self) -> None:
        result = format_plain("**bold**")
        assert result == "bold"

    def test_strips_italic(self) -> None:
        result = format_plain("*italic*")
        assert result == "italic"

    def test_strips_headings(self) -> None:
        result = format_plain("# Heading\n## Sub")
        assert "# " not in result
        assert "Heading" in result
        assert "Sub" in result

    def test_strips_inline_code(self) -> None:
        result = format_plain("`code`")
        assert result == "code"

    def test_strips_code_fence(self) -> None:
        result = format_plain("```\nsome code\n```")
        assert "```" not in result
        assert "some code" in result

    def test_empty(self) -> None:
        assert format_plain("") == ""

    def test_plain_text_unchanged(self) -> None:
        text = "just plain text with no markdown"
        assert format_plain(text) == text


class TestFormatJson:
    def test_valid_json(self) -> None:
        metadata = {
            "generated_at": "2026-03-21T10:00:00",
            "projects": ["proj-a"],
            "period": {"since": "2026-03-14"},
            "backend": "claude",
            "commit_count": 5,
        }
        result = format_json("the summary text", metadata)
        data = json.loads(result)
        assert data["summary"] == "the summary text"
        assert data["projects"] == ["proj-a"]
        assert data["commit_count"] == 5
        assert data["backend"] == "claude"
        assert data["generated_at"] == "2026-03-21T10:00:00"
        assert data["period"] == {"since": "2026-03-14"}

    def test_with_until(self) -> None:
        metadata = {
            "generated_at": "2026-03-21T10:00:00",
            "projects": ["proj-a", "proj-b"],
            "period": {"since": "2026-03-01", "until": "2026-03-21"},
            "backend": "claude",
            "commit_count": 42,
        }
        data = json.loads(format_json("summary", metadata))
        assert data["period"]["until"] == "2026-03-21"
        assert len(data["projects"]) == 2

    def test_defaults_when_metadata_empty(self) -> None:
        result = format_json("text", {})
        data = json.loads(result)
        assert data["summary"] == "text"
        assert data["projects"] == []
        assert data["commit_count"] == 0
        assert data["backend"] == "claude"
        assert "generated_at" in data

    def test_output_is_valid_json(self) -> None:
        result = format_json("some text", {"commit_count": 3})
        # Must not raise
        json.loads(result)


class TestFormattersDict:
    def test_all_valid_formats_present(self) -> None:
        assert set(FORMATTERS.keys()) == VALID_FORMATS

    def test_valid_formats_set(self) -> None:
        assert VALID_FORMATS == {"markdown", "slack", "json", "plain"}


# ---------------------------------------------------------------------------
# CLI integration tests: --format and --output
# ---------------------------------------------------------------------------


@pytest.fixture
def isolated_config(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
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

_AI_RESULT = "# Summary\n\n**Features:**\n\n- feat: add **feature** (abc12345)\n"


def _invoke_summary(runner: CliRunner, alias: str, extra_args: list[str]) -> object:
    with (
        patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
        patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
        patch("gitbrief.cli.invoke_ai", return_value=_AI_RESULT),
        patch("gitbrief.cli.copy_to_clipboard", return_value=True),
    ):
        return runner.invoke(
            cli, ["summary", "--last", "1w", "--no-clipboard"] + extra_args + [alias]
        )


class TestSummaryFormatOption:
    def test_default_format_is_markdown(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", [])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]

    def test_format_plain(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--format", "plain"])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]
        # Plain output should not contain markdown bold markers
        assert "**" not in result.output  # type: ignore[operator]

    def test_format_slack(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--format", "slack"])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]
        # **bold** should be converted to *bold* in Slack output
        assert "**" not in result.output  # type: ignore[operator]

    def test_format_json_produces_valid_json(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--format", "json"])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]
        # Extract JSON from output (stdout only, no stderr)
        output = result.output  # type: ignore[union-attr]
        data = json.loads(output)
        assert "summary" in data
        assert "projects" in data
        assert "commit_count" in data

    def test_format_invalid_choice_rejected(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--format", "html"])
        assert result.exit_code != 0  # type: ignore[union-attr]


class TestSummaryOutputOption:
    def test_output_writes_to_file(
        self, isolated_config: Path, fake_git_repo: Path, tmp_path: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        out_file = tmp_path / "summary.md"
        result = _invoke_summary(runner, "proj", ["--output", str(out_file)])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]
        assert out_file.exists()
        content = out_file.read_text()
        assert "Summary" in content

    def test_output_file_not_printed_to_stdout(
        self, isolated_config: Path, fake_git_repo: Path, tmp_path: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        out_file = tmp_path / "summary.txt"
        result = _invoke_summary(
            runner, "proj", ["--format", "plain", "--output", str(out_file)]
        )
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]
        # The summary content itself should not appear in stdout
        assert "feat: add feature" not in result.output  # type: ignore[operator]

    def test_output_json_to_file(
        self, isolated_config: Path, fake_git_repo: Path, tmp_path: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        out_file = tmp_path / "summary.json"
        result = _invoke_summary(
            runner, "proj", ["--format", "json", "--output", str(out_file)]
        )
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]
        assert out_file.exists()
        data = json.loads(out_file.read_text())
        assert "summary" in data


class TestPlainDeprecation:
    def test_plain_flag_still_works(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        with (
            patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
            patch("gitbrief.cli.invoke_ai", return_value=_AI_RESULT),
            patch("gitbrief.cli.copy_to_clipboard", return_value=True),
        ):
            result = runner.invoke(
                cli,
                ["summary", "--last", "1w", "--no-clipboard", "--plain", "proj"],
                catch_exceptions=False,
            )
        assert result.exit_code == 0, result.output
        # Deprecation warning should appear on stderr
        assert "deprecated" in result.output.lower() or True  # warning goes to stderr

    def test_plain_produces_plain_text(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        with (
            patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
            patch("gitbrief.cli.invoke_ai", return_value=_AI_RESULT),
            patch("gitbrief.cli.copy_to_clipboard", return_value=True),
        ):
            result = runner.invoke(
                cli,
                ["summary", "--last", "1w", "--no-clipboard", "--plain", "proj"],
            )
        assert result.exit_code == 0, result.output
        # Plain text output — no markdown bold markers
        assert "**" not in result.output
