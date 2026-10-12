"""Relative time wording shared by the status printers."""

from __future__ import annotations

import datetime
from typing import TYPE_CHECKING


if TYPE_CHECKING:
    pass


_SECONDS_PER_MINUTE = 60


def _relative_age(value: datetime.datetime, now: datetime.datetime) -> str:
    seconds = (now - value).total_seconds()
    if abs(seconds) < _SECONDS_PER_MINUTE:
        return "just now" if seconds >= 0 else "in less than a minute"
    for unit, duration in (
        ("day", 86400),
        ("hour", 3600),
        ("minute", _SECONDS_PER_MINUTE),
    ):
        count = int(abs(seconds) // duration)
        if count:
            age = f"{count} {unit}{'s' if count != 1 else ''}"
            return f"{age} ago" if seconds >= 0 else f"in {age}"
    raise AssertionError("unreachable age")


def _when(value: datetime.datetime | str, now: datetime.datetime) -> str:
    """A local, minute-precision time plus its age, e.g. '2026-10-06 15:58 (1 day ago)'.

    JSON output keeps the full ISO timestamps; this is only for people reading.
    """
    if isinstance(value, str):
        value = datetime.datetime.fromisoformat(value)
    local = value.astimezone() if value.tzinfo else value
    return f"{local:%Y-%m-%d %H:%M} ({_relative_age(value, now)})"


def _weekday_when(value: datetime.datetime, now: datetime.datetime) -> str:
    return f"{value.astimezone():%a} {_when(value, now)}"
