"""Commit extraction for the ``summary`` command, with optional progress bar."""

from collections.abc import Callable

import click
from rich.console import Console
from rich.progress import BarColumn, MofNCompleteColumn, Progress, TextColumn

from gitbrief.git import extract_commits, get_git_user_email, validate_repo

# Reporter signature: a warning line destined for stderr.
Warn = Callable[[str], None]


def collect_commits(
    selected: dict,
    per_project_since: dict[str, str],
    *,
    all_authors: bool,
    until: str | None,
    max_commits: int,
    warn: Warn,
    advance: Callable[[str], None],
) -> tuple[dict[str, list[dict]], list[str]]:
    """Extract commits per project, returning them plus the truncated aliases."""
    project_commits: dict[str, list[dict]] = {}
    truncated: list[str] = []

    for alias, project in selected.items():
        advance(alias)
        path = project["path"]
        error = validate_repo(path)
        if error:
            warn(f"Warning: Skipping '{alias}': {error}")
            continue

        author = None
        if not all_authors:
            author = get_git_user_email(path)
            if not author:
                warn(
                    f"Warning: No git user.email set for '{alias}', showing all authors."
                )

        commits = extract_commits(
            path, per_project_since[alias], author, until, max_commits=max_commits
        )
        if commits:
            project_commits[alias] = commits
            if len(commits) >= max_commits:
                truncated.append(alias)

    return project_commits, truncated


def collect_with_progress(
    selected: dict, per_project_since: dict[str, str], **kwargs
) -> tuple[dict[str, list[dict]], list[str]]:
    """Run collect_commits behind a progress bar when several projects are selected."""
    if len(selected) <= 1:
        return collect_commits(
            selected,
            per_project_since,
            warn=lambda msg: click.echo(msg, err=True),
            advance=lambda _alias: None,
            **kwargs,
        )

    with Progress(
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        MofNCompleteColumn(),
        TextColumn(" projects  [cyan]\\[{task.fields[current]}][/cyan]"),
        console=Console(stderr=True),
        transient=True,
    ) as progress:
        task = progress.add_task("Extracting commits", total=len(selected), current="")
        started = False

        def advance(alias: str) -> None:
            # The bar counts finished projects, so a step lands when the next
            # project starts and a final one lands after the loop.
            nonlocal started
            if started:
                progress.advance(task)
            started = True
            progress.update(task, current=alias)

        result = collect_commits(
            selected,
            per_project_since,
            warn=lambda msg: progress.console.print(msg, style="yellow"),
            advance=advance,
            **kwargs,
        )
        progress.advance(task)
        return result
