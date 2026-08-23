"""Shared rendering helpers for commands that emit a formatted summary."""

import click
from rich.console import Console
from rich.markdown import Markdown
from rich.panel import Panel

from gitbrief.formatters import FORMATTERS, format_json

stderr_console = Console(stderr=True)

# Formats printed verbatim; anything else is rendered as markdown in a panel.
VERBATIM_FORMATS = ("plain", "slack", "json")


def build_period(since: str, until: str | None) -> dict:
    """Build the ``period`` block of the JSON output metadata."""
    period: dict = {"since": since}
    if until:
        period["until"] = until
    return period


def apply_format(summary: str, output_format: str, metadata: dict) -> str:
    """Render a summary in the requested output format.

    ``metadata`` is only consulted by the JSON formatter.
    """
    if output_format == "json":
        return format_json(summary, metadata)
    formatter = FORMATTERS[output_format]
    return formatter(summary)  # type: ignore[operator]


def emit(
    formatted: str, output_format: str, output_file: str | None, panel_title: str
) -> None:
    """Write a formatted summary to a file, to stdout, or into a Rich panel."""
    if output_file:
        with open(output_file, "w") as fh:
            fh.write(formatted)
            if not formatted.endswith("\n"):
                fh.write("\n")
        click.echo(f"Summary written to {output_file}", err=True)
    elif output_format in VERBATIM_FORMATS:
        click.echo(formatted)
    else:
        # markdown — render with Rich
        Console().print(
            Panel(Markdown(formatted), title=panel_title, border_style="blue")
        )
