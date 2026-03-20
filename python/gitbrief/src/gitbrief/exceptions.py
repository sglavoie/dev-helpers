from __future__ import annotations

from enum import Enum


class GitBriefError(Exception):
    def __init__(self, message: str, hint: str | None = None) -> None:
        super().__init__(message)
        self.hint = hint


class AIBackendError(GitBriefError):
    def __init__(
        self,
        message: str,
        *,
        stderr: str = "",
        exit_code: int = -1,
        hint: str | None = None,
    ) -> None:
        super().__init__(message, hint=hint)
        self.stderr = stderr
        self.exit_code = exit_code


class AITimeoutError(AIBackendError):
    pass


class AIConnectionError(AIBackendError):
    pass


class AIRateLimitError(AIBackendError):
    pass


class ErrorClass(Enum):
    RETRYABLE_TIMEOUT = "retryable_timeout"
    RETRYABLE_CONNECTION = "retryable_connection"
    RETRYABLE_RATE_LIMIT = "retryable_rate_limit"
    PERMANENT = "permanent"


_TIMEOUT_PATTERNS = ["timed out", "timeout", "deadline exceeded"]
_CONNECTION_PATTERNS = [
    "connection refused",
    "connection error",
    "connection reset",
    "failed to connect",
    "network unreachable",
    "no route to host",
    "name or service not known",
]
_RATE_LIMIT_PATTERNS = ["rate limit", "too many requests", "429", "quota exceeded"]


def classify_error(stderr: str, exit_code: int) -> ErrorClass:
    text = stderr.lower()
    for pattern in _RATE_LIMIT_PATTERNS:
        if pattern in text:
            return ErrorClass.RETRYABLE_RATE_LIMIT
    for pattern in _TIMEOUT_PATTERNS:
        if pattern in text:
            return ErrorClass.RETRYABLE_TIMEOUT
    if exit_code == 124:  # GNU timeout exit code
        return ErrorClass.RETRYABLE_TIMEOUT
    for pattern in _CONNECTION_PATTERNS:
        if pattern in text:
            return ErrorClass.RETRYABLE_CONNECTION
    return ErrorClass.PERMANENT


def is_retryable(error: AIBackendError) -> bool:
    return isinstance(error, (AITimeoutError, AIConnectionError, AIRateLimitError))
