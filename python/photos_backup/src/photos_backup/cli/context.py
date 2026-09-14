from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

import click

from photos_backup.config import ApplePhotosConfig, load_apple_photos_config


@dataclass(frozen=True)
class CliContext:
    config_path: Path | None = None
    volume: Path | None = None


def config_path_from(ctx: click.Context) -> Path | None:
    return ctx.ensure_object(CliContext).config_path


def apple_photos_config_from(ctx: click.Context) -> ApplePhotosConfig:
    """Load the Apple Photos section with any `--volume` override applied."""
    cli_context = ctx.ensure_object(CliContext)
    return load_apple_photos_config(cli_context.config_path, volume=cli_context.volume)
