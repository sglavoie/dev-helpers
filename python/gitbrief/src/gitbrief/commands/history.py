"""The ``history`` command group."""

import click
from rich.console import Console
from rich.markdown import Markdown
from rich.panel import Panel

from gitbrief.clipboard import copy_to_clipboard
from gitbrief.formatters import VALID_FORMATS
from gitbrief.history import (
    clear_history,
    get_history_entry,
    list_history,
    parse_older_than,
)
from gitbrief.output import apply_format, build_period, emit


def _window(record: dict) -> str:
    """Human-readable time window of a history record."""
    since = record.get("since", "")
    until = record.get("until")
    return f"from {since} to {until}" if until else f"since {since}"


@click.group("history")
def history_group() -> None:
    """Manage summary history."""


@history_group.command("list")
def history_list() -> None:
    """List past summaries."""
    entries = list_history()
    if not entries:
        click.echo("No history found.")
        return
    for i, (stem, record) in enumerate(entries, 1):
        ts = record.get("timestamp", stem)
        projects = ", ".join(record.get("projects", []))
        count = record.get("commit_count", "?")
        click.echo(f"  {i:3d}  {ts}  [{projects}]  {count} commits")


@history_group.command("show")
@click.argument("id")
@click.option(
    "--format",
    "output_format",
    type=click.Choice(sorted(VALID_FORMATS), case_sensitive=False),
    default="markdown",
    help="Output format (default: markdown)",
)
@click.option("--output", "-o", "output_file", default=None, help="Write to file")
@click.option("--no-clipboard", is_flag=True, help="Do not copy to clipboard")
def history_show(
    id: str, output_format: str, output_file: str | None, no_clipboard: bool
) -> None:
    """Show a past summary by index or filename stem."""
    record = get_history_entry(id)
    if not record:
        raise click.ClickException(f"No history entry found for: {id!r}")

    ts = record.get("timestamp", "")
    projects = ", ".join(record.get("projects", []))
    metadata = {
        "generated_at": ts,
        "projects": record.get("projects", []),
        "period": build_period(record.get("since", ""), record.get("until")),
        "backend": record.get("backend", ""),
        "commit_count": record.get("commit_count", 0),
    }
    formatted = apply_format(record.get("summary", ""), output_format, metadata)

    title = f"[bold]{_window(record)}[/bold]  [{projects}]  {ts}"
    emit(formatted, output_format, output_file, title)

    if not no_clipboard:
        if copy_to_clipboard(formatted):
            click.echo("\nCopied to clipboard.", err=True)


@history_group.command("clear")
@click.option(
    "--older-than",
    "older_than",
    default=None,
    metavar="DURATION",
    help="Only clear entries older than this (e.g. 30d, 4w, 2m, 1y)",
)
@click.option("--yes", "-y", is_flag=True, help="Skip confirmation prompt")
def history_clear(older_than: str | None, yes: bool) -> None:
    """Delete history entries."""
    days: int | None = None
    if older_than:
        try:
            days = parse_older_than(older_than)
        except ValueError as e:
            raise click.ClickException(str(e))

    if not yes:
        suffix = f" older than {older_than}" if older_than else ""
        click.confirm(f"Delete history entries{suffix}?", abort=True)

    count = clear_history(days)
    click.echo(f"Deleted {count} history entr{'ies' if count != 1 else 'y'}.")


@history_group.command("diff")
@click.argument("id1")
@click.argument("id2")
def history_diff(id1: str, id2: str) -> None:
    """Compare two past summaries side by side."""
    r1 = get_history_entry(id1)
    r2 = get_history_entry(id2)
    if not r1:
        raise click.ClickException(f"No history entry found for: {id1!r}")
    if not r2:
        raise click.ClickException(f"No history entry found for: {id2!r}")

    console = Console()
    console.print(_make_panel(r1, "A"))
    console.rule()
    console.print(_make_panel(r2, "B"))


def _make_panel(record: dict, label: str) -> Panel:
    ts = record.get("timestamp", label)
    projects = ", ".join(record.get("projects", []))
    count = record.get("commit_count", "?")
    title = (
        f"[bold]{label}[/bold]: {_window(record)}  "
        f"[{projects}]  {count} commits  {ts}"
    )
    return Panel(Markdown(record.get("summary", "")), title=title, border_style="blue")
