import datetime
import re
import shutil
from typing import TYPE_CHECKING

import click

if TYPE_CHECKING:
    from click.shell_completion import CompletionItem
from rich.console import Console
from rich.markdown import Markdown
from rich.panel import Panel
from rich.progress import BarColumn, MofNCompleteColumn, Progress, TextColumn
from rich.table import Table

from gitbrief.ai import invoke_ai
from gitbrief.clipboard import copy_to_clipboard
from gitbrief.config import (
    add_project,
    add_to_group,
    create_group,
    delete_group,
    get_setting,
    get_setting_int,
    list_groups,
    load_config,
    remove_from_group,
    remove_project,
    resolve_group,
    save_config,
    set_last_summary,
    set_setting,
)
from gitbrief.exceptions import AIBackendError
from gitbrief.git import MAX_COMMITS as _DEFAULT_MAX_COMMITS
from gitbrief.git import (
    discover_repos,
    extract_commits,
    get_git_user_email,
    parse_duration,
    validate_date_string,
    validate_repo,
)
from gitbrief.history import (
    clear_history,
    get_history_entry,
    list_history,
    parse_older_than,
    save_summary,
)
from gitbrief.formatters import FORMATTERS as _FORMATTERS
from gitbrief.formatters import VALID_FORMATS, format_json
from gitbrief.prompt import build_summary_prompt
from gitbrief.templates import list_templates, load_template

_stderr_console = Console(stderr=True)


def _complete_project_alias(
    ctx: click.Context, param: click.Parameter, incomplete: str
) -> "list[CompletionItem]":
    """Shell completion for project aliases."""
    from click.shell_completion import CompletionItem

    try:
        config = load_config()
        aliases = list(config.get("projects", {}).keys())
        return [CompletionItem(a) for a in aliases if a.startswith(incomplete)]
    except Exception:
        return []


@click.group()
@click.version_option(package_name="gitbrief")
def cli() -> None:
    """AI-powered git activity summarizer."""


@cli.command()
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


@cli.command()
@click.argument("alias", shell_complete=_complete_project_alias)
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


@cli.group("group")
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


@cli.command()
def doctor() -> None:
    """Check project health and AI backend availability."""
    config = load_config()
    projects = config.get("projects", {})
    settings = config.get("settings", {})
    console = Console()

    table = Table(title="Gitbrief Health Check", show_header=True, header_style="bold")
    table.add_column("Check", style="bold")
    table.add_column("Status", justify="center")
    table.add_column("Details")

    # Check registered projects
    if not projects:
        table.add_row("Projects", "[yellow]WARN[/yellow]", "No projects registered")
    else:
        far_past = (
            datetime.date.today() - datetime.timedelta(days=365 * 10)
        ).isoformat()
        for alias, project in projects.items():
            path = project["path"]
            error = validate_repo(path)
            if error:
                table.add_row(f"Project: {alias}", "[red]FAIL[/red]", error)
            else:
                commits = extract_commits(path, far_past, max_commits=1)
                if commits:
                    table.add_row(f"Project: {alias}", "[green]OK[/green]", path)
                else:
                    table.add_row(
                        f"Project: {alias}",
                        "[yellow]WARN[/yellow]",
                        f"{path} (no commits found)",
                    )

    # Check AI backends
    for backend_name in ("claude", "copilot"):
        if shutil.which(backend_name):
            table.add_row(
                f"Backend: {backend_name}", "[green]OK[/green]", "Found in PATH"
            )
        else:
            table.add_row(
                f"Backend: {backend_name}", "[red]FAIL[/red]", "Not found in PATH"
            )

    # Check group health: groups referencing deleted projects
    groups = config.get("groups", {})
    if groups:
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
                    f"Group: {group_name}",
                    "[green]OK[/green]",
                    f"{len(aliases)} member(s)",
                )

    # Check config for unknown/deprecated keys
    known_settings = {"backend", "timeout", "retries", "max_commits"}
    unknown = set(settings) - known_settings
    if unknown:
        table.add_row(
            "Config",
            "[yellow]WARN[/yellow]",
            f"Unknown/deprecated keys: {', '.join(sorted(unknown))}",
        )
    else:
        table.add_row("Config", "[green]OK[/green]", "No deprecated or invalid keys")

    console.print(table)


