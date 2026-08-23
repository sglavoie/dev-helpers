"""Commands for registering and listing git repositories."""

import click

from gitbrief.commands.completion import complete_project_alias
from gitbrief.config import add_project, load_config, remove_project
from gitbrief.git import validate_repo


@click.command()
@click.argument("alias")
@click.argument("path")
@click.option(
    "--backend", default=None, help="AI backend for this project (claude, copilot)"
)
def add(alias: str, path: str, backend: str | None) -> None:
    """Register a git repository with an alias."""
    add_project(alias, path, backend=backend)
    msg = f"Added '{alias}' -> {path}"
    if backend:
        msg += f" (backend: {backend})"
    click.echo(msg)


@click.command()
@click.argument("alias", shell_complete=complete_project_alias)
def remove(alias: str) -> None:
    """Remove a registered repository."""
    remove_project(alias)
    click.echo(f"Removed '{alias}'")


@click.command("list")
def list_projects() -> None:
    """List registered repositories."""
    config = load_config()
    projects = config.get("projects", {})
    if not projects:
        click.echo(
            "No projects registered. Use 'gitbrief add <alias> <path>' to add one."
        )
        return
    for alias, project in projects.items():
        path = project["path"]
        backend = project.get("backend")
        error = validate_repo(path)
        warning = f"  [WARNING: {error}]" if error else ""
        backend_tag = f"  [{backend}]" if backend else ""
        click.echo(f"  {alias:20s} {path}{backend_tag}{warning}")
