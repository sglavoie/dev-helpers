def build_summary_prompt(
    project_commits: dict[str, list[dict]],
    time_description: str,
    per_project_windows: dict[str, str] | None = None,
) -> str:
    """Build a prompt for the AI to summarize git activity.

    per_project_windows: optional mapping of alias -> "since YYYY-MM-DD" used when
    different projects cover different time windows (e.g. --since-last mode).
    """
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
        "",
    ]

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
