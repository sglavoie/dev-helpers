from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.apple_photos.files import (
    delete_files,
    is_ignored,
    prune_empty_directories,
)
from photos_backup.archive.errors import ArchiveUnsafe

if TYPE_CHECKING:
    from photos_backup.config import ApplePhotosConfig

PHOTOS_LIBRARY_SUFFIX = ".photoslibrary"
EXPORT_DB_PREFIX = ".osxphotos_export.db"

MEDIA_SUFFIXES = frozenset(
    {
        ".3gp",
        ".arw",
        ".avi",
        ".cr2",
        ".cr3",
        ".dng",
        ".gif",
        ".heic",
        ".heif",
        ".jpeg",
        ".jpg",
        ".m4v",
        ".mov",
        ".mp4",
        ".nef",
        ".orf",
        ".png",
        ".raf",
        ".raw",
        ".rw2",
        ".tif",
        ".tiff",
    }
)
SIDECAR_SUFFIXES = frozenset({".aae", ".json", ".xmp"})
RECOGNIZED_SUFFIXES = MEDIA_SUFFIXES | SIDECAR_SUFFIXES

NOT_CONFIGURED = (
    "no [apple_photos] legacy_export is configured, so there is no local export "
    "this command is allowed to delete"
)

# How many offending paths to name before the message stops being useful.
_PATH_SAMPLE = 3


@dataclass(frozen=True)
class LocalExportScan:
    """Everything found under the local export, with nothing interpreted yet."""

    root: Path
    files: tuple[Path, ...]
    total_bytes: int
    symlinks: tuple[Path, ...]
    unrecognized: tuple[Path, ...]
    libraries: tuple[Path, ...]

    @property
    def file_count(self) -> int:
        return len(self.files)


@dataclass(frozen=True)
class LocalExportPlan:
    """One directory, everything in it, and the reason it may not be deleted."""

    target: Path | None
    scan: LocalExportScan | None
    refusal: str | None

    @property
    def deletable(self) -> bool:
        return self.refusal is None and self.scan is not None


@dataclass(frozen=True)
class LocalExportCleanup:
    """What emptying the local export deleted, pruned, and reclaimed."""

    target: Path
    deleted: tuple[Path, ...]
    pruned: tuple[Path, ...]
    freed_bytes: int


def plan_local_export_cleanup(
    config: ApplePhotosConfig, *, home: Path | None = None
) -> LocalExportPlan:
    """Decide whether the configured local export may be emptied, and by how much.

    The target is never taken from the caller: it is the configured
    `legacy_export` or nothing, so no argument can point this command elsewhere.
    """
    target = config.legacy_export
    if target is None:
        return LocalExportPlan(target=None, scan=None, refusal=NOT_CONFIGURED)

    refusal = target_refusal(
        target,
        archive=config.archive,
        volume=config.volume,
        library=config.library,
        home=home or Path.home(),
    )
    if refusal is not None:
        return LocalExportPlan(target=target, scan=None, refusal=refusal)
    if not target.is_dir() or target.is_symlink():
        return LocalExportPlan(
            target=target,
            scan=None,
            refusal=f"'{target}' is not a real directory, so there is nothing to delete",
        )

    scan = scan_local_export(target)
    return LocalExportPlan(target=target, scan=scan, refusal=contents_refusal(scan))


def target_refusal(
    target: Path, *, archive: Path, volume: Path, library: Path, home: Path
) -> str | None:
    """Why this directory may never be emptied, or None when it may.

    Purely lexical, so every rejection is testable without a filesystem: a path
    that encloses anything that matters, or that lives inside the archive or a
    Photos library, is refused before anything is read.
    """
    if target.parent == target:
        return f"'{target}' is the filesystem root"
    if target == home:
        return f"'{target}' is the home directory"
    if any(part.endswith(PHOTOS_LIBRARY_SUFFIX) for part in target.parts):
        return f"'{target}' is inside a Photos library"

    inside = (
        ("archive", archive),
        ("archive volume", volume),
        ("Photos library", library),
    )
    for label, other in inside:
        if target == other or target.is_relative_to(other):
            return f"'{target}' is the {label} '{other}' or lives inside it"
    for label, other in (*inside, ("home directory", home)):
        if other.is_relative_to(target):
            return f"'{target}' contains the {label} '{other}'"
    return None


def scan_local_export(root: Path) -> LocalExportScan:
    """Record every entry under the export directory without judging any of it."""
    files: list[Path] = []
    symlinks: list[Path] = []
    unrecognized: list[Path] = []
    libraries: list[Path] = []
    total = 0

    for path in sorted(root.rglob("*")):
        if path.is_symlink():
            symlinks.append(path)
        elif path.name.endswith(PHOTOS_LIBRARY_SUFFIX):
            libraries.append(path)
        elif path.is_dir():
            continue
        elif not path.is_file() or not _recognized(path):
            unrecognized.append(path)
        else:
            files.append(path)
            total += path.stat().st_size

    return LocalExportScan(
        root=root,
        files=tuple(files),
        total_bytes=total,
        symlinks=tuple(symlinks),
        unrecognized=tuple(unrecognized),
        libraries=tuple(libraries),
    )


def contents_refusal(scan: LocalExportScan) -> str | None:
    """Why these contents may not be deleted wholesale, or None when they may."""
    if scan.symlinks:
        return (
            f"{len(scan.symlinks)} entr(ies) under '{scan.root}' are symlinks that "
            f"could point anywhere ({_sample(scan.symlinks)})"
        )
    if scan.libraries:
        return (
            f"{len(scan.libraries)} Photos library(ies) are stored under "
            f"'{scan.root}' ({_sample(scan.libraries)})"
        )
    if scan.unrecognized:
        return (
            f"{len(scan.unrecognized)} file(s) under '{scan.root}' are not something "
            f"an Apple Photos export writes ({_sample(scan.unrecognized)})"
        )
    return None


def confirmation_matches(typed: str, target: Path) -> bool:
    """Whether a person typed this exact directory, and so meant this deletion."""
    return typed.strip() == str(target)


def delete_local_export(plan: LocalExportPlan) -> LocalExportCleanup:
    """Delete exactly what the plan classified, then prune the emptied directories.

    The plan is rechecked here rather than trusted, so no caller can delete a
    directory this module refused.
    """
    if not plan.deletable or plan.scan is None or plan.target is None:
        raise ArchiveUnsafe(
            f"'{plan.target}' was refused, so nothing may be deleted from it: "
            f"{plan.refusal}"
        )

    deleted = delete_files(plan.scan.files)
    return LocalExportCleanup(
        target=plan.target,
        deleted=deleted,
        pruned=prune_empty_directories(plan.target),
        freed_bytes=plan.scan.total_bytes,
    )


def _recognized(path: Path) -> bool:
    return (
        is_ignored(path)
        or path.name.startswith(EXPORT_DB_PREFIX)
        or path.suffix.lower() in RECOGNIZED_SUFFIXES
    )


def _sample(paths: tuple[Path, ...]) -> str:
    names = [str(path) for path in paths[:_PATH_SAMPLE]]
    remaining = len(paths) - len(names)
    if remaining > 0:
        names.append(f"and {remaining} more")
    return ", ".join(names)
