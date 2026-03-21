"""Tests for gitbrief.templates module and template-related CLI (Session 3)."""

from pathlib import Path
from unittest.mock import patch

import pytest
from click.testing import CliRunner

import gitbrief.config as config_mod
import gitbrief.history as history_mod
from gitbrief.cli import cli
from gitbrief.templates import (
    BUILT_IN_TEMPLATES,
    load_template,
    list_templates,
    render_template,
)


# ---------------------------------------------------------------------------
# Unit tests: load_template
# ---------------------------------------------------------------------------


class TestLoadTemplate:
    def test_load_default_builtin(self) -> None:
        tmpl = load_template("default")
        assert "{{period}}" in tmpl
        assert "{{commits}}" in tmpl

    def test_load_standup_builtin(self) -> None:
        tmpl = load_template("standup")
        assert "{{commits}}" in tmpl
        assert "3-5 bullet points" in tmpl

    def test_load_executive_builtin(self) -> None:
        tmpl = load_template("executive")
        assert "{{commits}}" in tmpl
        assert "strategic" in tmpl

    def test_load_nonexistent_raises(self) -> None:
        with pytest.raises(FileNotFoundError):
            load_template("nonexistent_template_xyz")

    def test_load_custom_file_path(self, tmp_path: Path) -> None:
        custom = tmp_path / "my_template.txt"
        custom.write_text("Custom: {{period}} {{commits}}")
        result = load_template(str(custom))
        assert "Custom:" in result
        assert "{{period}}" in result

    def test_load_nonexistent_file_path_raises(self, tmp_path: Path) -> None:
        with pytest.raises(FileNotFoundError):
            load_template(str(tmp_path / "does_not_exist.txt"))


# ---------------------------------------------------------------------------
# Unit tests: list_templates
# ---------------------------------------------------------------------------


class TestListTemplates:
    def test_returns_list_of_dicts(self) -> None:
        result = list_templates()
        assert isinstance(result, list)
        assert all(isinstance(t, dict) for t in result)

    def test_each_entry_has_name_and_description(self) -> None:
        for t in list_templates():
            assert "name" in t
            assert "description" in t

    def test_all_builtin_names_present(self) -> None:
        names = {t["name"] for t in list_templates()}
        assert names == set(BUILT_IN_TEMPLATES.keys())

    def test_builtin_templates_include_default(self) -> None:
        names = {t["name"] for t in list_templates()}
        assert "default" in names

    def test_builtin_templates_include_standup(self) -> None:
        names = {t["name"] for t in list_templates()}
        assert "standup" in names

    def test_builtin_templates_include_executive(self) -> None:
        names = {t["name"] for t in list_templates()}
        assert "executive" in names


# ---------------------------------------------------------------------------
# Unit tests: render_template
# ---------------------------------------------------------------------------


