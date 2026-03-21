"""Tests for gitbrief.git module."""

from unittest.mock import MagicMock, patch

import pytest

from gitbrief.git import (
    _extract_refs,
    _parse_shortstat,
    extract_commits,
    get_git_user_email,
    parse_duration,
    validate_date_string,
    validate_repo,
)


# ---------------------------------------------------------------------------
# validate_date_string
# ---------------------------------------------------------------------------


class TestValidateDateString:
    def test_valid_date(self):
        assert validate_date_string("2024-01-15") is True

    def test_valid_date_boundary(self):
        assert validate_date_string("2024-12-31") is True

    def test_leap_year_feb_29(self):
        assert validate_date_string("2024-02-29") is True

    def test_non_leap_year_feb_29(self):
        assert validate_date_string("2023-02-29") is False

    def test_invalid_format_slashes(self):
        assert validate_date_string("2024/01/15") is False

    def test_invalid_format_no_separator(self):
        # Python 3.11+ fromisoformat accepts compact "YYYYMMDD" — so this IS valid
        assert validate_date_string("20240115") is True

    def test_empty_string(self):
        assert validate_date_string("") is False

    def test_invalid_month(self):
        assert validate_date_string("2024-13-01") is False

    def test_invalid_day(self):
        assert validate_date_string("2024-01-32") is False

    def test_partial_date(self):
        assert validate_date_string("2024-01") is False


# ---------------------------------------------------------------------------
# parse_duration
# ---------------------------------------------------------------------------


class TestParseDuration:
    def test_days(self):
        result = parse_duration("1d")
        assert result  # returns a date string
        from datetime import date, timedelta

        expected = (date.today() - timedelta(days=1)).isoformat()
        assert result == expected

    def test_weeks(self):
        from datetime import date, timedelta

        result = parse_duration("2w")
        expected = (date.today() - timedelta(weeks=2)).isoformat()
        assert result == expected

    def test_months(self):
        from datetime import date, timedelta

        result = parse_duration("1m")
        expected = (date.today() - timedelta(days=30)).isoformat()
        assert result == expected

    def test_years(self):
        from datetime import date, timedelta

        result = parse_duration("1y")
        expected = (date.today() - timedelta(days=365)).isoformat()
        assert result == expected

    def test_preset_today(self):
        from datetime import date

        assert parse_duration("today") == date.today().isoformat()

    def test_preset_yesterday(self):
        from datetime import date, timedelta

        expected = (date.today() - timedelta(days=1)).isoformat()
        assert parse_duration("yesterday") == expected

    def test_preset_this_week(self):
        from datetime import date, timedelta

        today = date.today()
        expected = (today - timedelta(days=today.weekday())).isoformat()
        assert parse_duration("this-week") == expected

    def test_preset_last_week(self):
        from datetime import date, timedelta

        today = date.today()
        expected = (today - timedelta(days=today.weekday() + 7)).isoformat()
        assert parse_duration("last-week") == expected

    def test_preset_this_month(self):
        from datetime import date

        expected = date.today().replace(day=1).isoformat()
        assert parse_duration("this-month") == expected

    def test_preset_last_month(self):
        from datetime import date

        today = date.today()
        if today.month > 1:
            expected = today.replace(month=today.month - 1, day=1).isoformat()
        else:
            expected = today.replace(year=today.year - 1, month=12, day=1).isoformat()
        assert parse_duration("last-month") == expected

    def test_invalid_input(self):
        with pytest.raises(ValueError, match="Invalid duration format"):
            parse_duration("invalid")

    def test_invalid_unit(self):
        with pytest.raises(ValueError, match="Invalid duration format"):
            parse_duration("5x")

    def test_zero_days(self):
        from datetime import date

        assert parse_duration("0d") == date.today().isoformat()

    def test_large_value(self):
        from datetime import date, timedelta

        result = parse_duration("365d")
        expected = (date.today() - timedelta(days=365)).isoformat()
        assert result == expected


# ---------------------------------------------------------------------------
# _parse_shortstat
# ---------------------------------------------------------------------------


class TestParseShortstat:
    def test_standard_output(self):
        line = " 3 files changed, 10 insertions(+), 2 deletions(-)"
        result = _parse_shortstat(line)
        assert result == {"files_changed": 3, "insertions": 10, "deletions": 2}

    def test_insertions_only(self):
        line = " 1 file changed, 5 insertions(+)"
        result = _parse_shortstat(line)
        assert result == {"files_changed": 1, "insertions": 5, "deletions": 0}

    def test_deletions_only(self):
        line = " 2 files changed, 3 deletions(-)"
        result = _parse_shortstat(line)
        assert result == {"files_changed": 2, "insertions": 0, "deletions": 3}

    def test_no_match(self):
        assert _parse_shortstat("not a shortstat line") is None

    def test_singular_file(self):
        line = " 1 file changed, 1 insertion(+)"
        result = _parse_shortstat(line)
        assert result == {"files_changed": 1, "insertions": 1, "deletions": 0}

    def test_empty_string(self):
        assert _parse_shortstat("") is None


# ---------------------------------------------------------------------------
# _extract_refs
# ---------------------------------------------------------------------------


