"""The ``doctor`` health-check command."""

import datetime
import shutil

import click
from rich.console import Console
from rich.table import Table

from gitbrief.config import load_config
from gitbrief.git import extract_commits, validate_repo

KNOWN_SETTINGS = {"backend", "timeout", "retries", "max_commits"}


def _add_project_rows(table: Table, projects: dict) -> None:
    if not projects:
        table.add_row("Projects", "[yellow]WARN[/yellow]", "No projects registered")
        return
    far_past = (datetime.date.today() - datetime.timedelta(days=365 * 10)).isoformat()
    for alias, project in projects.items():
        path = project["path"]
        error = validate_repo(path)
        if error:
            table.add_row(f"Project: {alias}", "[red]FAIL[/red]", error)
        elif extract_commits(path, far_past, max_commits=1):
            table.add_row(f"Project: {alias}", "[green]OK[/green]", path)
        else:
            table.add_row(
                f"Project: {alias}",
                "[yellow]WARN[/yellow]",
                f"{path} (no commits found)",
            )


def _add_backend_rows(table: Table) -> None:
    for backend_name in ("claude", "copilot"):
        if shutil.which(backend_name):
            table.add_row(
                f"Backend: {backend_name}", "[green]OK[/green]", "Found in PATH"
            )
        else:
            table.add_row(
                f"Backend: {backend_name}", "[red]FAIL[/red]", "Not found in PATH"
            )


def _add_group_rows(table: Table, groups: dict, projects: dict) -> None:
    """Report groups that reference deleted projects."""
    for group_name, aliases in groups.items():
        stale = [a for a in aliases if a not in projects]
        if stale:
            table.add_row(
                f"Group: {group_name}",
                "[yellow]WARN[/yellow]",
                f"References missing projects: {', '.join(stale)}",
            )
        else:
            table.add_row(
                f"Group: {group_name}", "[green]OK[/green]", f"{len(aliases)} member(s)"
            )


def _add_config_row(table: Table, settings: dict) -> None:
    unknown = set(settings) - KNOWN_SETTINGS
    if unknown:
        table.add_row(
            "Config",
            "[yellow]WARN[/yellow]",
            f"Unknown/deprecated keys: {', '.join(sorted(unknown))}",
        )
    else:
        table.add_row("Config", "[green]OK[/green]", "No deprecated or invalid keys")


@click.command()
def doctor() -> None:
    """Check project health and AI backend availability."""
    config = load_config()
    projects = config.get("projects", {})

    table = Table(title="Gitbrief Health Check", show_header=True, header_style="bold")
    table.add_column("Check", style="bold")
    table.add_column("Status", justify="center")
    table.add_column("Details")

    _add_project_rows(table, projects)
    _add_backend_rows(table)
    _add_group_rows(table, config.get("groups", {}), projects)
    _add_config_row(table, config.get("settings", {}))

    Console().print(table)
