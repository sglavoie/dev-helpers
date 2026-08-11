from __future__ import annotations

import fcntl
import os
from collections.abc import Iterator
from contextlib import contextmanager
from typing import TYPE_CHECKING

from photos_backup.archive.errors import ArchiveLocked, ArchiveUnavailable

if TYPE_CHECKING:
    from photos_backup.archive.paths import ArchivePaths
    from photos_backup.archive.probes import SystemProbes


@contextmanager
def archive_lock(
    paths: ArchivePaths, probes: SystemProbes, *, dry_run: bool = False
) -> Iterator[None]:
    """Hold the archive lock for the duration of the block.

    `flock` is bound to the open file description, so the lock is released by
    closing the descriptor and also when the process dies.
    """
    if dry_run:
        with _shared_lock(paths):
            yield
        return
    with _exclusive_lock(paths, probes):
        yield


@contextmanager
def _exclusive_lock(paths: ArchivePaths, probes: SystemProbes) -> Iterator[None]:
    try:
        descriptor = os.open(paths.lock_file, os.O_RDWR | os.O_CREAT, 0o644)
    except OSError as error:
        raise ArchiveUnavailable(
            f"Could not open archive lock '{paths.lock_file}': {error}"
        ) from error
    try:
        _acquire(descriptor, fcntl.LOCK_EX, paths)
        _record_owner(descriptor, probes)
        yield
    finally:
        os.close(descriptor)


@contextmanager
def _shared_lock(paths: ArchivePaths) -> Iterator[None]:
    """Take a read lock without creating anything, for dry runs."""
    if not paths.lock_file.is_file():
        yield
        return
    try:
        descriptor = os.open(paths.lock_file, os.O_RDONLY)
    except OSError as error:
        raise ArchiveUnavailable(
            f"Could not open archive lock '{paths.lock_file}': {error}"
        ) from error
    try:
        _acquire(descriptor, fcntl.LOCK_SH, paths)
        yield
    finally:
        os.close(descriptor)


def _acquire(descriptor: int, operation: int, paths: ArchivePaths) -> None:
    try:
        fcntl.flock(descriptor, operation | fcntl.LOCK_NB)
    except BlockingIOError as error:
        raise ArchiveLocked(
            f"Another photos-backup run holds the archive lock "
            f"'{paths.lock_file}'{_owner_suffix(paths)}"
        ) from error
    except OSError as error:
        raise ArchiveUnavailable(
            f"Could not lock '{paths.lock_file}': {error}"
        ) from error


def _record_owner(descriptor: int, probes: SystemProbes) -> None:
    owner = f"pid {os.getpid()} on {probes.hostname()} since {probes.now().isoformat()}"
    os.ftruncate(descriptor, 0)
    os.lseek(descriptor, 0, os.SEEK_SET)
    os.write(descriptor, f"{owner}\n".encode())


def _owner_suffix(paths: ArchivePaths) -> str:
    try:
        owner = paths.lock_file.read_text().strip()
    except OSError:
        return ""
    return f" ({owner})" if owner else ""
