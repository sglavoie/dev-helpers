"""The ``scan`` repository-discovery command."""

import re

import click
from rich.console import Console
from rich.table import Table

from gitbrief.config import load_config, save_config
from gitbrief.git import discover_repos
from gitbrief.output import stderr_console


def _resolve_alias(name: str, existing: dict) -> str:
    """Return name if not already an alias, otherwise name-2, name-3, etc."""
    if name not in existing:
        return name
    n = 2
    while f"{name}-{n}" in existing:
        n += 1
    return f"{name}-{n}"


def _register_repos(repos: list[dict], config: dict, console: Console) -> None:
    """Add repos to config (in-place), persist, and report how many were added."""
    for repo in repos:
        alias = _resolve_alias(repo["name"], config["projects"])
        config["projects"][alias] = {"path": repo["path"], "backend": None}
        console.print(f"  Added [bold]{alias}[/bold] → {repo['path']}")
    save_config(config)
    added = len(repos)
    click.echo(f"\nAdded {added} new repositor{'y' if added == 1 else 'ies'}.")


def _discovery_table(directory: str, repos: list[dict]) -> Table:
    table = Table(
        title=f"Discovered Repositories: {directory}",
        show_header=True,
        header_style="bold",
    )
    table.add_column("#", justify="right", style="dim")
    table.add_column("Name")
    table.add_column("Path")
    table.add_column("Last Commit")
    table.add_column("Status")
    for i, repo in enumerate(repos, 1):
        status_cell = (
            "[green]registered[/green]"
            if repo["status"] == "registered"
            else "[blue]new[/blue]"
        )
        table.add_row(
            str(i), repo["name"], repo["path"], repo["last_commit"] or "—", status_cell
        )
    return table


def _prompt_selection(new_repos: list[dict]) -> list[dict] | None:
    """Ask for indices into new_repos and return the repos they name.

    Returns None when nothing was entered at all, which the caller reports
    differently from an entry that named no valid repository.
    """
    click.echo("\nNew repositories:")
    for i, repo in enumerate(new_repos, 1):
        click.echo(f"  {i:3d}  {repo['name']:30s}  {repo['path']}")
    raw = click.prompt(
        "\nEnter numbers to register (space- or comma-separated)", default=""
    ).strip()
    if not raw:
        return None

    selected: list[dict] = []
    for token in re.split(r"[\s,]+", raw):
        token = token.strip()
        if not token:
            continue
        try:
            idx = int(token) - 1
        except ValueError:
            stderr_console.print(
                f"[yellow]Warning:[/yellow] Ignoring invalid selection: {token!r}"
            )
            continue
        if idx < 0 or idx >= len(new_repos):
            stderr_console.print(
                f"[yellow]Warning:[/yellow] Index {token} out of range, skipping."
            )
            continue
        repo = new_repos[idx]
        if repo not in selected:
            selected.append(repo)
    return selected


@click.command()
@click.argument(
    "directory", type=click.Path(exists=True, file_okay=False, resolve_path=True)
)
@click.option(
    "--depth", default=3, show_default=True, help="Max directory depth to scan"
)
@click.option("--auto", is_flag=True, help="Register all new repos without prompting")
def scan(directory: str, depth: int, auto: bool) -> None:
    """Discover git repositories under DIRECTORY and register them."""
    console = Console()
    config = load_config()
    registered_paths = {p["path"] for p in config.get("projects", {}).values()}

    repos = discover_repos(directory, max_depth=depth)
    if not repos:
        click.echo(f"No git repositories found under {directory} (depth={depth}).")
        return

    for repo in repos:
        repo["status"] = "registered" if repo["path"] in registered_paths else "new"
    console.print(_discovery_table(directory, repos))

    new_repos = [r for r in repos if r["status"] == "new"]
    if not new_repos:
        click.echo("All discovered repositories are already registered.")
        return

    if auto:
        _register_repos(new_repos, config, console)
        return

    count = len(new_repos)
    response = (
        click.prompt(
            f"\nFound {count} new repositor{'y' if count == 1 else 'ies'}."
            " Add them? [y/N/select]",
            default="N",
        )
        .strip()
        .lower()
    )

    if response == "y":
        _register_repos(new_repos, config, console)
    elif response == "select":
        selected = _prompt_selection(new_repos)
        if selected is None:
            click.echo("No repositories selected.")
        elif not selected:
            click.echo("No valid repositories selected.")
        else:
            _register_repos(selected, config, console)
    else:
        click.echo("No repositories added.")