class TestRenderTemplate:
    def test_replaces_single_placeholder(self) -> None:
        result = render_template("Hello {{name}}!", {"name": "World"})
        assert result == "Hello World!"

    def test_replaces_multiple_placeholders(self) -> None:
        tmpl = "{{greeting}} {{name}}, the period is {{period}}."
        context = {"greeting": "Hi", "name": "Bob", "period": "last week"}
        result = render_template(tmpl, context)
        assert result == "Hi Bob, the period is last week."

    def test_empty_value_removes_placeholder(self) -> None:
        result = render_template("Before{{extra}}After", {"extra": ""})
        assert result == "BeforeAfter"

    def test_unused_context_keys_ignored(self) -> None:
        result = render_template("Hello {{name}}!", {"name": "Alice", "unused": "x"})
        assert result == "Hello Alice!"

    def test_missing_placeholder_left_intact(self) -> None:
        result = render_template("Hello {{name}} and {{other}}!", {"name": "Alice"})
        assert "{{other}}" in result

    def test_default_template_renders_with_context(self) -> None:
        tmpl = load_template("default")
        context = {
            "period": "the last 1w",
            "commits": "## Project: proj\n\n- abc12345 feat: thing\n",
            "projects": "proj",
            "detail_instructions": "",
        }
        result = render_template(tmpl, context)
        assert "the last 1w" in result
        assert "abc12345" in result
        assert "proj" in result

    def test_detail_instructions_injected(self) -> None:
        tmpl = load_template("default")
        context = {
            "period": "today",
            "commits": "## Project: p\n\n- abc12345 fix: thing\n",
            "projects": "p",
            "detail_instructions": "Be extremely concise.",
        }
        result = render_template(tmpl, context)
        assert "Be extremely concise." in result

    def test_standup_template_renders(self) -> None:
        tmpl = load_template("standup")
        context = {
            "period": "this week",
            "commits": "## Project: p\n\n- abc12345 feat: thing\n",
            "projects": "p",
            "detail_instructions": "",
        }
        result = render_template(tmpl, context)
        assert "this week" in result

    def test_executive_template_renders(self) -> None:
        tmpl = load_template("executive")
        context = {
            "period": "Q1",
            "commits": "## Project: p\n\n- abc12345 feat: big feature\n",
            "projects": "p",
            "detail_instructions": "",
        }
        result = render_template(tmpl, context)
        assert "Q1" in result


# ---------------------------------------------------------------------------
# CLI integration tests: template list / template show
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

_AI_RESULT = "# Summary\n\n- feat: add feature (abc12345)\n"


class TestTemplateCLIList:
    def test_template_list_exits_ok(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["template", "list"])
        assert result.exit_code == 0, result.output

    def test_template_list_shows_default(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["template", "list"])
        assert "default" in result.output

    def test_template_list_shows_standup(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["template", "list"])
        assert "standup" in result.output

    def test_template_list_shows_executive(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["template", "list"])
        assert "executive" in result.output


class TestTemplateCLIShow:
    def test_show_default(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["template", "show", "default"])
        assert result.exit_code == 0, result.output
        assert "{{commits}}" in result.output

    def test_show_standup(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["template", "show", "standup"])
        assert result.exit_code == 0, result.output
        assert "bullet points" in result.output

    def test_show_nonexistent_errors(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["template", "show", "no_such_template"])
        assert result.exit_code != 0

    def test_show_nonexistent_suggests_list(self, isolated_config: Path) -> None:
        runner = CliRunner()
        result = runner.invoke(cli, ["template", "show", "no_such_template"])
        assert "template list" in result.output


# ---------------------------------------------------------------------------
# CLI integration tests: --template and --detail on summary
# ---------------------------------------------------------------------------


def _invoke_summary(
    runner: CliRunner, alias: str, extra_args: list[str]
) -> object:
    with (
        patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
        patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
        patch("gitbrief.cli.invoke_ai", return_value=_AI_RESULT),
        patch("gitbrief.cli.copy_to_clipboard", return_value=True),
    ):
        return runner.invoke(
            cli, ["summary", "--last", "1w", "--no-clipboard"] + extra_args + [alias]
        )


