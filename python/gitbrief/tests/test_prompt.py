"""Tests for gitbrief.prompt module."""

from gitbrief.prompt import DETAIL_INSTRUCTIONS, build_summary_prompt


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

    def test_per_project_windows_included(self):
        commits = [_commit()]
        prompt = build_summary_prompt(
            {"proj": commits},
            "varies",
            per_project_windows={"proj": "since 2026-01-01"},
        )
        assert "since 2026-01-01" in prompt


class TestDetailInstructions:
    def test_detail_instructions_dict_has_three_levels(self):
        assert set(DETAIL_INSTRUCTIONS.keys()) == {"brief", "normal", "detailed"}

    def test_normal_is_empty(self):
        assert DETAIL_INSTRUCTIONS["normal"] == ""

    def test_brief_mentions_bullet_points(self):
        assert "3-5 bullet points" in DETAIL_INSTRUCTIONS["brief"]

    def test_detailed_mentions_shas(self):
        assert "SHAs" in DETAIL_INSTRUCTIONS["detailed"]


class TestBuildSummaryPromptDetail:
    def test_default_detail_is_normal(self):
        commits = [_commit()]
        prompt = build_summary_prompt({"proj": commits}, "today")
        # normal detail → no extra instruction injected
        assert "3-5 bullet points" not in prompt
        assert "detailed commentary" not in prompt.lower()

    def test_brief_detail_injects_instructions(self):
        commits = [_commit()]
        prompt = build_summary_prompt({"proj": commits}, "today", detail="brief")
        assert "3-5 bullet points" in prompt

    def test_detailed_detail_injects_instructions(self):
        commits = [_commit()]
        prompt = build_summary_prompt({"proj": commits}, "today", detail="detailed")
        assert "SHAs" in prompt

    def test_normal_detail_no_extra_text(self):
        commits = [_commit()]
        prompt = build_summary_prompt({"proj": commits}, "today", detail="normal")
        assert "3-5 bullet points" not in prompt

    def test_commits_always_included_regardless_of_detail(self):
        commits = [_commit(subject="fix: critical bug")]
        for level in ("brief", "normal", "detailed"):
            prompt = build_summary_prompt({"proj": commits}, "today", detail=level)
            assert "fix: critical bug" in prompt

    def test_period_always_included(self):
        commits = [_commit()]
        for level in ("brief", "normal", "detailed"):
            prompt = build_summary_prompt({"proj": commits}, "last 7 days", detail=level)
            assert "last 7 days" in prompt


class TestBuildSummaryPromptTemplate:
    def test_default_template_used_by_default(self):
        commits = [_commit()]
        prompt = build_summary_prompt({"proj": commits}, "today")
        # default template has manager-oriented language
        assert "manager" in prompt or "status update" in prompt

    def test_standup_template_used(self):
        commits = [_commit()]
        prompt = build_summary_prompt({"proj": commits}, "today", template="standup")
        assert "bullet points" in prompt

    def test_executive_template_used(self):
        commits = [_commit()]
        prompt = build_summary_prompt({"proj": commits}, "today", template="executive")
        assert "strategic" in prompt

    def test_nonexistent_template_falls_back(self):
        commits = [_commit()]
        # Should not raise; falls back to hardcoded prompt
        prompt = build_summary_prompt(
            {"proj": commits}, "today", template="no_such_template_xyz"
        )
        assert "proj" in prompt
        assert len(prompt) > 0

    def test_custom_template_file(self, tmp_path):
        custom = tmp_path / "custom.txt"
        custom.write_text("CUSTOM: {{period}} | {{commits}}")
        commits = [_commit(subject="feat: custom")]
        prompt = build_summary_prompt(
            {"proj": commits}, "this-week", template=str(custom)
        )
        assert "CUSTOM:" in prompt
        assert "this-week" in prompt
        assert "feat: custom" in prompt
