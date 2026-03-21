"""Tests for gitbrief.exceptions module."""

from gitbrief.exceptions import (
    AIBackendError,
    AIConnectionError,
    AIRateLimitError,
    AITimeoutError,
    ErrorClass,
    classify_error,
    is_retryable,
)


# ---------------------------------------------------------------------------
# classify_error
# ---------------------------------------------------------------------------


class TestClassifyError:
    def test_rate_limit_pattern(self):
        assert (
            classify_error("rate limit exceeded", 429)
            == ErrorClass.RETRYABLE_RATE_LIMIT
        )

    def test_too_many_requests(self):
        assert (
            classify_error("too many requests", 429) == ErrorClass.RETRYABLE_RATE_LIMIT
        )

    def test_quota_exceeded(self):
        assert classify_error("quota exceeded", 1) == ErrorClass.RETRYABLE_RATE_LIMIT

    def test_429_in_text(self):
        assert (
            classify_error("error 429 returned", 0) == ErrorClass.RETRYABLE_RATE_LIMIT
        )

    def test_timeout_pattern(self):
        assert classify_error("connection timed out", 1) == ErrorClass.RETRYABLE_TIMEOUT

    def test_timeout_word(self):
        assert (
            classify_error("timeout waiting for response", 1)
            == ErrorClass.RETRYABLE_TIMEOUT
        )

    def test_deadline_exceeded(self):
        assert classify_error("deadline exceeded", 1) == ErrorClass.RETRYABLE_TIMEOUT

    def test_exit_code_124(self):
        assert classify_error("", 124) == ErrorClass.RETRYABLE_TIMEOUT

    def test_connection_refused(self):
        assert (
            classify_error("connection refused", 1) == ErrorClass.RETRYABLE_CONNECTION
        )

    def test_connection_error(self):
        assert classify_error("connection error", 1) == ErrorClass.RETRYABLE_CONNECTION

    def test_network_unreachable(self):
        assert (
            classify_error("network unreachable", 1) == ErrorClass.RETRYABLE_CONNECTION
        )

    def test_no_route_to_host(self):
        assert classify_error("no route to host", 1) == ErrorClass.RETRYABLE_CONNECTION

    def test_name_or_service_not_known(self):
        assert (
            classify_error("name or service not known", 1)
            == ErrorClass.RETRYABLE_CONNECTION
        )

    def test_unknown_error(self):
        assert (
            classify_error("something went completely wrong", 1) == ErrorClass.PERMANENT
        )

    def test_empty_stderr_nonzero_exit(self):
        assert classify_error("", 1) == ErrorClass.PERMANENT

    def test_case_insensitive(self):
        assert (
            classify_error("Rate Limit Exceeded", 0) == ErrorClass.RETRYABLE_RATE_LIMIT
        )

    def test_rate_limit_takes_priority_over_timeout(self):
        # If both patterns appear, rate limit should win (checked first)
        assert (
            classify_error("rate limit timed out", 0) == ErrorClass.RETRYABLE_RATE_LIMIT
        )


# ---------------------------------------------------------------------------
# is_retryable
# ---------------------------------------------------------------------------


class TestIsRetryable:
    def test_timeout_error_is_retryable(self):
        assert is_retryable(AITimeoutError("timeout")) is True

    def test_connection_error_is_retryable(self):
        assert is_retryable(AIConnectionError("conn")) is True

    def test_rate_limit_error_is_retryable(self):
        assert is_retryable(AIRateLimitError("rate")) is True

    def test_base_backend_error_not_retryable(self):
        assert is_retryable(AIBackendError("generic")) is False

    def test_subclass_hierarchy(self):
        # AITimeoutError is a subclass of AIBackendError but IS retryable
        err = AITimeoutError("t", stderr="timed out", exit_code=124)
        assert is_retryable(err) is True
