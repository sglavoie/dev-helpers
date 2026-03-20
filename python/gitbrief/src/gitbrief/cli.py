import click
from rich.console import Console

from gitbrief.ai import invoke_ai
from gitbrief.clipboard import copy_to_clipboard
from gitbrief.config import (
    add_project,
    get_setting,
    get_setting_int,
    load_config,
    remove_project,
    set_setting,
)
from gitbrief.exceptions import AIBackendError
from gitbrief.git import (
    extract_commits,
    get_git_user_email,
    parse_duration,
    validate_date_string,
    validate_repo,
)
from gitbrief.prompt import build_summary_prompt


@click.group()
def cli() -> None:
    """AI-powered git activity summarizer."""


@cli.command()
@click.argument("alias")
@click.argument("path")
@click.option("--backend", default=None, help="AI backend for this project (claude, copilot)")
def add(alias: str, path: str, backend: str | None) -> None:
    """Register a git repository with an alias."""
    add_project(alias, path, backend=backend)
    msg = f"Added '{alias}' -> {path}"
    if backend:
        msg += f" (backend: {backend})"
    click.echo(msg)


@cli.command()
@click.argument("alias")
def remove(alias: str) -> None:
    """Remove a registered repository."""
    remove_project(alias)
    click.echo(f"Removed '{alias}'")


@cli.command("list")
def list_projects() -> None:
    """List registered repositories."""
    config = load_config()
    projects = config.get("projects", {})
    if not projects:
        click.echo("No projects registered. Use 'gitbrief add <alias> <path>' to add one.")
        return
    for alias, project in projects.items():
        path = project["path"]
        backend = project.get("backend")
        error = validate_repo(path)
        warning = f"  [WARNING: {error}]" if error else ""
        backend_tag = f"  [{backend}]" if backend else ""
        click.echo(f"  {alias:20s} {path}{backend_tag}{warning}")


@cli.group("config")
def config_group() -> None:
    """Manage configuration settings."""


@config_group.command("set")
@click.argument("key")
@click.argument("value")
def config_set(key: str, value: str) -> None:
    """Set a configuration value."""
    set_setting(key, value)
    click.echo(f"Set {key} = {value}")


@config_group.command("get")
@click.argument("key")
def config_get(key: str) -> None:
    """Get a configuration value."""
    value = get_setting(key)
    if value is None:
        click.echo(f"Key '{key}' not set.")
    else:
        click.echo(f"{key} = {value}")


@config_group.command("list")
def config_list() -> None:
    """List all configuration settings."""
    config = load_config()
    settings = config.get("settings", {})
    if not settings:
        click.echo("No settings configured.")
        return
    for key, value in settings.items():
        click.echo(f"  {key:20s} {value}")


def _resolve_time_arg(label: str, value: str) -> str:
    """Parse a date string or duration into an ISO date. Raises ClickException on failure."""
    if validate_date_string(value):
        return value
    try:
        return parse_duration(value)
    except ValueError as e:
        raise click.ClickException(f"Invalid {label} value {value!r}: {e}")


