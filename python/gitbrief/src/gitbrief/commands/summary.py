"""The ``summary`` command: extract commits and have an AI summarize them."""

import datetime
import shutil

import click
from rich.console import Console

from gitbrief.ai import invoke_ai
from gitbrief.clipboard import copy_to_clipboard
from gitbrief.commands.collect import collect_with_progress
from gitbrief.commands.completion import complete_project_alias
from gitbrief.commands.templates import TEMPLATE_NOT_FOUND_HINT
from gitbrief.config import get_setting, get_setting_int, load_config, set_last_summary
from gitbrief.exceptions import AIBackendError
from gitbrief.formatters import VALID_FORMATS
from gitbrief.git import MAX_COMMITS as _DEFAULT_MAX_COMMITS
from gitbrief.git import parse_duration, validate_date_string
from gitbrief.history import save_summary
from gitbrief.output import apply_format, build_period, emit, stderr_console
from gitbrief.prompt import build_summary_prompt
from gitbrief.templates import load_template


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
    """Parse a date string or duration into an ISO date.

    Raises ClickException on failure.
    """
    if validate_date_string(value):
        return value
    try:
        return parse_duration(value)
    except ValueError as e:
        raise click.ClickException(f"Invalid {label} value {value!r}: {e}")


def _expand_projects(config: dict, tokens: tuple[str, ...]) -> dict:
    """Resolve project tokens, expanding @all and @<group>, preserving order."""
    all_projects = config.get("projects", {})
    if not all_projects:
        raise click.ClickException(
            "No projects registered. Use 'gitbrief add <alias> <path>' first."
        )
    if not tokens:
        return all_projects

    expanded: list[str] = []

    def take(aliases) -> None:
        for a in aliases:
            if a not in expanded:
                expanded.append(a)

    for token in tokens:
        if token == "@all":
            take(all_projects)
        elif token.startswith("@"):
            group_name = token[1:]
            groups = config.get("groups", {})
            if group_name not in groups:
                raise click.ClickException(
                    f"Unknown group '@{group_name}'. "
                    "Use 'gitbrief group list' to see available groups."
                )
            take(groups[group_name])
        else:
            if token not in all_projects:
                raise click.ClickException(f"Unknown project: '{token}'")
            take([token])
    return {a: all_projects[a] for a in expanded}


def _since_from_last_summary(config: dict, selected: dict) -> tuple[dict[str, str], str]:
    """Per-project start dates taken from each project's last summary."""
    last_summaries = config.get("last_summary", {})
    default_since = parse_duration("1w")
    # Use the date portion of the stored ISO timestamp.
    per_project = {
        alias: (last_summaries[alias][:10] if last_summaries.get(alias) else default_since)
        for alias in selected
    }
    unique_dates = set(per_project.values())
    if len(unique_dates) == 1:
        description = f"since {next(iter(unique_dates))}"
    else:
        description = "since last summary (varies per project)"
    return per_project, description


def _since_from_window(
    selected: dict, duration: str | None, since_date: str | None, until: str | None
) -> tuple[dict[str, str], str]:
    """A single start date shared by every project."""
    if duration:
        since = _resolve_time_arg("--last", duration)
        description = f"the last {duration}"
    else:
        assert since_date is not None
        since = since_date
        description = f"since {since_date}"

    if until:
        if until <= since:
            raise click.ClickException(
                f"--until ({until}) must be after --since/--last ({since})."
            )
        description = f"from {since} to {until}"

    return {alias: since for alias in selected}, description


@click.command()
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
@click.argument("projects", nargs=-1, shell_complete=complete_project_alias)
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
    until = _validate_options(
        duration, since_date, until_date, since_last, template_name
    )

    config = load_config()
    selected = _expand_projects(config, projects)

    if since_last:
        per_project_since, time_description = _since_from_last_summary(config, selected)
    else:
        per_project_since, time_description = _since_from_window(
            selected, duration, since_date, until
        )

    max_commits = (
        max_commits_override
        if max_commits_override is not None
        else get_setting_int("max_commits", _DEFAULT_MAX_COMMITS)
    )
    project_commits, truncated_projects = collect_with_progress(
        selected,
        per_project_since,
        all_authors=all_authors,
        until=until,
        max_commits=max_commits,
    )

    if not project_commits:
        click.echo(f"No activity found across selected projects ({time_description}).")
        return

    if raw:
        _print_raw_commits(project_commits)
        return

    prompt = _build_prompt(
        project_commits,
        per_project_since,
        time_description,
        since_last=since_last,
        detail_level=detail_level,
        template_name=template_name,
        truncated_projects=truncated_projects,
        max_commits=max_commits,
    )
    if dry_run:
        click.echo(prompt)
        return

    backend = get_setting("backend") or "claude"
    result = _run_backend(prompt, backend, no_fallback=no_fallback)
    if result is None:
        _print_raw_commits(project_commits)
        return

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

    formatted = apply_format(
        result,
        output_format,
        {
            "generated_at": now_ts,
            "projects": list(project_commits.keys()),
            "period": build_period(representative_since, until),
            "backend": backend,
            "commit_count": total_commits,
        },
    )
    emit(
        formatted,
        output_format,
        output_file,
        f"[bold]Summary: {time_description}[/bold]",
    )

    if not no_clipboard:
        if copy_to_clipboard(formatted):
            click.echo("\nCopied to clipboard.", err=True)
        else:
            click.echo("\nCould not copy to clipboard.", err=True)


def _validate_options(
    duration: str | None,
    since_date: str | None,
    until_date: str | None,
    since_last: bool,
    template_name: str,
) -> str | None:
    """Check mutually exclusive time options and the template, return --until."""
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

    until = _resolve_time_arg("--until", until_date) if until_date else None

    # Validate the template early, before expensive commit extraction.
    if template_name != "default":
        try:
            load_template(template_name)
        except FileNotFoundError:
            raise click.ClickException(
                f"Template not found: {template_name!r}. {TEMPLATE_NOT_FOUND_HINT}"
            )
    return until


def _build_prompt(
    project_commits: dict[str, list[dict]],
    per_project_since: dict[str, str],
    time_description: str,
    *,
    since_last: bool,
    detail_level: str,
    template_name: str,
    truncated_projects: list[str],
    max_commits: int,
) -> str:
    # Per-project windows are only worth spelling out when they differ.
    per_project_windows: dict[str, str] | None = None
    if since_last and len(set(per_project_since.values())) > 1:
        per_project_windows = {
            a: f"since {s}" for a, s in per_project_since.items() if a in project_commits
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
    return prompt


def _run_backend(prompt: str, backend: str, *, no_fallback: bool) -> str | None:
    """Invoke the AI backend. Returns None when the caller should fall back to raw."""
    if not shutil.which(backend):
        stderr_console.print(
            f"Warning: '{backend}' CLI not found in PATH. Summarization will fail.",
            style="yellow",
        )

    timeout = get_setting_int("timeout", 120)
    max_retries = get_setting_int("retries", 2)

    try:
        with Console(stderr=True).status("Generating summary..."):
            return invoke_ai(prompt, backend, timeout=timeout, max_retries=max_retries)
    except AIBackendError as e:
        msg = str(e)
        if e.hint:
            msg = f"{msg}\nHint: {e.hint}"
        if no_fallback:
            raise click.ClickException(msg)
        click.echo(f"Error: {msg}", err=True)
        click.echo("AI summarization failed. Showing raw commit data instead:", err=True)
        return None
