import os
from pathlib import Path, PurePath

from photos_backup.errors import ActionRequired


def check_copy_paths(
    sources: tuple[Path, ...], destination: Path, *, workflow: str
) -> None:
    """Check rsync's directory-copy layout before creating or copying anything.

    Sources have no trailing slash, so each lands at destination/source.name.
    Resolve aliases on both sides, including an existing symlink at that target.
    Compare without letter case, as macOS volumes usually do: a false overlap
    only refuses a copy, while a missed one could overwrite a source.
    """
    for source in sources:
        check_copy_path(source, workflow=workflow, source=True)
    check_copy_path(destination, workflow=workflow)
    resolved_sources = [_folded(source.resolve()) for source in sources]
    resolved_destination = _folded(destination.resolve())
    targets = [destination / source.name for source in sources]
    resolved_targets = []
    for target in targets:
        check_copy_path(target, workflow=workflow)
        resolved_targets.append(_folded(target.resolve()))

    for source, resolved_source in zip(sources, resolved_sources):
        if resolved_destination.is_relative_to(resolved_source):
            raise ActionRequired(
                f"{workflow} destination '{destination}' is inside or equal to "
                f"source '{source}'; choose a separate destination"
            )
        for target, resolved_target in zip(targets, resolved_targets):
            if resolved_target.is_relative_to(
                resolved_source
            ) or resolved_source.is_relative_to(resolved_target):
                raise ActionRequired(
                    f"{workflow} copy target '{target}' overlaps source '{source}'; "
                    "choose a separate destination"
                )

    for index, resolved_target in enumerate(resolved_targets):
        for other in resolved_targets[:index]:
            if resolved_target.is_relative_to(other) or other.is_relative_to(
                resolved_target
            ):
                raise ActionRequired(
                    f"{workflow} sources map to overlapping copy targets beneath "
                    f"'{destination}': {', '.join(str(source) for source in sources)}; "
                    "use distinct destination directories"
                )


def check_copy_path(path: Path, *, workflow: str, source: bool = False) -> None:
    """Never create a missing macOS volume while preparing a copy destination."""
    if ".." in path.parts:
        raise ActionRequired(f"{workflow} path '{path}' must not contain '..'")
    # Check both spellings so a local symlink into /Volumes cannot bypass this,
    # and ignore letter case, since /volumes names the same directory on macOS.
    for candidate in (path, path.resolve()):
        if _folded(candidate).parts[:2] != ("/", "volumes"):
            continue
        _, volumes, *names = candidate.parts
        if not names:
            raise ActionRequired(f"Configure a {workflow} path beneath a mounted drive")
        volume = Path("/", volumes, names[0])
        if volume.is_symlink() or not volume.is_dir() or not volume.is_mount():
            raise ActionRequired(
                f"{workflow} volume '{volume}' is not a mounted drive; connect it and retry"
            )
    if source and not path.is_dir():
        raise ActionRequired(
            f"{workflow} source '{path}' is not an available directory; "
            "connect the source drive or correct the configured path"
        )


def disconnected_volume(path: str) -> str | None:
    """The /Volumes drive `path` lives on when it is not mounted, else None.

    Lexical and cheap: it never resolves symlinks or reads the path itself, so
    status can use it as a hint without probing copy contents.
    """
    root, volumes, name, *_ = (*PurePath(path).parts, "", "", "")
    if root != "/" or volumes.casefold() != "volumes" or not name:
        return None
    volume = str(PurePath(root, volumes, name))
    return None if os.path.ismount(volume) else volume


def check_mirror_source(path: Path, *, workflow: str) -> None:
    """Refuse to mirror deletions from a source holding only Finder clutter.

    An emptied folder or a bare mount point directory would otherwise delete
    every copy at the destination.
    """
    try:
        has_content = any(not _finder_clutter(entry.name) for entry in path.iterdir())
    except OSError as error:
        raise ActionRequired(
            f"{workflow} source '{path}' could not be listed: {error}"
        ) from error
    if not has_content:
        raise ActionRequired(
            f"{workflow} source '{path}' is empty; refusing to mirror deletions, "
            "which would empty the destination. Check that the right drive is "
            "connected, or rerun without deleting at the destination."
        )


def _finder_clutter(name: str) -> bool:
    return name == ".DS_Store" or name.startswith("._")


def _folded(path: PurePath) -> PurePath:
    return PurePath(str(path).casefold())
