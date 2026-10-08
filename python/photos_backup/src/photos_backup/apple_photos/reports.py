"""Bounded retention for the archive's per-run export reports."""

from __future__ import annotations

import re
from collections.abc import Iterable
from pathlib import Path

# One export run writes up to three files sharing `<host>_<date>[_<sequence>]`.
_RUN_FILE = re.compile(
    r"^(?:photos_export|late_photo_additions)_(?P<run>.+?)(?:\.downloads\.json|\.csv)$"
)


def report_run(path: Path) -> str | None:
    """Return the run a managed report file belongs to, or None for any other file."""
    match = _RUN_FILE.match(path.name)
    return match["run"] if match else None


def prune_reports(
    reports: Path, keep: int, protected: Iterable[Path | None]
) -> tuple[list[Path], list[str]]:
    """Delete the files of all but the `keep` most recent report runs.

    Runs named by `protected` (the baseline and the current run) always stay.
    Only regular files whose names this tool writes are considered, so anything
    else placed in the directory is left alone. Returns what was removed and a
    warning for each file that could not be.
    """
    keep_runs = {run for path in protected if path and (run := report_run(path))}
    runs: dict[str, list[Path]] = {}
    for path in reports.iterdir():
        run = report_run(path)
        if run is not None and path.is_file() and not path.is_symlink():
            runs.setdefault(run, []).append(path)
    newest_first = sorted(
        runs,
        key=lambda run: max(path.stat().st_mtime for path in runs[run]),
        reverse=True,
    )
    keep_runs.update(newest_first[:keep])
    removed: list[Path] = []
    warnings: list[str] = []
    for run in newest_first:
        if run in keep_runs:
            continue
        for path in runs[run]:
            try:
                path.unlink()
                removed.append(path)
            except OSError as error:
                warnings.append(f"Could not remove old report '{path}': {error}")
    return removed, warnings
