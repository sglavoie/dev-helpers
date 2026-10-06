# gitbrief

Summarize git activity across registered repositories, using the Claude or
Copilot CLI. Raw commit reports and prompt previews work without an AI backend.

## Install

Requires Python 3.13+, uv, and git. From this directory:

```bash
uv tool install --editable .
gitbrief --help
```

For AI summaries, install and authenticate your chosen `claude` or `copilot`
CLI separately. Claude is the default; use `gitbrief config set backend copilot`
to change it. `gitbrief doctor` checks repositories, settings, and installed
backends; you only need the backend you intend to use.

## Quick start

```bash
gitbrief add helpers /path/to/sglavoie_dev-helpers
gitbrief list
gitbrief summary helpers --last 1w --raw
gitbrief summary helpers --last 1w --dry-run
gitbrief summary helpers --last 1w --no-clipboard
```

`--raw` prints commits and `--dry-run` prints the prompt without invoking AI.
A normal summary passes the collected git activity to the selected AI CLI.
Summaries copy to the clipboard by default; `--no-clipboard` disables that.
By default the report filters to your git author identity; use `--all-authors`
for the whole team.

## Everyday use

```bash
gitbrief summary --last today --template standup
gitbrief summary helpers --since-last
gitbrief summary helpers --last 1m --output monthly-summary.md --no-clipboard
gitbrief history list
gitbrief config list
gitbrief doctor
```

Without project arguments, `summary` includes all registered repositories.
Use `gitbrief group --help` to organize them into named groups and
`gitbrief summary @group-name --last 1w` to summarize a group. Built-in templates
include `default`, `standup`, and `executive`.

Configuration lives at `~/.gitbrief/config.json`; an older `~/.gitbrief.json`
is migrated automatically. Removing a project with `gitbrief remove ALIAS`
removes its registration, not its repository. See `gitbrief history --help`
for saved summaries and retention commands.

## Development

```bash
uv sync
uv run pytest
uv run gitbrief --help
```

After changing dependencies, refresh the installed tool with
`uv tool install --force --editable .`.
