"""Template loading and rendering for gitbrief prompt generation."""

from pathlib import Path

BUILT_IN_TEMPLATES: dict[str, str] = {
    "default": "Standard summary suitable for manager status updates.",
    "standup": "Ultra-brief 3-5 bullet points for daily standups.",
    "executive": "High-level strategic summary grouped by business impact.",
}


def _templates_dir() -> Path:
    """Return the path to the built-in templates directory."""
    return Path(__file__).parent / "templates"


def load_template(name_or_path: str) -> str:
    """Load a template by built-in name or file path.

    Raises FileNotFoundError if the template cannot be found.
    """
    p = Path(name_or_path)
    if p.exists() and p.is_file():
        return p.read_text()

    tfile = _templates_dir() / f"{name_or_path}.txt"
    if tfile.exists():
        return tfile.read_text()

    raise FileNotFoundError(f"Template not found: {name_or_path!r}")


def list_templates() -> list[dict]:
    """Return name and description for each built-in template."""
    return [{"name": name, "description": desc} for name, desc in BUILT_IN_TEMPLATES.items()]


def render_template(template: str, context: dict) -> str:
    """Substitute {{key}} placeholders in template with context values."""
    result = template
    for key, value in context.items():
        result = result.replace("{{" + key + "}}", value)
    return result
