from __future__ import annotations

import click

ACTION_REQUIRED_EXIT_CODE = 3


class ActionRequired(click.ClickException):
    """The run stopped because a person has to do something first."""

    exit_code = ACTION_REQUIRED_EXIT_CODE
