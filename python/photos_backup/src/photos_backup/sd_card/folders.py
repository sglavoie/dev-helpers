"""Camera folders an SD card copy would leave behind."""

from __future__ import annotations

import re
from pathlib import Path

# DCF names a folder with three digits from 100 to 999 and five free characters,
# e.g. Sony's 100MSDCF. Cameras open the next one after 9,999 files.
DCF_FOLDER = re.compile(r"[1-9][0-9]{2}[0-9A-Za-z_]{5}")


def uncovered_camera_folders(source: Path) -> list[Path]:
    """Sibling DCF folders when `source` names a single one, sorted by name."""
    if not DCF_FOLDER.fullmatch(source.name):
        return []
    try:
        siblings = list(source.parent.iterdir())
    except OSError:
        return []
    return sorted(
        sibling
        for sibling in siblings
        if sibling.name != source.name
        and DCF_FOLDER.fullmatch(sibling.name)
        and sibling.is_dir()
    )


def uncovered_camera_folders_message(source: Path, folders: list[Path]) -> str:
    names = ", ".join(folder.name for folder in folders)
    return (
        f"SD card folder(s) {names} beside '{source}' are not copied; set "
        f"[sd_card] source to '{source.parent}' to include every camera folder"
    )
