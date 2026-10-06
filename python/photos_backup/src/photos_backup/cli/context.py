from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import shlex

import click

from photos_backup.config import ApplePhotosConfig, load_apple_photos_config


@dataclass(frozen=True)
class CliContext:
    config_path: Path | None = None
    volume: Path | None = None


def suggested_command(*arguments: str) -> str:
    """Render a copyable command for the current archive, or a plain CLI hint."""
    command = ["photos-backup"]
    ctx = click.get_current_context(silent=True)
    options = ctx.find_object(CliContext) if ctx is not None else None
    if options is not None:
        if options.config_path is not None:
            command.extend(["--config", str(options.config_path)])
        if options.volume is not None:
            command.extend(["--volume", str(options.volume)])
    return shlex.join([*command, *arguments])


def config_path_from(ctx: click.Context) -> Path | None:
    return ctx.ensure_object(CliContext).config_path


def export_retry_arguments() -> list[str] | None:
    """Capture the export options, excluding unrelated pipeline transfers."""
    ctx = click.get_current_context(silent=True)
    if ctx is None or ctx.command.name not in (
        "daily",
        "bootstrap",
        "recent",
        "apple-photos",
        "backup-all",
    ):
        return None
    arguments = [ctx.command.name]
    if ctx.command.name == "backup-all":
        arguments.extend(["--skip-sd-card", "--skip-ssd", "--skip-remote"])
    for key in ("days", "download_timeout"):
        if key in ctx.params:
            arguments.extend(["--" + key.replace("_", "-"), str(ctx.params[key])])
    return arguments


def apple_photos_config_from(ctx: click.Context) -> ApplePhotosConfig:
    """Load the Apple Photos section with any `--volume` override applied."""
    cli_context = ctx.ensure_object(CliContext)
    return load_apple_photos_config(cli_context.config_path, volume=cli_context.volume)
