from pathlib import Path

from photos_backup.errors import ActionRequired


def check_copy_paths(
    sources: tuple[Path, ...], destination: Path, *, workflow: str
) -> None:
    """Check rsync's directory-copy layout before creating or copying anything.

    Sources have no trailing slash, so each lands at destination/source.name.
    Resolve aliases on both sides, including an existing symlink at that target.
    """
    for source in sources:
        check_copy_path(source, workflow=workflow, source=True)
    check_copy_path(destination, workflow=workflow)
    resolved_sources = [source.resolve() for source in sources]
    resolved_destination = destination.resolve()
    targets = [destination / source.name for source in sources]
    resolved_targets = []
    for target in targets:
        check_copy_path(target, workflow=workflow)
        resolved_targets.append(target.resolve())

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

    for index, target in enumerate(resolved_targets):
        for other in resolved_targets[:index]:
            if target.is_relative_to(other) or other.is_relative_to(target):
                raise ActionRequired(
                    f"{workflow} sources map to overlapping copy targets beneath "
                    f"'{destination}': {', '.join(str(source) for source in sources)}; "
                    "use distinct destination directories"
                )


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
