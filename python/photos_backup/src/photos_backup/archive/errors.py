from __future__ import annotations

import click


class ArchiveError(click.UsageError):
    """Base class for fail-closed archive errors."""


class ArchiveUnavailable(ArchiveError):
    """The archive volume is not mounted or cannot be written to right now."""


class ArchiveUnsafe(ArchiveError):
    """The archive path or state cannot be trusted and must not be used."""


class ArchiveLocked(ArchiveError):
    """Another photos-backup run already holds the archive lock."""
