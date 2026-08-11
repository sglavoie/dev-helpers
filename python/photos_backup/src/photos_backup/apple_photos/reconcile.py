from __future__ import annotations

from collections.abc import Iterable
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.apple_photos.identity import within_absence_limits

if TYPE_CHECKING:
    from photos_backup.apple_photos.adapter import ExportedFile
    from photos_backup.apple_photos.identity import AssetIdentity, LibraryComparison

# How many offending paths to name before the message stops being useful.
_PATH_SAMPLE = 3


@dataclass(frozen=True)
class ArchiveFile:
    """One file that is in the archive right now, with its current signature."""

    path: Path
    size: int
    mtime: float


@dataclass(frozen=True)
class CandidateFile:
    """One archive file that only an asset the library lost still claims."""

    path: Path
    uuid: str
    size: int
    mtime: float


@dataclass(frozen=True)
class Reconciliation:
    """What reconciling the archive against the library would and would not do."""

    absent: tuple[AssetIdentity, ...]
    candidates: tuple[CandidateFile, ...]
    changed: tuple[Path, ...]
    unknown: tuple[Path, ...]
    ambiguous: tuple[Path, ...]

    @property
    def paths(self) -> tuple[Path, ...]:
        return tuple(candidate.path for candidate in self.candidates)


def plan_reconciliation(
    comparison: LibraryComparison,
    files: Iterable[ExportedFile],
    present: Iterable[ArchiveFile],
) -> Reconciliation:
    """Map the assets a library lost onto the archive files only they claim.

    A file is a deletion candidate only when exactly one export-database asset
    claims it, that asset is gone from the library, and the file still carries
    the size and modification time osxphotos recorded for it.
    """
    absent = {asset.uuid for asset in comparison.absent}
    on_disk = {item.path: item for item in present}

    claimants: dict[Path, set[str]] = {}
    records: dict[Path, ExportedFile] = {}
    for record in files:
        claimants.setdefault(record.path, set()).add(record.uuid)
        records[record.path] = record

    candidates: list[CandidateFile] = []
    changed: list[Path] = []
    ambiguous: list[Path] = []
    for path, uuids in claimants.items():
        if not uuids & absent:
            continue
        if len(uuids) > 1:
            ambiguous.append(path)
            continue
        current = on_disk.get(path)
        if current is None:
            continue
        record = records[path]
        if not _signature_matches(record, current):
            changed.append(path)
            continue
        candidates.append(
            CandidateFile(
                path=path,
                uuid=record.uuid,
                size=current.size,
                mtime=current.mtime,
            )
        )

    return Reconciliation(
        absent=comparison.absent,
        candidates=tuple(sorted(candidates, key=lambda item: item.path)),
        changed=tuple(sorted(changed)),
        unknown=tuple(sorted(set(on_disk) - set(claimants))),
        ambiguous=tuple(sorted(ambiguous)),
    )


def approval_reason(
    reconciliation: Reconciliation,
    comparison: LibraryComparison,
    *,
    max_absent_assets: int,
    max_absent_fraction: float,
) -> str | None:
    """Why a person has to approve these deletions, or None when nobody has to."""
    if reconciliation.unknown:
        return (
            f"{len(reconciliation.unknown)} archive file(s) have no export-database "
            f"record ({_sample(reconciliation.unknown)})"
        )
    if reconciliation.ambiguous:
        return (
            f"{len(reconciliation.ambiguous)} archive file(s) are claimed by more "
            f"than one export-database asset ({_sample(reconciliation.ambiguous)})"
        )
    if reconciliation.changed:
        return (
            f"{len(reconciliation.changed)} deletion candidate(s) no longer carry "
            "the size and modification time osxphotos recorded "
            f"({_sample(reconciliation.changed)})"
        )
    if not within_absence_limits(
        comparison,
        max_absent_assets=max_absent_assets,
        max_absent_fraction=max_absent_fraction,
    ):
        return (
            f"{comparison.absent_count} of {comparison.comparable} export-database "
            f"record(s) ({comparison.absent_fraction:.3%}) left the library, over "
            f"the limit of {max_absent_assets} asset(s) or {max_absent_fraction:.3%}"
        )
    return None


def _signature_matches(record: ExportedFile, current: ArchiveFile) -> bool:
    if record.size is None or record.mtime is None:
        return False
    return record.size == current.size and int(record.mtime) == int(current.mtime)


def _sample(paths: tuple[Path, ...]) -> str:
    names = [str(path) for path in paths[:_PATH_SAMPLE]]
    remaining = len(paths) - len(names)
    if remaining > 0:
        names.append(f"and {remaining} more")
    return ", ".join(names)
