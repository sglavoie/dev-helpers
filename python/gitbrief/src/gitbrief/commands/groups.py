"""The ``group`` command group."""

import click
from rich.console import Console
from rich.table import Table

from gitbrief.config import (
    add_to_group,
    create_group,
    delete_group,
    list_groups,
    load_config,
    remove_from_group,
    save_config,
)


@click.group("group")
def group_group() -> None:
    """Manage project groups."""


@group_group.command("create")
@click.argument("name")
@click.argument("aliases", nargs=-1, required=True)
def group_create(name: str, aliases: tuple[str, ...]) -> None:
    """Create a new group with the given project aliases."""
    config = load_config()
    create_group(config, name, list(aliases))
    save_config(config)
    click.echo(f"Created group '{name}' with: {', '.join(aliases)}")


@group_group.command("delete")
@click.argument("name")
def group_delete(name: str) -> None:
    """Delete a group."""
    config = load_config()
    delete_group(config, name)
    save_config(config)
    click.echo(f"Deleted group '{name}'.")


@group_group.command("add")
@click.argument("name")
@click.argument("alias")
def group_add(name: str, alias: str) -> None:
    """Add a project alias to a group."""
    config = load_config()
    add_to_group(config, name, alias)
    save_config(config)
    click.echo(f"Added '{alias}' to group '{name}'.")


@group_group.command("remove")
@click.argument("name")
@click.argument("alias")
def group_remove(name: str, alias: str) -> None:
    """Remove a project alias from a group."""
    config = load_config()
    remove_from_group(config, name, alias)
    save_config(config)
    click.echo(f"Removed '{alias}' from group '{name}'.")


@group_group.command("list")
def group_list() -> None:
    """List all groups and their members."""
    config = load_config()
    groups = list_groups(config)
    if not groups:
        click.echo("No groups defined. Use 'gitbrief group create <name> <aliases...>'.")
        return
    console = Console()
    table = Table(title="Project Groups", show_header=True, header_style="bold")
    table.add_column("Group")
    table.add_column("Members")
    for group_name, aliases in sorted(groups.items()):
        table.add_row(group_name, ", ".join(aliases) if aliases else "(empty)")
    console.print(table)
