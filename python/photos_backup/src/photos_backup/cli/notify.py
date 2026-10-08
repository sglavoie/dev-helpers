"""Optional macOS notifications for scheduled runs that need attention."""

from __future__ import annotations

import functools
import subprocess
from collections.abc import Callable
from typing import Any, TypeVar

import click

from photos_backup.errors import ActionRequired

F = TypeVar("F", bound=Callable[..., Any])

_MESSAGE_LIMIT = 240
# Title and message arrive as arguments, so neither needs AppleScript quoting.
_SCRIPT = (
    "on run argv",
    "display notification (item 2 of argv) with title (item 1 of argv)",
    "end run",
)


def notify_on_problems(command: F) -> F:
    """Add `--notify` and post a notification when the command fails or is stopped.

    Success stays silent: a scheduled run only interrupts a person when they
    have something to fix or decide. The exit status is never changed.
    """

    @functools.wraps(command)
    def wrapper(*args: Any, notify: bool, **kwargs: Any) -> Any:
        if not notify:
            return command(*args, **kwargs)
        name = click.get_current_context().info_name or "photos-backup"
        try:
            return command(*args, **kwargs)
        except ActionRequired as error:
            post_notification(f"photos-backup {name} needs you", error.format_message())
            raise
        except click.ClickException as error:
            post_notification(f"photos-backup {name} failed", error.format_message())
            raise
        except (click.exceptions.Exit, click.Abort):
            raise
        except KeyboardInterrupt:
            # SIGTERM and SIGHUP arrive here too, so a stopped launchd job notifies.
            post_notification(
                f"photos-backup {name} interrupted", "Stopped before finishing."
            )
            raise
        except Exception as error:
            post_notification(f"photos-backup {name} failed", str(error))
            raise

    return click.option(
        "--notify",
        is_flag=True,
        help="Post a macOS notification if the run fails or needs you; "
        "success stays silent.",
    )(wrapper)  # type: ignore[return-value]


def post_notification(title: str, message: str) -> None:
    """Best effort: a notification failure never hides the run's own outcome."""
    if len(message) > _MESSAGE_LIMIT:
        message = message[: _MESSAGE_LIMIT - 1] + "…"
    command = ["osascript"]
    for line in _SCRIPT:
        command.extend(["-e", line])
    try:
        subprocess.run(
            [*command, title, message or "See the command output for details."],
            check=False,
            capture_output=True,
            timeout=10,
        )
    except (OSError, subprocess.SubprocessError):
        pass
