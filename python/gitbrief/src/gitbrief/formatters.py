"""Output formatters for gitbrief summaries."""

import json
import re
from datetime import datetime

VALID_FORMATS = {"markdown", "slack", "json", "plain"}


def format_markdown(summary: str) -> str:
    """Pass-through — keep AI output as-is (markdown)."""
    return summary


def format_slack(summary: str) -> str:
    """Convert markdown to Slack mrkdwn format."""
    # **bold** → *bold*
    text = re.sub(r"\*\*(.+?)\*\*", r"*\1*", summary)
    # # heading → *heading*
    text = re.sub(r"^#{1,6}\s+(.+)$", r"*\1*", text, flags=re.MULTILINE)
    # Code blocks (triple backticks) stay as-is — Slack supports them
    return text


def format_json(summary: str, metadata: dict) -> str:
    """Wrap summary in a JSON structure with metadata."""
    data = {
        "generated_at": metadata.get(
            "generated_at", datetime.now().isoformat(timespec="seconds")
        ),
        "projects": metadata.get("projects", []),
        "period": metadata.get("period", {}),
        "backend": metadata.get("backend", "claude"),
        "summary": summary,
        "commit_count": metadata.get("commit_count", 0),
    }
    return json.dumps(data, indent=2)


def format_plain(summary: str) -> str:
    """Strip markdown formatting to produce plain text."""
    # Remove code fences
    text = re.sub(r"```[^\n]*\n([\s\S]*?)```", r"\1", summary)
    # Remove inline code backticks
    text = re.sub(r"`(.+?)`", r"\1", text)
    # Remove **bold** and __bold__
    text = re.sub(r"\*\*(.+?)\*\*", r"\1", text)
    text = re.sub(r"__(.+?)__", r"\1", text)
    # Remove *italic* and _italic_
    text = re.sub(r"\*(.+?)\*", r"\1", text)
    text = re.sub(r"_(.+?)_", r"\1", text)
    # Remove markdown headings
    text = re.sub(r"^#{1,6}\s+", "", text, flags=re.MULTILINE)
    return text


# Formatters that take only (summary: str) -> str.
# format_json requires an additional metadata dict; call it directly.
FORMATTERS: dict[str, object] = {
    "markdown": format_markdown,
    "slack": format_slack,
    "json": format_json,
    "plain": format_plain,
}