class TestSummaryDetailOption:
    def test_detail_normal_default(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", [])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]

    def test_detail_brief(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--detail", "brief"])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]

    def test_detail_detailed(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--detail", "detailed"])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]

    def test_detail_invalid_rejected(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--detail", "ultra"])
        assert result.exit_code != 0  # type: ignore[union-attr]

    def test_detail_brief_injected_into_prompt(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        captured_prompts: list[str] = []

        def fake_ai(prompt: str, *args, **kwargs) -> str:
            captured_prompts.append(prompt)
            return _AI_RESULT

        with (
            patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
            patch("gitbrief.cli.invoke_ai", side_effect=fake_ai),
            patch("gitbrief.cli.copy_to_clipboard", return_value=True),
        ):
            runner.invoke(
                cli,
                ["summary", "--last", "1w", "--no-clipboard", "--detail", "brief", "proj"],
            )
        assert captured_prompts, "invoke_ai was not called"
        assert "3-5 bullet points" in captured_prompts[0]

    def test_detail_detailed_injected_into_prompt(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        captured_prompts: list[str] = []

        def fake_ai(prompt: str, *args, **kwargs) -> str:
            captured_prompts.append(prompt)
            return _AI_RESULT

        with (
            patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
            patch("gitbrief.cli.invoke_ai", side_effect=fake_ai),
            patch("gitbrief.cli.copy_to_clipboard", return_value=True),
        ):
            runner.invoke(
                cli,
                ["summary", "--last", "1w", "--no-clipboard", "--detail", "detailed", "proj"],
            )
        assert captured_prompts
        assert "detailed commentary" in captured_prompts[0].lower() or "SHAs" in captured_prompts[0]

    def test_detail_normal_no_extra_instructions(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        captured_prompts: list[str] = []

        def fake_ai(prompt: str, *args, **kwargs) -> str:
            captured_prompts.append(prompt)
            return _AI_RESULT

        with (
            patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
            patch("gitbrief.cli.invoke_ai", side_effect=fake_ai),
            patch("gitbrief.cli.copy_to_clipboard", return_value=True),
        ):
            runner.invoke(
                cli,
                ["summary", "--last", "1w", "--no-clipboard", "--detail", "normal", "proj"],
            )
        assert captured_prompts
        # normal detail level should not inject extra instructions
        assert "3-5 bullet points" not in captured_prompts[0]
        assert "detailed commentary" not in captured_prompts[0].lower()


class TestSummaryTemplateOption:
    def test_template_default_ok(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--template", "default"])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]

    def test_template_standup_ok(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--template", "standup"])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]

    def test_template_executive_ok(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--template", "executive"])
        assert result.exit_code == 0, result.output  # type: ignore[union-attr]

    def test_template_nonexistent_errors(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        result = _invoke_summary(runner, "proj", ["--template", "no_such_template"])
        assert result.exit_code != 0  # type: ignore[union-attr]

    def test_custom_template_file(
        self, isolated_config: Path, fake_git_repo: Path, tmp_path: Path
    ) -> None:
        custom = tmp_path / "custom.txt"
        custom.write_text("CUSTOM PROMPT for {{period}}:\n{{commits}}")
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        captured_prompts: list[str] = []

        def fake_ai(prompt: str, *args, **kwargs) -> str:
            captured_prompts.append(prompt)
            return _AI_RESULT

        with (
            patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
            patch("gitbrief.cli.invoke_ai", side_effect=fake_ai),
            patch("gitbrief.cli.copy_to_clipboard", return_value=True),
        ):
            result = runner.invoke(
                cli,
                [
                    "summary",
                    "--last",
                    "1w",
                    "--no-clipboard",
                    "--template",
                    str(custom),
                    "proj",
                ],
            )
        assert result.exit_code == 0, result.output
        assert captured_prompts
        assert "CUSTOM PROMPT" in captured_prompts[0]

    def test_standup_template_used_in_prompt(
        self, isolated_config: Path, fake_git_repo: Path
    ) -> None:
        runner = CliRunner()
        runner.invoke(cli, ["add", "proj", str(fake_git_repo)])
        captured_prompts: list[str] = []

        def fake_ai(prompt: str, *args, **kwargs) -> str:
            captured_prompts.append(prompt)
            return _AI_RESULT

        with (
            patch("gitbrief.cli.extract_commits", return_value=_SAMPLE_COMMITS),
            patch("gitbrief.cli.get_git_user_email", return_value="user@test.com"),
            patch("gitbrief.cli.invoke_ai", side_effect=fake_ai),
            patch("gitbrief.cli.copy_to_clipboard", return_value=True),
        ):
            runner.invoke(
                cli,
                ["summary", "--last", "1w", "--no-clipboard", "--template", "standup", "proj"],
            )
        assert captured_prompts
        assert "bullet points" in captured_prompts[0]
