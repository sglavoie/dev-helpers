"""Tests for graceful degradation and new error classification (Session 2)."""

from unittest.mock import MagicMock, patch

import pytest
from click.testing import CliRunner

from gitbrief.cli import cli
from gitbrief.exceptions import (
    AIAuthenticationError,
    AIContextOverflowError,
    ErrorClass,
    classify_error,
    is_retryable,
)


# ---------------------------------------------------------------------------
# classify_error — new patterns
# ---------------------------------------------------------------------------


class TestClassifyErrorNewPatterns:
    def test_unauthorized(self):
        assert classify_error("unauthorized", 401) == ErrorClass.AUTHENTICATION

    def test_401_in_text(self):
        assert classify_error("error 401 returned", 0) == ErrorClass.AUTHENTICATION

    def test_invalid_api_key(self):
        assert classify_error("invalid api key", 1) == ErrorClass.AUTHENTICATION

    def test_authentication_word(self):
        assert classify_error("authentication failed", 1) == ErrorClass.AUTHENTICATION

    def test_forbidden(self):
        assert classify_error("forbidden", 403) == ErrorClass.AUTHENTICATION

    def test_403_in_text(self):
        assert classify_error("error 403 forbidden", 0) == ErrorClass.AUTHENTICATION

    def test_auth_case_insensitive(self):
        assert classify_error("Unauthorized Access", 0) == ErrorClass.AUTHENTICATION

    def test_prompt_too_long(self):
        assert classify_error("prompt is too long", 1) == ErrorClass.CONTEXT_OVERFLOW

    def test_token_limit(self):
        assert classify_error("token limit exceeded", 1) == ErrorClass.CONTEXT_OVERFLOW

    def test_context_window(self):
        assert (
            classify_error("context window exceeded", 1) == ErrorClass.CONTEXT_OVERFLOW
        )

    def test_too_large(self):
        assert classify_error("request is too large", 1) == ErrorClass.CONTEXT_OVERFLOW

    def test_context_overflow_case_insensitive(self):
        assert classify_error("Token Limit Exceeded", 0) == ErrorClass.CONTEXT_OVERFLOW

    def test_rate_limit_takes_priority_over_auth(self):
        # Rate limit patterns checked first
        assert (
            classify_error("rate limit unauthorized", 0)
            == ErrorClass.RETRYABLE_RATE_LIMIT
        )

    def test_timeout_takes_priority_over_context_overflow(self):
        # Timeout patterns checked before context overflow
        assert (
            classify_error("timed out token limit", 0) == ErrorClass.RETRYABLE_TIMEOUT
        )


# ---------------------------------------------------------------------------
# is_retryable — new error classes are NOT retryable
# ---------------------------------------------------------------------------


class TestIsRetryableNewErrors:
    def test_auth_error_not_retryable(self):
        assert is_retryable(AIAuthenticationError("auth failed")) is False

    def test_context_overflow_not_retryable(self):
        assert is_retryable(AIContextOverflowError("too large")) is False


# ---------------------------------------------------------------------------
# invoke_ai — new error classes raised with hints
# ---------------------------------------------------------------------------


def _failure_result(stderr="", returncode=1):
    r = MagicMock()
    r.returncode = returncode
    r.stdout = ""
    r.stderr = stderr
    return r


class TestInvokeAINewErrors:
    def test_auth_error_raised_immediately(self):
        from gitbrief.ai import invoke_ai
        from gitbrief.exceptions import AUTH_HINT

        with patch(
            "gitbrief.ai.subprocess.run",
            return_value=_failure_result("unauthorized access", 401),
        ):
            with pytest.raises(AIAuthenticationError) as exc_info:
                invoke_ai("prompt", "claude", timeout=10, max_retries=2)
        assert exc_info.value.hint == AUTH_HINT

    def test_context_overflow_raised_immediately(self):
        from gitbrief.ai import invoke_ai
        from gitbrief.exceptions import CONTEXT_OVERFLOW_HINT

        with patch(
            "gitbrief.ai.subprocess.run",
            return_value=_failure_result("prompt is too long", 1),
        ):
            with pytest.raises(AIContextOverflowError) as exc_info:
                invoke_ai("prompt", "claude", timeout=10, max_retries=2)
        assert exc_info.value.hint == CONTEXT_OVERFLOW_HINT

    def test_auth_error_not_retried(self):
        """Auth errors should not trigger retries — subprocess.run called only once."""
        from gitbrief.ai import invoke_ai

        with patch(
            "gitbrief.ai.subprocess.run",
            return_value=_failure_result("unauthorized", 401),
        ) as mock_run:
            with patch("gitbrief.ai.time.sleep"):
                with pytest.raises(AIAuthenticationError):
                    invoke_ai("prompt", "claude", timeout=10, max_retries=2)
        assert mock_run.call_count == 1


# ---------------------------------------------------------------------------
# retry progress feedback
# ---------------------------------------------------------------------------