@cli.command("install-completion")
@click.argument("shell", type=click.Choice(["bash", "zsh", "fish"]))
def install_completion(shell: str) -> None:
    """Print instructions for enabling shell tab completions.

    \b
    Example usage:
      gitbrief install-completion zsh
      gitbrief install-completion bash
      gitbrief install-completion fish
    """
    var = f"_GITBRIEF_COMPLETE={shell}_source"
    click.echo(
        f"To enable {shell} completions for gitbrief, add this to your shell config:\n"
    )
    if shell in ("bash", "zsh"):
        config_file = "~/.bashrc" if shell == "bash" else "~/.zshrc"
        click.echo(f'  eval "$({var} gitbrief)"\n')
        click.echo(f"  # Paste the above line into {config_file}")
    elif shell == "fish":
        click.echo(f"  {var} gitbrief | source\n")
        click.echo("  # Paste the above line into ~/.config/fish/config.fish")
    click.echo()
    click.echo("Or generate the completion script to a file:")
    click.echo(f"  {var} gitbrief > /tmp/gitbrief-complete.{shell}")


def _print_raw_commits(project_commits: dict[str, list[dict]]) -> None:
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


def _resolve_time_arg(label: str, value: str) -> str:
    """Parse a date string or duration into an ISO date. Raises ClickException on failure."""
    if validate_date_string(value):
        return value
    try:
        return parse_duration(value)
    except ValueError as e:
        raise click.ClickException(f"Invalid {label} value {value!r}: {e}")


