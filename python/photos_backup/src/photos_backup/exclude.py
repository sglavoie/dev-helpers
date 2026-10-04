from __future__ import annotations

from pathlib import Path

import click


def exclude_from_arg(exclude_file: Path | None) -> str:
    """Build the rsync exclude flag, warning when a configured file is absent."""
    if exclude_file is None:
        return ""
    if not exclude_file.is_file():
        click.echo(
            f"Warning: configured exclusion file '{exclude_file}' is missing or "
            "not a regular file; continuing without these exclusions.",
            err=True,
        )
        return ""
    return f"--exclude-from={exclude_file}"