class TestRetryProgressFeedback:
    def test_retry_message_printed_to_stderr(self):
        from gitbrief.ai import invoke_ai

        results = [
            _failure_result("rate limit exceeded", 1),
            _failure_result("rate limit exceeded", 1),
        ]
        with patch("gitbrief.ai.subprocess.run", side_effect=results):
            with patch("gitbrief.ai.time.sleep"):
                with patch("gitbrief.ai._stderr_console") as mock_console:
                    with pytest.raises(Exception):
                        invoke_ai("prompt", "claude", timeout=10, max_retries=1)
        assert mock_console.print.called
        call_args = mock_console.print.call_args[0][0]
        assert "Retrying" in call_args
        assert "1/1" in call_args


# ---------------------------------------------------------------------------
# CLI graceful degradation
# ---------------------------------------------------------------------------


def _make_project_config(tmp_path):
    """Write a config with one project pointing to a temp git-like path."""
    import json

    repo = tmp_path / "repo"
    repo.mkdir()
    (repo / ".git").mkdir()

    config_file = tmp_path / ".gitbrief.json"
    config = {
        "projects": {"myproj": {"path": str(repo)}},
        "settings": {
            "backend": "claude",
            "timeout": 10,
            "retries": 0,
            "max_commits": 100,
        },
    }
    config_file.write_text(json.dumps(config))
    return config_file, repo


SAMPLE_COMMITS = [
    {
        "sha": "abc1234",
        "subject": "feat: do something",
        "body": "",
        "branch": "main",
        "refs": [],
        "files_changed": 2,
        "insertions": 5,
        "deletions": 1,
    }
]


class TestGracefulDegradation:
    def test_fallback_raw_output_on_ai_failure(self, tmp_path):
        from gitbrief.exceptions import AIBackendError

        config_file, _ = _make_project_config(tmp_path)
        runner = CliRunner()

        with (
            patch("gitbrief.cli.load_config") as mock_load,
            patch("gitbrief.cli.validate_repo", return_value=None),
            patch("gitbrief.cli.get_git_user_email", return_value="user@example.com"),
            patch("gitbrief.cli.extract_commits", return_value=SAMPLE_COMMITS),
            patch("gitbrief.cli.invoke_ai", side_effect=AIBackendError("AI failed")),
        ):
            mock_load.return_value = {
                "projects": {"myproj": {"path": "/some/path"}},
                "settings": {
                    "backend": "claude",
                    "timeout": 10,
                    "retries": 0,
                    "max_commits": 100,
                },
            }
            result = runner.invoke(cli, ["summary", "--last", "1w", "myproj"])

        assert result.exit_code == 0
        assert "feat: do something" in result.output

    def test_no_fallback_exits_on_ai_failure(self, tmp_path):
        from gitbrief.exceptions import AIBackendError

        runner = CliRunner()

        with (
            patch("gitbrief.cli.load_config") as mock_load,
            patch("gitbrief.cli.validate_repo", return_value=None),
            patch("gitbrief.cli.get_git_user_email", return_value="user@example.com"),
            patch("gitbrief.cli.extract_commits", return_value=SAMPLE_COMMITS),
            patch("gitbrief.cli.invoke_ai", side_effect=AIBackendError("AI failed")),
        ):
            mock_load.return_value = {
                "projects": {"myproj": {"path": "/some/path"}},
                "settings": {
                    "backend": "claude",
                    "timeout": 10,
                    "retries": 0,
                    "max_commits": 100,
                },
            }
            result = runner.invoke(
                cli, ["summary", "--last", "1w", "--no-fallback", "myproj"]
            )

        assert result.exit_code != 0
        assert "feat: do something" not in result.output

    def test_fallback_shows_error_message(self, tmp_path):
        from gitbrief.exceptions import AIBackendError

        runner = CliRunner()

        with (
            patch("gitbrief.cli.load_config") as mock_load,
            patch("gitbrief.cli.validate_repo", return_value=None),
            patch("gitbrief.cli.get_git_user_email", return_value="user@example.com"),
            patch("gitbrief.cli.extract_commits", return_value=SAMPLE_COMMITS),
            patch("gitbrief.cli.invoke_ai", side_effect=AIBackendError("AI failed")),
        ):
            mock_load.return_value = {
                "projects": {"myproj": {"path": "/some/path"}},
                "settings": {
                    "backend": "claude",
                    "timeout": 10,
                    "retries": 0,
                    "max_commits": 100,
                },
            }
            result = runner.invoke(cli, ["summary", "--last", "1w", "myproj"])

        assert "AI summarization failed" in result.output

    def test_fallback_includes_hint(self, tmp_path):
        from gitbrief.exceptions import AIBackendError

        runner = CliRunner()

        with (
            patch("gitbrief.cli.load_config") as mock_load,
            patch("gitbrief.cli.validate_repo", return_value=None),
            patch("gitbrief.cli.get_git_user_email", return_value="user@example.com"),
            patch("gitbrief.cli.extract_commits", return_value=SAMPLE_COMMITS),
            patch(
                "gitbrief.cli.invoke_ai",
                side_effect=AIBackendError("AI failed", hint="Do this"),
            ),
        ):
            mock_load.return_value = {
                "projects": {"myproj": {"path": "/some/path"}},
                "settings": {
                    "backend": "claude",
                    "timeout": 10,
                    "retries": 0,
                    "max_commits": 100,
                },
            }
            result = runner.invoke(cli, ["summary", "--last", "1w", "myproj"])

        assert "Do this" in result.output
