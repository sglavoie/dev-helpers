"""The ``template`` command group."""

import click
from rich.console import Console
from rich.table import Table

from gitbrief.templates import list_templates, load_template

TEMPLATE_NOT_FOUND_HINT = "Use 'gitbrief template list' to see available templates."


@click.group("template")
def template_group() -> None:
    """Manage prompt templates."""


@template_group.command("list")
def template_list() -> None:
    """List available built-in templates."""
    templates = list_templates()
    console = Console()
    table = Table(title="Available Templates", show_header=True, header_style="bold")
    table.add_column("Name")
    table.add_column("Description")
    for t in templates:
        table.add_row(t["name"], t["description"])
    console.print(table)


@template_group.command("show")
@click.argument("name")
def template_show(name: str) -> None:
    """Print the content of a template."""
    try:
        content = load_template(name)
    except FileNotFoundError:
        raise click.ClickException(
            f"Template not found: {name!r}. {TEMPLATE_NOT_FOUND_HINT}"
        )
    click.echo(content)
