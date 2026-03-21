"""Tests for gitbrief.ai module."""

import subprocess
from unittest.mock import MagicMock, patch

import pytest

from gitbrief.ai import invoke_ai
from gitbrief.exceptions import AIBackendError, AIRateLimitError, AITimeoutError


def _success_result(stdout="AI summary output"):
    r = MagicMock()
    r.returncode = 0
    r.stdout = stdout
    r.stderr = ""
    return r


def _failure_result(stderr="", returncode=1):
    r = MagicMock()
    r.returncode = returncode
    r.stdout = ""
    r.stderr = stderr
    return r


class TestInvokeAI:
    def test_success_returns_stdout(self):
        with patch(
            "gitbrief.ai.subprocess.run", return_value=_success_result("summary")
        ):
            result = invoke_ai("prompt", "claude", timeout=10, max_retries=0)
        assert result == "summary"

    def test_strips_trailing_whitespace(self):
        with patch(
            "gitbrief.ai.subprocess.run", return_value=_success_result("  result  \n")
        ):
            result = invoke_ai("prompt", "claude", timeout=10, max_retries=0)
        assert result == "result"

    def test_raises_when_backend_not_found(self):
        with patch("gitbrief.ai.subprocess.run", side_effect=FileNotFoundError):
            with pytest.raises(AIBackendError, match="not found"):
                invoke_ai("prompt", "claude", timeout=10, max_retries=0)

    def test_raises_on_permanent_failure(self):
        with patch(
            "gitbrief.ai.subprocess.run",
            return_value=_failure_result("unknown error", 1),
        ):
            with pytest.raises(AIBackendError):
                invoke_ai("prompt", "claude", timeout=10, max_retries=0)

    def test_retries_on_rate_limit(self):
        results = [
            _failure_result("rate limit exceeded", 1),
            _failure_result("rate limit exceeded", 1),
            _success_result("ok"),
        ]
        with patch("gitbrief.ai.subprocess.run", side_effect=results):
            with patch("gitbrief.ai.time.sleep"):
                result = invoke_ai("prompt", "claude", timeout=10, max_retries=2)
        assert result == "ok"

    def test_raises_after_all_retries_exhausted(self):
        failure = _failure_result("rate limit exceeded", 1)
        with patch("gitbrief.ai.subprocess.run", return_value=failure):
            with patch("gitbrief.ai.time.sleep"):
                with pytest.raises(AIRateLimitError):
                    invoke_ai("prompt", "claude", timeout=10, max_retries=2)

    def test_timeout_raises_ai_timeout_error(self):
        with patch(
            "gitbrief.ai.subprocess.run",
            side_effect=subprocess.TimeoutExpired(cmd=["claude"], timeout=10),
        ):
            with pytest.raises(AITimeoutError):
                invoke_ai("prompt", "claude", timeout=10, max_retries=0)

    def test_timeout_retries_then_raises(self):
        with patch(
            "gitbrief.ai.subprocess.run",
            side_effect=subprocess.TimeoutExpired(cmd=["claude"], timeout=10),
        ):
            with patch("gitbrief.ai.time.sleep"):
                with pytest.raises(AITimeoutError):
                    invoke_ai("prompt", "claude", timeout=10, max_retries=2)

    def test_unknown_backend_raises(self):
        with pytest.raises(AIBackendError, match="Unknown backend"):
            invoke_ai("prompt", "notabackend", timeout=10, max_retries=0)

    def test_sleep_called_between_retries(self):
        results = [_failure_result("rate limit exceeded", 1), _success_result("ok")]
        with patch("gitbrief.ai.subprocess.run", side_effect=results):
            with patch("gitbrief.ai.time.sleep") as mock_sleep:
                invoke_ai("prompt", "claude", timeout=10, max_retries=1)
        assert mock_sleep.called
