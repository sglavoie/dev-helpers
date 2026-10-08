"""Terminal tables, with plain output left to the calling reporter."""

from collections.abc import Sequence
from typing import Any

import click
from rich_click import RichGroup
from rich import box
from rich.console import Console
from rich.table import Table
from rich.text import Text


class TerminalGroup(RichGroup):
    """Keep redirected errors and their suggested shell commands unwrapped."""

    def main(self, *args: Any, **kwargs: Any) -> Any:
        console = Console(file=click.get_text_stream("stderr"))
        if console.is_terminal and not console.is_dumb_terminal:
            return super().main(*args, **kwargs)
        return click.Group.main(self, *args, **kwargs)


def print_terminal_table(
    title: str,
    headers: tuple[str, ...],
    rows: Sequence[Sequence[str]],
    *,
    styles: list[str] | None = None,
    caption: str | None = None,
) -> bool:
    """Render only to a capable terminal; never interpret data as markup."""
    console = Console(file=click.get_text_stream("stdout"), highlight=False)
    if not console.is_terminal or console.is_dumb_terminal:
        return False
    table = Table(
        title=Text(title, style="bold"),
        caption=Text(caption) if caption else None,
        box=box.ROUNDED,
        border_style="cyan",
        header_style="bold cyan",
        show_header=bool(headers),
        expand=True,
    )
    for header in headers:
        table.add_column(header)
    for index, row in enumerate(rows):
        table.add_row(
            *(Text(value) for value in row),
            style=styles[index] if styles else None,
        )
    console.print()
    console.print(table)
    return True
