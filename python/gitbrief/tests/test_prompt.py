"""Tests for gitbrief.prompt module."""

from gitbrief.prompt import build_summary_prompt


def _commit(sha="abc12345", subject="feat: add thing", **kwargs):
    base = {"sha": sha, "subject": subject, "body": "", "refs": []}
    base.update(kwargs)
    return base


class TestBuildSummaryPrompt:
    def test_single_project_appears_in_output(self):
        commits = [_commit()]
        prompt = build_summary_prompt({"myrepo": commits}, "the last 1w")
        assert "myrepo" in prompt
        assert "the last 1w" in prompt

    def test_commit_sha_truncated_to_8(self):
        commits = [_commit(sha="abcdef1234567890" * 2)]
        prompt = build_summary_prompt({"proj": commits}, "today")
        assert "abcdef12" in prompt

    def test_commit_subject_in_output(self):
        commits = [_commit(subject="fix: correct the bug")]
        prompt = build_summary_prompt({"proj": commits}, "today")
        assert "fix: correct the bug" in prompt

    def test_body_included_when_present(self):
        commits = [_commit(body="This fixes the login issue")]
        prompt = build_summary_prompt({"proj": commits}, "today")
        assert "This fixes the login issue" in prompt

    def test_body_omitted_when_empty(self):
        commits = [_commit(body="")]
        prompt = build_summary_prompt({"proj": commits}, "today")
        # body line should not appear
        assert "  \n" not in prompt  # no blank body indented line

    def test_stats_included_when_present(self):
        commits = [_commit(files_changed=3, insertions=10, deletions=2)]
        prompt = build_summary_prompt({"proj": commits}, "today")
        assert "+10" in prompt
        assert "-2" in prompt
        assert "3 files" in prompt

    def test_stats_singular_file(self):
        commits = [_commit(files_changed=1, insertions=1, deletions=0)]
        prompt = build_summary_prompt({"proj": commits}, "today")
        assert "1 file," in prompt

    def test_branch_included_when_present(self):
        commits = [_commit(branch="feature/login")]
        prompt = build_summary_prompt({"proj": commits}, "today")
        assert "feature/login" in prompt

    def test_branch_omitted_when_absent(self):
        commits = [_commit()]
        prompt = build_summary_prompt({"proj": commits}, "today")
        assert "branch:" not in prompt

    def test_refs_included_when_present(self):
        commits = [_commit(refs=["#42", "#99"])]
        prompt = build_summary_prompt({"proj": commits}, "today")
        assert "#42" in prompt
        assert "#99" in prompt

    def test_refs_omitted_when_empty(self):
        commits = [_commit(refs=[])]
        prompt = build_summary_prompt({"proj": commits}, "today")
        assert "refs:" not in prompt

    def test_multi_project(self):
        commits_a = [_commit(sha="aaaa1234", subject="feat: a")]
        commits_b = [_commit(sha="bbbb5678", subject="fix: b")]
        prompt = build_summary_prompt(
            {"alpha": commits_a, "beta": commits_b}, "last-week"
        )
        assert "alpha" in prompt
        assert "beta" in prompt
        assert "feat: a" in prompt
        assert "fix: b" in prompt
