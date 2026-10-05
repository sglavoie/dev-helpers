from __future__ import annotations

from pathlib import Path

import click

from photos_backup.errors import ActionRequired


def exclude_from_arg(
    exclude_file: Path | None, *, delete_at_destination: bool = False
) -> str:
    """Require configured exclusions for deletions; otherwise warn if absent."""
    if exclude_file is None:
        return ""
    if not exclude_file.is_file():
        if delete_at_destination:
            raise ActionRequired(
                f"Configured exclusion file '{exclude_file}' is missing or not a "
                "regular file; refusing SSD deletions. Restore the file or rerun "
                "without --delete."
            )
        click.echo(
            f"Warning: configured exclusion file '{exclude_file}' is missing or "
            "not a regular file; continuing without these exclusions.",
            err=True,
        )
        return ""
    return f"--exclude-from={exclude_file}"
