from __future__ import annotations

import fcntl
import os
import stat
from collections.abc import Iterator
from contextlib import ExitStack, contextmanager
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.archive.errors import ArchiveError, ArchiveLocked, ArchiveUnavailable
from photos_backup.archive.paths import ArchivePaths, METADATA_DIR_NAME
from photos_backup.errors import ActionRequired

if TYPE_CHECKING:
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
def copy_source_lock(source: Path) -> Iterator[None]:
    """Lock an archive root (or a directory within it) for a consistent copy.

    Ordinary directories require no lock or Apple Photos configuration. A managed
    archive must already have its writer-created lock; copies never create one.
    """
    resolved = source.resolve()
    with ExitStack() as locks:
        for root in (resolved, *resolved.parents):
            metadata = root / METADATA_DIR_NAME
            try:
                status = metadata.lstat()
            except FileNotFoundError:
                continue
            except OSError as error:
                raise ActionRequired(
                    f"Could not inspect archive metadata '{metadata}': {error}"
                ) from error
            if not stat.S_ISDIR(status.st_mode):
                raise ActionRequired(
                    f"Archive metadata '{metadata}' must be a real directory"
                )
            paths = ArchivePaths(volume=root.parent, archive=root)
            try:
                locks.enter_context(_shared_lock(paths, required=True))
            except ArchiveError as error:
                raise ActionRequired(
                    f"SSD source archive cannot be locked: {error}"
                ) from error
        yield


@contextmanager
def _shared_lock(paths: ArchivePaths, *, required: bool = False) -> Iterator[None]:
    """Take a read lock without creating anything, for dry runs."""
    if not required and not paths.lock_file.is_file():
        yield
        return
    try:
        descriptor = os.open(
            paths.lock_file, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
        )
    except OSError as error:
        raise ArchiveUnavailable(
            f"Could not open archive lock '{paths.lock_file}': {error}"
        ) from error
    try:
        if not stat.S_ISREG(os.fstat(descriptor).st_mode):
            raise ArchiveUnavailable(
                f"Archive lock '{paths.lock_file}' must be a regular file"
            )
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