@cli.command()
@click.option(
    "--last",
    "duration",
    default=None,
    help="Time window (e.g. 1d, 1w, 1m, 1y, today, this-week)",
)
@click.option("--since", "since_date", default=None, help="Start date (YYYY-MM-DD)")
@click.option(
    "--until", "until_date", default=None, help="End date (YYYY-MM-DD or duration)"
)
@click.option(
    "--since-last",
    "since_last",
    is_flag=True,
    help="Use each project's last summary as start date (falls back to 1w if no history)",
)
@click.option("--all-authors", is_flag=True, help="Include all contributors")
@click.option("--no-clipboard", is_flag=True, help="Skip copying to clipboard")
@click.option("--dry-run", is_flag=True, help="Print prompt to stdout, skip AI call")
@click.option("--raw", is_flag=True, help="Print raw commit data, skip AI call")
@click.option(
    "--max-commits",
    "max_commits_override",
    default=None,
    type=int,
    help="Max commits per project (overrides config)",
)
@click.option(
    "--no-fallback",
    is_flag=True,
    help="Exit immediately on AI failure (no raw data fallback)",
)
@click.option(
    "--plain",
    is_flag=True,
    help="Disable rich formatting (deprecated: use --format plain)",
)
@click.option(
    "--format",
    "output_format",
    type=click.Choice(sorted(VALID_FORMATS)),
    default="markdown",
    show_default=True,
    help="Output format for the summary",
)
@click.option(
    "--output",
    "output_file",
    type=click.Path(),
    default=None,
    help="Write summary to this file instead of stdout",
)
@click.option(
    "--detail",
    "detail_level",
    type=click.Choice(["brief", "normal", "detailed"]),
    default="normal",
    show_default=True,
    help="Summary verbosity level",
)
@click.option(
    "--template",
    "template_name",
    default="default",
    show_default=True,
    help="Template name (built-in) or path to a custom template file",
)
@click.argument("projects", nargs=-1, shell_complete=_complete_project_alias)
def summary(
    duration: str | None,
    since_date: str | None,
    until_date: str | None,
    since_last: bool,
    all_authors: bool,
    no_clipboard: bool,
    dry_run: bool,
    raw: bool,
    max_commits_override: int | None,
    no_fallback: bool,
    plain: bool,
    output_format: str,
    output_file: str | None,
    detail_level: str,
    template_name: str,
    projects: tuple[str, ...],
) -> None:
    """Generate an AI summary of recent git activity."""
    # Validate time window arguments
    if not duration and not since_date and not since_last:
        raise click.ClickException("Specify --last, --since, or --since-last.")
    if since_last and (duration or since_date):
        raise click.ClickException(
            "--since-last cannot be combined with --last or --since."
        )
    if duration and since_date:
        raise click.ClickException("Use either --last or --since, not both.")

    if since_date and not validate_date_string(since_date):
        raise click.ClickException(
            f"Invalid --since date {since_date!r}. Expected format: YYYY-MM-DD."
        )

    until: str | None = None
    if until_date:
        until = _resolve_time_arg("--until", until_date)

    # Validate template exists early (before expensive commit extraction)
    if template_name != "default":
        try:
            load_template(template_name)
        except FileNotFoundError:
            raise click.ClickException(
                f"Template not found: {template_name!r}. "
                "Use 'gitbrief template list' to see available templates."
            )

    config = load_config()
    all_projects = config.get("projects", {})

    if not all_projects:
        raise click.ClickException(
            "No projects registered. Use 'gitbrief add <alias> <path>' first."
        )

    if projects:
        # Expand @all and @<groupname> references, then deduplicate
        expanded: list[str] = []
        for token in projects:
            if token == "@all":
                for a in all_projects:
                    if a not in expanded:
                        expanded.append(a)
            elif token.startswith("@"):
                group_name = token[1:]
                groups = config.get("groups", {})
                if group_name not in groups:
                    raise click.ClickException(
                        f"Unknown group '@{group_name}'. "
                        "Use 'gitbrief group list' to see available groups."
                    )
                for a in groups[group_name]:
                    if a not in expanded:
                        expanded.append(a)
            else:
                if token not in all_projects:
                    raise click.ClickException(f"Unknown project: '{token}'")
                if token not in expanded:
                    expanded.append(token)
        selected = {a: all_projects[a] for a in expanded}
    else:
        selected = all_projects

    # Compute per-project since dates
    per_project_since: dict[str, str] = {}
    if since_last:
        last_summaries = config.get("last_summary", {})
        default_since = parse_duration("1w")
        for alias in selected:
            ts = last_summaries.get(alias)
            if ts:
                # Use date portion of ISO timestamp
                per_project_since[alias] = ts[:10]
            else:
                per_project_since[alias] = default_since
        # Build time_description
        unique_dates = set(per_project_since.values())
        if len(unique_dates) == 1:
            time_description = f"since {next(iter(unique_dates))}"
        else:
            time_description = "since last summary (varies per project)"
    else:
        if duration:
            since = _resolve_time_arg("--last", duration)
            time_description = f"the last {duration}"
        else:
            assert since_date is not None
            since = since_date
            time_description = f"since {since_date}"

        if until and not since_last:
            if until <= since:
                raise click.ClickException(
                    f"--until ({until}) must be after --since/--last ({since})."
                )
            time_description = f"from {since} to {until}"

        for alias in selected:
            per_project_since[alias] = since

    max_commits = (
        max_commits_override
        if max_commits_override is not None
        else get_setting_int("max_commits", _DEFAULT_MAX_COMMITS)
    )

    project_commits: dict[str, list[dict]] = {}
    truncated_projects: list[str] = []

    if len(selected) > 1:
        with Progress(
            TextColumn("[progress.description]{task.description}"),
            BarColumn(),
            MofNCompleteColumn(),
            TextColumn(" projects  [cyan]\\[{task.fields[current]}][/cyan]"),
            console=Console(stderr=True),
            transient=True,
        ) as progress:
            task = progress.add_task(
                "Extracting commits", total=len(selected), current=""
            )
            for alias, project in selected.items():
                progress.update(task, current=alias)
                path = project["path"]
                error = validate_repo(path)
                if error:
                    progress.console.print(
                        f"Warning: Skipping '{alias}': {error}", style="yellow"
                    )
                    progress.advance(task)
                    continue

                author = None
                if not all_authors:
                    author = get_git_user_email(path)
                    if not author:
                        progress.console.print(
                            f"Warning: No git user.email set for '{alias}', showing all authors.",
                            style="yellow",
                        )

                proj_since = per_project_since[alias]
                commits = extract_commits(
                    path, proj_since, author, until, max_commits=max_commits
                )
                if commits:
                    project_commits[alias] = commits
                    if len(commits) >= max_commits:
                        truncated_projects.append(alias)
                progress.advance(task)
    else:
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

            proj_since = per_project_since[alias]
            commits = extract_commits(
                path, proj_since, author, until, max_commits=max_commits
            )
            if commits:
                project_commits[alias] = commits
                if len(commits) >= max_commits:
                    truncated_projects.append(alias)

    if not project_commits:
        click.echo(f"No activity found across selected projects ({time_description}).")
        return

    if raw:
        _print_raw_commits(project_commits)
        return

    # Build per_project_windows only when using --since-last and projects differ
    per_project_windows: dict[str, str] | None = None
    if since_last and len(set(per_project_since.values())) > 1:
        per_project_windows = {
            a: f"since {s}"
            for a, s in per_project_since.items()
            if a in project_commits
        }

    try:
        prompt = build_summary_prompt(
            project_commits,
            time_description,
            per_project_windows,
            detail=detail_level,
            template=template_name,
        )
    except FileNotFoundError as e:
        raise click.ClickException(str(e))
    if truncated_projects:
        prompt += (
            f"\nNote: Commit history was truncated to {max_commits} for: "
            f"{', '.join(truncated_projects)}.\n"
        )

    if dry_run:
        click.echo(prompt)
        return

    backend = get_setting("backend") or "claude"

    if not shutil.which(backend):
        _stderr_console.print(
            f"Warning: '{backend}' CLI not found in PATH. Summarization will fail.",
            style="yellow",
        )

    timeout = get_setting_int("timeout", 120)
    max_retries = get_setting_int("retries", 2)

    console = Console(stderr=True)
    try:
        with console.status("Generating summary..."):
            result = invoke_ai(
                prompt, backend, timeout=timeout, max_retries=max_retries
            )
    except AIBackendError as e:
        msg = str(e)
        if e.hint:
            msg = f"{msg}\nHint: {e.hint}"
        if no_fallback:
            raise click.ClickException(msg)
        click.echo(f"Error: {msg}", err=True)
        click.echo(
            "AI summarization failed. Showing raw commit data instead:", err=True
        )
        _print_raw_commits(project_commits)
        return

    # Save to history and update last_summary timestamps
    total_commits = sum(len(c) for c in project_commits.values())
    representative_since = next(iter(per_project_since.values()))
    save_summary(
        projects=list(project_commits.keys()),
        since=representative_since,
        until=until,
        backend=backend,
        commit_count=total_commits,
        summary=result,
    )
    now_ts = datetime.datetime.now().isoformat(timespec="seconds")
    for alias in project_commits:
        set_last_summary(alias, now_ts)

    # --plain is deprecated; map to --format plain
    if plain:
        click.echo(
            "Warning: --plain is deprecated, use --format plain instead.", err=True
        )
        output_format = "plain"

    # Apply formatter
    if output_format == "json":
        period: dict = {"since": representative_since}
        if until:
            period["until"] = until
        metadata = {
            "generated_at": now_ts,
            "projects": list(project_commits.keys()),
            "period": period,
            "backend": backend,
            "commit_count": total_commits,
        }
        formatted = format_json(result, metadata)
    else:
        formatter = _FORMATTERS[output_format]
        formatted = formatter(result)  # type: ignore[operator]

    if output_file:
        with open(output_file, "w") as fh:
            fh.write(formatted)
            if not formatted.endswith("\n"):
                fh.write("\n")
        click.echo(f"Summary written to {output_file}", err=True)
    elif output_format in ("plain", "slack", "json"):
        click.echo(formatted)
    else:
        # markdown — render with Rich
        out_console = Console()
        md = Markdown(formatted)
        panel = Panel(
            md, title=f"[bold]Summary: {time_description}[/bold]", border_style="blue"
        )
        out_console.print(panel)

    if not no_clipboard:
        if copy_to_clipboard(formatted):
            click.echo("\nCopied to clipboard.", err=True)
        else:
            click.echo("\nCould not copy to clipboard.", err=True)


