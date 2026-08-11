from __future__ import annotations

import os
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.archive.errors import ArchiveUnavailable, ArchiveUnsafe
from photos_backup.archive.paths import ArchivePaths

if TYPE_CHECKING:
    from photos_backup.archive.probes import SystemProbes
    from photos_backup.config import ApplePhotosConfig


def resolve_archive(config: ApplePhotosConfig, probes: SystemProbes) -> ArchivePaths:
    """Validate the configured volume and archive without touching either."""
    volume = config.volume
    archive = config.archive
    _reject_ambiguous(volume, "volume")
    _reject_ambiguous(archive, "archive")
    if archive == volume or not archive.is_relative_to(volume):
        raise ArchiveUnsafe(
            f"[apple_photos] archive: '{archive}' must be a directory inside "
            f"volume '{volume}'"
        )
    _check_volume(volume, probes)
    _check_descent(volume, archive)
    return ArchivePaths(volume=volume, archive=archive)


def create_archive_tree(paths: ArchivePaths) -> None:
    """Create the archive subtree only; the volume itself is never created."""
    current = paths.volume
    for part in paths.archive.relative_to(paths.volume).parts:
        current = current / part
        _make_directory(current)
    for directory in paths.managed_directories:
        _make_directory(directory)


def _reject_ambiguous(path: Path, key: str) -> None:
    if not path.is_absolute():
        raise ArchiveUnsafe(f"[apple_photos] {key}: '{path}' must be absolute")
    if ".." in path.parts:
        raise ArchiveUnsafe(
            f"[apple_photos] {key}: '{path}' must not contain '..'; "
            "write the resolved path instead"
        )


def _check_volume(volume: Path, probes: SystemProbes) -> None:
    if volume.is_symlink():
        raise ArchiveUnsafe(
            f"[apple_photos] volume: '{volume}' is a symlink; "
            "point it at the real mount point"
        )
    if not volume.is_dir():
        raise ArchiveUnavailable(
            f"[apple_photos] volume: '{volume}' does not exist; "
            "connect the drive and retry"
        )
    if not probes.is_mount(volume):
        raise ArchiveUnavailable(
            f"[apple_photos] volume: '{volume}' is not a mount point; "
            "connect the drive and retry"
        )
    resolved = Path(os.path.realpath(volume))
    if resolved != volume:
        raise ArchiveUnsafe(
            f"[apple_photos] volume: '{volume}' resolves to '{resolved}'; "
            "configure the resolved path instead"
        )


def _check_descent(volume: Path, archive: Path) -> None:
    current = volume
    for part in archive.relative_to(volume).parts:
        current = current / part
        if current.is_symlink():
            raise ArchiveUnsafe(
                f"[apple_photos] archive: '{current}' is a symlink and could "
                f"escape volume '{volume}'"
            )
        if current.exists() and not current.is_dir():
            raise ArchiveUnsafe(
                f"[apple_photos] archive: '{current}' exists and is not a directory"
            )


def _make_directory(directory: Path) -> None:
    try:
        directory.mkdir(exist_ok=True)
    except OSError as error:
        raise ArchiveUnavailable(
            f"[apple_photos] archive: could not create '{directory}': {error}"
        ) from error
    if directory.is_symlink() or not directory.is_dir():
        raise ArchiveUnsafe(
            f"[apple_photos] archive: '{directory}' is not a real directory"
        )
