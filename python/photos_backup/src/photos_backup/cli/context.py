from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

import click


@dataclass(frozen=True)
class CliContext:
    config_path: Path | None = None


def config_path_from(ctx: click.Context) -> Path | None:
    return ctx.ensure_object(CliContext).config_path
