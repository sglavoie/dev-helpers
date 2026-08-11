from __future__ import annotations

import datetime
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass
from typing import TYPE_CHECKING

from photos_backup.archive.errors import (
    ArchiveError,
    ArchiveLocked,
    ArchiveUnavailable,
    ArchiveUnsafe,
)
from photos_backup.archive.lock import archive_lock
from photos_backup.archive.paths import ArchivePaths, safe_name
from photos_backup.archive.probes import SystemProbes
from photos_backup.archive.safety import create_archive_tree, resolve_archive
from photos_backup.archive.state import (
    STATE_VERSION,
    ArchiveState,
    ArchiveStateStore,
)

if TYPE_CHECKING:
    from photos_backup.config import ApplePhotosConfig

__all__ = [
    "STATE_VERSION",
    "Archive",
    "ArchiveError",
    "ArchiveLocked",
    "ArchivePaths",
    "ArchiveState",
    "ArchiveStateStore",
    "ArchiveUnavailable",
    "ArchiveUnsafe",
    "SystemProbes",
    "create_archive_tree",
    "open_archive",
    "resolve_archive",
    "safe_name",
]


@dataclass(frozen=True)
class Archive:
    """A validated, locked archive plus everything a command needs to use it."""

    paths: ArchivePaths
    state_store: ArchiveStateStore
    probes: SystemProbes
    dry_run: bool

    @property
    def hostname(self) -> str:
        return self.probes.hostname()

    def now(self) -> datetime.datetime:
        return self.probes.now()


@contextmanager
def open_archive(
    config: ApplePhotosConfig,
    *,
    dry_run: bool = False,
    probes: SystemProbes | None = None,
) -> Iterator[Archive]:
    """Validate, create, and lock the archive for one run.

    A dry run validates and reads but creates no directory, lock file, or state.
    """
    active = probes or SystemProbes()
    paths = resolve_archive(config, active)
    if not dry_run:
        create_archive_tree(paths)
    with archive_lock(paths, active, dry_run=dry_run):
        yield Archive(
            paths=paths,
            state_store=ArchiveStateStore(paths, dry_run=dry_run),
            probes=active,
            dry_run=dry_run,
        )