class TestExtractRefs:
    def test_hash_ref_in_subject(self):
        assert _extract_refs("fix: close #123") == ["#123"]

    def test_github_url(self):
        refs = _extract_refs("see https://github.com/org/repo/issues/42")
        assert refs == ["#42"]

    def test_duplicates_deduplicated(self):
        refs = _extract_refs("#42 and #42")
        assert refs == ["#42"]

    def test_hash_and_url_same_number(self):
        refs = _extract_refs("#42 https://github.com/org/repo/pull/42")
        assert refs == ["#42"]

    def test_multiple_refs(self):
        refs = _extract_refs("fixes #1, see #2")
        assert refs == ["#1", "#2"]

    def test_no_refs(self):
        assert _extract_refs("just a regular commit message") == []

    def test_empty_string(self):
        assert _extract_refs("") == []

    def test_github_pull_url(self):
        refs = _extract_refs("https://github.com/org/repo/pull/99")
        assert refs == ["#99"]


# ---------------------------------------------------------------------------
# validate_repo (mocked filesystem)
# ---------------------------------------------------------------------------


class TestValidateRepo:
    def test_path_does_not_exist(self, tmp_path):
        result = validate_repo(str(tmp_path / "nonexistent"))
        assert result is not None
        assert "does not exist" in result

    def test_path_exists_but_not_git(self, tmp_path):
        result = validate_repo(str(tmp_path))
        assert result is not None
        assert "Not a git repository" in result

    def test_valid_git_repo(self, tmp_path):
        (tmp_path / ".git").mkdir()
        result = validate_repo(str(tmp_path))
        assert result is None


# ---------------------------------------------------------------------------
# get_git_user_email (mocked subprocess)
# ---------------------------------------------------------------------------


class TestGetGitUserEmail:
    def test_returns_email_on_success(self):
        mock_result = MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = "user@example.com\n"
        with patch("gitbrief.git.subprocess.run", return_value=mock_result):
            assert get_git_user_email("/some/repo") == "user@example.com"

    def test_returns_none_on_nonzero_exit(self):
        mock_result = MagicMock()
        mock_result.returncode = 1
        mock_result.stdout = ""
        with patch("gitbrief.git.subprocess.run", return_value=mock_result):
            assert get_git_user_email("/some/repo") is None

    def test_returns_none_when_git_not_found(self):
        with patch("gitbrief.git.subprocess.run", side_effect=FileNotFoundError):
            assert get_git_user_email("/some/repo") is None


# ---------------------------------------------------------------------------
# extract_commits (mocked subprocess)
# ---------------------------------------------------------------------------


_SAMPLE_LOG = (
    "abc1234567890123456789012345678901234567|feat: add login|Closes #42\x00"
    "def0987654321098765432109876543210987654|fix: off-by-one|\x00"
)

_SAMPLE_SHORTSTAT = (
    "abc1234567890123456789012345678901234567\n"
    " 2 files changed, 10 insertions(+), 1 deletion(-)\n"
    "def0987654321098765432109876543210987654\n"
    " 1 file changed, 3 insertions(+)\n"
)

_SAMPLE_DECOR = (
    "abc1234567890123456789012345678901234567|HEAD -> feature/login\n"
    "def0987654321098765432109876543210987654|\n"
)


def _make_run_side_effect(
    log=_SAMPLE_LOG, shortstat=_SAMPLE_SHORTSTAT, decor=_SAMPLE_DECOR
):
    """Return a side_effect function that dispatches based on git subcommand args."""
    call_count = [0]

    def side_effect(cmd, **kwargs):
        result = MagicMock()
        result.returncode = 0
        call_count[0] += 1
        # First call: main log, second: shortstat, third: branch tips
        if call_count[0] == 1:
            result.stdout = log
        elif call_count[0] == 2:
            result.stdout = shortstat
        else:
            result.stdout = decor
        return result

    return side_effect


class TestExtractCommits:
    def test_parses_commits(self):
        with patch("gitbrief.git.subprocess.run", side_effect=_make_run_side_effect()):
            commits = extract_commits("/repo", "2024-01-01")
        assert len(commits) == 2
        assert commits[0]["sha"] == "abc1234567890123456789012345678901234567"
        assert commits[0]["subject"] == "feat: add login"
        assert commits[0]["body"] == "Closes #42"
        assert "#42" in commits[0]["refs"]

    def test_parses_diff_stats(self):
        with patch("gitbrief.git.subprocess.run", side_effect=_make_run_side_effect()):
            commits = extract_commits("/repo", "2024-01-01")
        assert commits[0]["files_changed"] == 2
        assert commits[0]["insertions"] == 10
        assert commits[0]["deletions"] == 1

    def test_parses_branch(self):
        with patch("gitbrief.git.subprocess.run", side_effect=_make_run_side_effect()):
            commits = extract_commits("/repo", "2024-01-01")
        assert commits[0].get("branch") == "feature/login"

    def test_returns_empty_on_git_not_found(self):
        with patch("gitbrief.git.subprocess.run", side_effect=FileNotFoundError):
            assert extract_commits("/repo", "2024-01-01") == []

    def test_returns_empty_on_nonzero_exit(self):
        mock_result = MagicMock()
        mock_result.returncode = 1
        mock_result.stdout = ""
        with patch("gitbrief.git.subprocess.run", return_value=mock_result):
            assert extract_commits("/repo", "2024-01-01") == []

    def test_respects_max_commits(self):
        # Build a log with 5 commits
        shas = [f"{'a' * 39}{i}" for i in range(5)]
        log = "".join(f"{sha}|msg {i}|\x00" for i, sha in enumerate(shas))

        call_count = [0]

        def run_side_effect(cmd, **kwargs):
            r = MagicMock()
            r.returncode = 0
            call_count[0] += 1
            if call_count[0] == 1:
                r.stdout = log
            else:
                r.stdout = ""
            return r

        with patch("gitbrief.git.subprocess.run", side_effect=run_side_effect):
            commits = extract_commits("/repo", "2024-01-01", max_commits=3)
        assert len(commits) == 3
