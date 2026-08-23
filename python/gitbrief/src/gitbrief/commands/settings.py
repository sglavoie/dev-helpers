"""The ``config`` command group."""

import click

from gitbrief.config import get_setting, load_config, set_setting


@click.group("config")
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
