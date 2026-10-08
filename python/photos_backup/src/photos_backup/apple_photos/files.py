from __future__ import annotations

from collections.abc import Iterable, Sequence
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.archive.errors import ArchiveUnavailable

if TYPE_CHECKING:
    from photos_backup.apple_photos.adapter import ExportedFile

# Files macOS drops into any browsed directory; never ours to explain.
IGNORED_NAMES = (".DS_Store", ".localized")
IGNORED_PREFIX = "._"

# How many offending paths to name before the message stops being useful.
PATH_SAMPLE = 3


def is_ignored(path: Path) -> bool:
    """Whether macOS wrote this file rather than an export."""
    return path.name in IGNORED_NAMES or path.name.startswith(IGNORED_PREFIX)


def sample_paths(items: Sequence[object], *, separator: str = ", ") -> str:
    """Name the first few offending paths and count the rest."""
    names = [str(item) for item in items[:PATH_SAMPLE]]
    remaining = len(items) - len(names)
    if remaining > 0:
        names.append(f"and {remaining} more")
    return separator.join(names)


def signature_matches(record: ExportedFile, size: int, mtime: float) -> bool:
    """Whether a file still has the size and whole-second mtime it was exported with."""
    if record.size is None or record.mtime is None:
        return False
    return record.size == size and int(record.mtime) == int(mtime)


def delete_files(paths: Iterable[Path]) -> tuple[Path, ...]:
    """Delete each path, treating one already gone as done rather than as an error."""
    deleted: list[Path] = []
    for path in paths:
        try:
            path.unlink()
        except FileNotFoundError:
            continue
        except OSError as error:
            raise ArchiveUnavailable(f"Could not delete '{path}': {error}") from error
        deleted.append(path)
    return tuple(deleted)


def prune_empty_directories(
    root: Path, *, keep: Iterable[Path] = ()
) -> tuple[Path, ...]:
    """Remove the directories a deletion emptied, never the root or a kept subtree."""
    return _remove_empty(sorted(root.rglob("*"), reverse=True), keep)


def prune_emptied_parents(
    deleted: Iterable[Path], root: Path, *, keep: Iterable[Path] = ()
) -> tuple[Path, ...]:
    """Remove only the ancestors of `deleted` files left empty, below `root`.

    Unlike `prune_empty_directories`, this never walks the whole tree, so a
    directory that was already empty before this run is left as it was.
    """
    ancestors = {
        parent for path in deleted for parent in path.parents if root in parent.parents
    }
    # Deepest first, so a directory emptied by removing its child goes too.
    ordered = sorted(
        ancestors, key=lambda directory: len(directory.parts), reverse=True
    )
    return _remove_empty(ordered, keep)


def _remove_empty(
    directories: Iterable[Path], keep: Iterable[Path]
) -> tuple[Path, ...]:
    protected = tuple(keep)
    removed: list[Path] = []
    for directory in directories:
        if any(
            directory == subtree or subtree in directory.parents
            for subtree in protected
        ):
            continue
        if directory.is_symlink() or not directory.is_dir():
            continue
        try:
            directory.rmdir()
        except OSError:
            continue
        removed.append(directory)
    return tuple(removed)
