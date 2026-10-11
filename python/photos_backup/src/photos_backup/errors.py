from __future__ import annotations

from typing import IO, Any

import click

ACTION_REQUIRED_EXIT_CODE = 3


def show_action_needed(
    error: click.ClickException, file: IO[Any] | None = None
) -> None:
    """Label an exit 3 as something to do, since click's `Error:` reads as a failure."""
    click.echo(
        f"Action needed: {error.format_message()}",
        file=file or click.get_text_stream("stderr"),
    )


class ActionRequired(click.ClickException):
    """The run stopped because a person has to do something first."""

    exit_code = ACTION_REQUIRED_EXIT_CODE
    show = show_action_needed
