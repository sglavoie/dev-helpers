"""Shell completion helpers and the ``install-completion`` command."""

from typing import TYPE_CHECKING

import click

from gitbrief.config import load_config

if TYPE_CHECKING:
    from click.shell_completion import CompletionItem


def complete_project_alias(
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


@click.command("install-completion")
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
