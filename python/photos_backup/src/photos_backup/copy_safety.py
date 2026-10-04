from pathlib import Path

from photos_backup.errors import ActionRequired


def check_copy_path(path: Path, *, workflow: str, source: bool = False) -> None:
    """Never create a missing macOS volume while preparing a copy destination."""
    if ".." in path.parts:
        raise ActionRequired(f"{workflow} path '{path}' must not contain '..'")
    volumes = Path("/Volumes")
    # Check both spellings so a local symlink into /Volumes cannot bypass this.
    for candidate in (path, path.resolve()):
        if not candidate.is_relative_to(volumes):
            continue
        parts = candidate.relative_to(volumes).parts
        if not parts:
            raise ActionRequired(f"Configure a {workflow} path beneath a mounted drive")
        volume = volumes / parts[0]
        if volume.is_symlink() or not volume.is_dir() or not volume.is_mount():
            raise ActionRequired(
                f"{workflow} volume '{volume}' is not a mounted drive; connect it and retry"
            )
    if source and not path.is_dir():
        raise ActionRequired(
            f"{workflow} source '{path}' is not an available directory; "
            "connect the source drive or correct the configured path"
        )
