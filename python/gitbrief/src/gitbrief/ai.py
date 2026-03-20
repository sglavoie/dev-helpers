from __future__ import annotations

import subprocess
import time

from gitbrief.config import get_setting_int
from gitbrief.exceptions import (
    AIBackendError,
    AIConnectionError,
    AIRateLimitError,
    AITimeoutError,
    ErrorClass,
    classify_error,
    is_retryable,
)

BASE_DELAY = 2.0
RATE_LIMIT_BASE_DELAY = 30.0

_INSTALL_HINTS: dict[str, str] = {
    "claude": (
        "Install Claude Code CLI: npm install -g @anthropic-ai/claude-code\n"
        "  Then authenticate: claude auth login\n"
        "  Docs: https://docs.anthropic.com/claude-code"
    ),
    "copilot": (
        "Install GitHub Copilot CLI extension: gh extension install github/gh-copilot\n"
        "  Requires GitHub CLI: https://cli.github.com\n"
        "  Then authenticate: gh auth login"
    ),
}

_DEFAULT_TIMEOUT = 120
_DEFAULT_RETRIES = 2


def _build_cmd(prompt: str, backend: str) -> list[str]:
    if backend == "claude":
        return ["claude", "-p", prompt]
    if backend == "copilot":
        return ["copilot", "-p", prompt, "-s"]
    raise AIBackendError(
        f"Unknown backend '{backend}'",
        hint=f"Valid backends: {', '.join(sorted(_INSTALL_HINTS))}",
    )


def invoke_ai(
    prompt: str,
    backend: str,
    *,
    timeout: int | None = None,
    max_retries: int | None = None,
) -> str:
    """Invoke an AI CLI tool to summarize commits, with retry logic."""
    resolved_timeout = timeout if timeout is not None else get_setting_int("timeout", _DEFAULT_TIMEOUT)
    resolved_retries = max_retries if max_retries is not None else get_setting_int("retries", _DEFAULT_RETRIES)

    cmd = _build_cmd(prompt, backend)
    hint = _INSTALL_HINTS.get(backend)

    last_error: AIBackendError | None = None

    for attempt in range(resolved_retries + 1):
        if attempt > 0:
            error_class = classify_error(last_error.stderr if last_error else "", last_error.exit_code if last_error else -1)
            base = RATE_LIMIT_BASE_DELAY if error_class == ErrorClass.RETRYABLE_RATE_LIMIT else BASE_DELAY
            delay = base * (2 ** (attempt - 1))
            time.sleep(delay)

        try:
            result = subprocess.run(
                cmd, capture_output=True, text=True, timeout=resolved_timeout
            )
        except FileNotFoundError:
            install_hint = f"Install it with: {hint}" if hint else f"Install '{backend}' CLI first."
            raise AIBackendError(
                f"'{backend}' CLI not found.",
                hint=install_hint,
            )
        except subprocess.TimeoutExpired:
            last_error = AITimeoutError(
                f"'{backend}' timed out after {resolved_timeout}s.",
                exit_code=124,
                hint="Try increasing timeout: gitbrief config set timeout 300",
            )
            if attempt < resolved_retries:
                continue
            raise last_error

        if result.returncode != 0:
            error_class = classify_error(result.stderr, result.returncode)
            msg = f"'{backend}' failed (exit {result.returncode})."
            stderr = result.stderr
            exit_code = result.returncode
            if error_class == ErrorClass.RETRYABLE_RATE_LIMIT:
                last_error = AIRateLimitError(msg, stderr=stderr, exit_code=exit_code)
            elif error_class == ErrorClass.RETRYABLE_TIMEOUT:
                last_error = AITimeoutError(msg, stderr=stderr, exit_code=exit_code)
            elif error_class == ErrorClass.RETRYABLE_CONNECTION:
                last_error = AIConnectionError(msg, stderr=stderr, exit_code=exit_code)
            else:
                raise AIBackendError(msg, stderr=stderr, exit_code=exit_code)

            if attempt < resolved_retries and is_retryable(last_error):
                continue
            raise last_error

        return result.stdout.strip()

    assert last_error is not None
    raise last_error