@cli.command()
@click.option("--last", "duration", default=None, help="Time window (e.g. 1d, 1w, 1m, 1y, today, this-week)")
@click.option("--since", "since_date", default=None, help="Start date (YYYY-MM-DD)")
@click.option("--until", "until_date", default=None, help="End date (YYYY-MM-DD or duration)")
@click.option("--all-authors", is_flag=True, help="Include all contributors")
@click.option("--no-clipboard", is_flag=True, help="Skip copying to clipboard")
@click.option("--dry-run", is_flag=True, help="Print prompt to stdout, skip AI call")
@click.option("--raw", is_flag=True, help="Print raw commit data, skip AI call")
@click.argument("projects", nargs=-1)
def summary(
    duration: str | None,
    since_date: str | None,
    until_date: str | None,
    all_authors: bool,
    no_clipboard: bool,
    dry_run: bool,
    raw: bool,
    projects: tuple[str, ...],
) -> None:
    """Generate an AI summary of recent git activity."""
    if not duration and not since_date:
        raise click.ClickException("Specify --last or --since.")
    if duration and since_date:
        raise click.ClickException("Use either --last or --since, not both.")

    if since_date and not validate_date_string(since_date):
        raise click.ClickException(
            f"Invalid --since date {since_date!r}. Expected format: YYYY-MM-DD."
        )

    if duration:
        since = _resolve_time_arg("--last", duration)
        time_description = f"the last {duration}"
    else:
        assert since_date is not None
        since = since_date
        time_description = f"since {since_date}"

    until: str | None = None
    if until_date:
        until = _resolve_time_arg("--until", until_date)
        if until <= since:
            raise click.ClickException(
                f"--until ({until}) must be after --since/--last ({since})."
            )
        time_description = f"from {since} to {until}"

    config = load_config()
    all_projects = config.get("projects", {})

    if not all_projects:
        raise click.ClickException(
            "No projects registered. Use 'gitbrief add <alias> <path>' first."
        )

    if projects:
        selected = {}
        for alias in projects:
            if alias not in all_projects:
                raise click.ClickException(f"Unknown project: '{alias}'")
            selected[alias] = all_projects[alias]
    else:
        selected = all_projects

    project_commits: dict[str, list[dict]] = {}
    truncated_projects: list[str] = []

    for alias, project in selected.items():
        path = project["path"]
        error = validate_repo(path)
        if error:
            click.echo(f"Warning: Skipping '{alias}': {error}", err=True)
            continue

        author = None
        if not all_authors:
            author = get_git_user_email(path)
            if not author:
                click.echo(
                    f"Warning: No git user.email set for '{alias}', showing all authors.",
                    err=True,
                )

        commits = extract_commits(path, since, author, until)
        if commits:
            project_commits[alias] = commits
            if len(commits) >= 100:
                truncated_projects.append(alias)

    if not project_commits:
        click.echo(f"No activity found across selected projects ({time_description}).")
        return

    if raw:
        for alias, commits in project_commits.items():
            click.echo(f"=== {alias} ===")
            for commit in commits:
                click.echo(f"sha: {commit['sha']}")
                click.echo(f"subject: {commit['subject']}")
                if commit["body"]:
                    click.echo(f"body: {commit['body']}")
                if "files_changed" in commit:
                    fc = commit["files_changed"]
                    ins = commit.get("insertions", 0)
                    dels = commit.get("deletions", 0)
                    click.echo(f"stats: {fc} file{'s' if fc != 1 else ''}, +{ins}/-{dels}")
                if commit.get("branch"):
                    click.echo(f"branch: {commit['branch']}")
                if commit.get("refs"):
                    click.echo(f"refs: {', '.join(commit['refs'])}")
                click.echo("---")
        return

    prompt = build_summary_prompt(project_commits, time_description)
    if truncated_projects:
        prompt += (
            f"\nNote: Commit history was truncated to 100 for: "
            f"{', '.join(truncated_projects)}.\n"
        )

    if dry_run:
        click.echo(prompt)
        return

    backend = get_setting("backend") or "claude"
    timeout = get_setting_int("timeout", 120)
    max_retries = get_setting_int("retries", 2)

    console = Console(stderr=True)
    try:
        with console.status("Generating summary..."):
            result = invoke_ai(prompt, backend, timeout=timeout, max_retries=max_retries)
    except AIBackendError as e:
        msg = str(e)
        if e.hint:
            msg = f"{msg}\nHint: {e.hint}"
        raise click.ClickException(msg)

    click.echo(result)

    if not no_clipboard:
        if copy_to_clipboard(result):
            click.echo("\nCopied to clipboard.", err=True)
        else:
            click.echo("\nCould not copy to clipboard.", err=True)


if __name__ == "__main__":
    cli()