# ---------------------------------------------------------------------------
# Template commands
# ---------------------------------------------------------------------------


@cli.group("template")
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
            f"Template not found: {name!r}. "
            "Use 'gitbrief template list' to see available templates."
        )
    click.echo(content)


# ---------------------------------------------------------------------------
# History commands
# ---------------------------------------------------------------------------


@cli.group("history")
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
def history_show(id: str) -> None:
    """Show a past summary by index or filename stem."""
    record = get_history_entry(id)
    if not record:
        raise click.ClickException(f"No history entry found for: {id!r}")
    console = Console()
    ts = record.get("timestamp", "")
    projects = ", ".join(record.get("projects", []))
    since = record.get("since", "")
    until_val = record.get("until")
    window = f"from {since} to {until_val}" if until_val else f"since {since}"
    md = Markdown(record.get("summary", ""))
    title = f"[bold]{window}[/bold]  [{projects}]  {ts}"
    console.print(Panel(md, title=title, border_style="blue"))


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

    def _make_panel(record: dict, label: str) -> Panel:
        ts = record.get("timestamp", label)
        projects = ", ".join(record.get("projects", []))
        since = record.get("since", "")
        until_val = record.get("until")
        window = f"from {since} to {until_val}" if until_val else f"since {since}"
        count = record.get("commit_count", "?")
        title = f"[bold]{label}[/bold]: {window}  [{projects}]  {count} commits  {ts}"
        md = Markdown(record.get("summary", ""))
        return Panel(md, title=title, border_style="blue")

    console.print(_make_panel(r1, "A"))
    console.rule()
    console.print(_make_panel(r2, "B"))


