DETAIL_INSTRUCTIONS: dict[str, str] = {
    "brief": (
        "Produce a maximum of 3-5 bullet points total. Be extremely concise. "
        "No commit SHAs. No diff statistics. Focus only on the most significant changes."
    ),
    "normal": "",
    "detailed": (
        "Provide detailed commentary for each significant commit or group of "
        "related commits. Include SHAs, diff statistics, branch context, and explain the "
        "purpose and impact of each change."
    ),
}


def _format_commits(
    project_commits: dict[str, list[dict]],
    per_project_windows: dict[str, str] | None = None,
) -> str:
    """Format commit data for all projects into text."""
    lines: list[str] = []
    for project, commits in project_commits.items():
        header = f"## Project: {project}"
        if per_project_windows and project in per_project_windows:
            header += f"  ({per_project_windows[project]})"
        lines.append(header)
        lines.append("")
        for c in commits:
            line = f"- {c['sha'][:8]} {c['subject']}"
            if c.get("body"):
                line += f"\n  {c['body']}"
            if "files_changed" in c:
                fc = c["files_changed"]
                ins = c.get("insertions", 0)
                dels = c.get("deletions", 0)
                line += f"\n  [{fc} file{'s' if fc != 1 else ''}, +{ins}/-{dels}]"
            if c.get("branch"):
                line += f"\n  (branch: {c['branch']})"
            if c.get("refs"):
                line += f"\n  refs: {', '.join(c['refs'])}"
            lines.append(line)
        lines.append("")
    return "\n".join(lines)


def build_summary_prompt(
    project_commits: dict[str, list[dict]],
    time_description: str,
    per_project_windows: dict[str, str] | None = None,
    detail: str = "normal",
    template: str = "default",
) -> str:
    """Build a prompt for the AI to summarize git activity.

    per_project_windows: optional mapping of alias -> "since YYYY-MM-DD" used when
    different projects cover different time windows (e.g. --since-last mode).
    detail: one of "brief", "normal", "detailed".
    template: built-in template name or path to a custom template file.
    """
    from gitbrief.templates import load_template, render_template

    commits_text = _format_commits(project_commits, per_project_windows)
    projects_text = ", ".join(project_commits.keys())
    detail_instr = DETAIL_INSTRUCTIONS.get(detail, "")
    detail_instructions_text = detail_instr  # empty string for "normal"

    try:
        tmpl = load_template(template)
    except FileNotFoundError:
        return _fallback_prompt(
            project_commits, time_description, per_project_windows, detail_instr
        )

    context = {
        "commits": commits_text,
        "projects": projects_text,
        "period": time_description,
        "detail_instructions": detail_instructions_text,
    }
    return render_template(tmpl, context)


def _fallback_prompt(
    project_commits: dict[str, list[dict]],
    time_description: str,
    per_project_windows: dict[str, str] | None = None,
    detail_instr: str = "",
) -> str:
    """Hardcoded fallback if template files are not available."""
    lines = [
        f"Summarize the following git activity from {time_description}.",
        "",
        "Instructions:",
        "- Produce concise bullet points grouped by theme (e.g., Features, Fixes, Refactoring, Docs).",
        "- Include short commit SHAs (first 8 characters) in parentheses after each bullet.",
        "- When multiple projects are included, note which project each item belongs to.",
        "- Omit merge commits and trivial changes unless they are the only activity.",
        "- Keep the summary brief and suitable for a status update to a manager.",
        "- Use diff statistics to highlight the scale of changes. Note branch context and PR/issue references when relevant.",
    ]
    if detail_instr:
        lines.append(detail_instr)
    lines.append("")
    lines.append(_format_commits(project_commits, per_project_windows))
    return "\n".join(lines)
