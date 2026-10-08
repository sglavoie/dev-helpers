"""Card files an SD card copy skips because another file holds their name."""

from __future__ import annotations

import os
import stat
from pathlib import Path

from photos_backup.cli.context import suggested_command

# rsync -a preserves modification times, but FAT cards store them in 2-second
# steps and openrsync may round to whole seconds when it sets them.
_MTIME_TOLERANCE_SECONDS = 2
_LISTED_CONFLICTS = 5


def conflicting_files(source: Path, target: Path) -> list[Path]:
    """Card files whose archived namesake at `target` is a different file.

    `rsync --ignore-existing` keeps the archived copy, so after a camera counter
    reset these new photos would otherwise never be copied. A file counts as
    different when its size or modification time does not match. Dot files are
    Finder or camera clutter and are ignored. Paths are relative to `source`.
    """
    if not target.is_dir():
        return []
    conflicts: list[Path] = []
    for root, directories, files in os.walk(source):
        directories[:] = [name for name in directories if not name.startswith(".")]
        for name in files:
            if name.startswith("."):
                continue
            card_file = Path(root) / name
            relative = card_file.relative_to(source)
            try:
                card = card_file.lstat()
                archived = (target / relative).lstat()
            except OSError:
                continue
            if not stat.S_ISREG(card.st_mode):
                continue
            if (
                not stat.S_ISREG(archived.st_mode)
                or card.st_size != archived.st_size
                or abs(card.st_mtime - archived.st_mtime) > _MTIME_TOLERANCE_SECONDS
            ):
                conflicts.append(relative)
    return sorted(conflicts)


def conflicting_files_message(
    target: Path, conflicts: list[Path], *, dry_run: bool
) -> str:
    listed = ", ".join(str(path) for path in conflicts[:_LISTED_CONFLICTS])
    if len(conflicts) > _LISTED_CONFLICTS:
        listed += f", and {len(conflicts) - _LISTED_CONFLICTS} more"
    return (
        f"{len(conflicts)} SD card file(s) {'would not be' if dry_run else 'were not'} "
        f"copied because a different file with the same name is already in "
        f"'{target}' (camera file counter reset?): {listed}. Move or rename the "
        f"archived copies, then run `{suggested_command('sd-card')}` again"
    )