# ---------------------------------------------------------------------------
# Scan command
# ---------------------------------------------------------------------------


def _resolve_alias(name: str, existing: dict) -> str:
    """Return name if not already an alias, otherwise name-2, name-3, etc."""
    if name not in existing:
        return name
    n = 2
    while f"{name}-{n}" in existing:
        n += 1
    return f"{name}-{n}"


def _register_repos(
    repos: list[dict], config: dict, console: Console
) -> int:
    """Add repos to config (in-place). Returns number of repos added."""
    added = 0
    for repo in repos:
        alias = _resolve_alias(repo["name"], config["projects"])
        config["projects"][alias] = {"path": repo["path"], "backend": None}
        console.print(f"  Added [bold]{alias}[/bold] → {repo['path']}")
        added += 1
    return added


@cli.command()
@click.argument("directory", type=click.Path(exists=True, file_okay=False, resolve_path=True))
@click.option("--depth", default=3, show_default=True, help="Max directory depth to scan")
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

    # Annotate each repo with its registration status
    for repo in repos:
        repo["status"] = "registered" if repo["path"] in registered_paths else "new"

    # Display discovery table
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

    new_repos = [r for r in repos if r["status"] == "new"]

    for i, repo in enumerate(repos, 1):
        if repo["status"] == "registered":
            status_cell = "[green]registered[/green]"
        else:
            status_cell = "[blue]new[/blue]"
        table.add_row(
            str(i),
            repo["name"],
            repo["path"],
            repo["last_commit"] or "—",
            status_cell,
        )
    console.print(table)

    if not new_repos:
        click.echo("All discovered repositories are already registered.")
        return

    if auto:
        added = _register_repos(new_repos, config, console)
        save_config(config)
        click.echo(f"\nAdded {added} new repositor{'y' if added == 1 else 'ies'}.")
        return

    # Interactive mode
    response = click.prompt(
        f"\nFound {len(new_repos)} new repositor{'y' if len(new_repos) == 1 else 'ies'}."
        " Add them? [y/N/select]",
        default="N",
    ).strip().lower()

    if response == "y":
        added = _register_repos(new_repos, config, console)
        save_config(config)
        click.echo(f"\nAdded {added} new repositor{'y' if added == 1 else 'ies'}.")

    elif response == "select":
        # Show numbered list of new repos only
        click.echo("\nNew repositories:")
        for i, repo in enumerate(new_repos, 1):
            click.echo(f"  {i:3d}  {repo['name']:30s}  {repo['path']}")
        raw = click.prompt(
            "\nEnter numbers to register (space- or comma-separated)",
            default="",
        ).strip()
        if not raw:
            click.echo("No repositories selected.")
            return
        selected: list[dict] = []
        for token in re.split(r"[\s,]+", raw):
            token = token.strip()
            if not token:
                continue
            try:
                idx = int(token) - 1
            except ValueError:
                _stderr_console.print(
                    f"[yellow]Warning:[/yellow] Ignoring invalid selection: {token!r}"
                )
                continue
            if idx < 0 or idx >= len(new_repos):
                _stderr_console.print(
                    f"[yellow]Warning:[/yellow] Index {token} out of range, skipping."
                )
                continue
            repo = new_repos[idx]
            if repo not in selected:
                selected.append(repo)
        if not selected:
            click.echo("No valid repositories selected.")
            return
        added = _register_repos(selected, config, console)
        save_config(config)
        click.echo(f"\nAdded {added} new repositor{'y' if added == 1 else 'ies'}.")

    else:
        click.echo("No repositories added.")


if __name__ == "__main__":
    cli()
