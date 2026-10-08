from __future__ import annotations

import click

from photos_backup.errors import ACTION_REQUIRED_EXIT_CODE


class ArchiveError(click.ClickException):
    """Base class for fail-closed archive errors."""

    exit_code = 2


class ArchiveUnavailable(ArchiveError):
    """The archive volume is not mounted or cannot be written to right now."""

    exit_code = ACTION_REQUIRED_EXIT_CODE


class ArchiveUnsafe(ArchiveError):
    """The archive path or state cannot be trusted and must not be used."""


class ArchiveLocked(ArchiveError):
    """Another photos-backup run already holds the archive lock."""

    exit_code = ACTION_REQUIRED_EXIT_CODE
