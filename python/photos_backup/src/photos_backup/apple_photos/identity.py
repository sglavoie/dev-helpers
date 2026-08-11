from __future__ import annotations

from collections.abc import Iterable
from dataclasses import dataclass
from enum import Enum


@dataclass(frozen=True)
class AssetIdentity:
    """One asset as either the export database or the Photos library sees it."""

    uuid: str
    cloud_guid: str | None
    original_filename: str

    @property
    def cloud_key(self) -> str | None:
        if not self.cloud_guid:
            return None
        return f"{self.original_filename}:{self.cloud_guid}"


@dataclass(frozen=True)
class LibraryComparison:
    """What the export database and the current library agree and disagree on."""

    recorded: int
    unidentifiable: int
    matched: int
    uuid_changed: int
    absent: tuple[AssetIdentity, ...]

    @property
    def comparable(self) -> int:
        return self.recorded - self.unidentifiable

    @property
    def absent_count(self) -> int:
        return len(self.absent)

    @property
    def absent_fraction(self) -> float:
        if not self.comparable:
            return 0.0
        return self.absent_count / self.comparable


@dataclass(frozen=True)
class CoverageReport:
    """How much of the Photos library the export database already accounts for."""

    library: int
    recorded: int
    unrecorded: tuple[AssetIdentity, ...]

    @property
    def complete(self) -> bool:
        return not self.unrecorded


class WriterStatus(Enum):
    UNCHANGED = "unchanged"
    CLAIMED = "claimed"
    TAKEOVER = "takeover"


@dataclass(frozen=True)
class IdentityVerdict:
    """Whether this library may be used, and what it would cost to use it."""

    comparison: LibraryComparison
    blocked_reason: str | None = None
    migration_required: bool = False
    deletion_candidates: tuple[AssetIdentity, ...] = ()

    @property
    def blocked(self) -> bool:
        return self.blocked_reason is not None


def compare_library(
    recorded: Iterable[AssetIdentity],
    library: Iterable[AssetIdentity],
) -> LibraryComparison:
    """Match export-database records to library assets through iCloud cloud GUIDs."""
    by_cloud_key = {
        asset.cloud_key: asset for asset in library if asset.cloud_key is not None
    }

    total = 0
    unidentifiable = 0
    matched = 0
    uuid_changed = 0
    absent: list[AssetIdentity] = []
    for asset in recorded:
        total += 1
        key = asset.cloud_key
        if key is None:
            unidentifiable += 1
            continue
        current = by_cloud_key.get(key)
        if current is None:
            absent.append(asset)
            continue
        matched += 1
        if current.uuid != asset.uuid:
            uuid_changed += 1

    return LibraryComparison(
        recorded=total,
        unidentifiable=unidentifiable,
        matched=matched,
        uuid_changed=uuid_changed,
        absent=tuple(absent),
    )


def assess_coverage(
    library: Iterable[AssetIdentity],
    recorded: Iterable[AssetIdentity],
) -> CoverageReport:
    """Find library assets the export database has no record of, matched by UUID.

    UUIDs are the right key here because coverage is only ever measured right
    after the Mac that owns the library exported it, so no migration sits
    between the two sides.
    """
    known = {item.uuid for item in recorded}
    assets = tuple(library)
    return CoverageReport(
        library=len(assets),
        recorded=len(known),
        unrecorded=tuple(item for item in assets if item.uuid not in known),
    )


def within_absence_limits(
    comparison: LibraryComparison,
    *,
    max_absent_assets: int,
    max_absent_fraction: float,
) -> bool:
    """True when the records absent from a library stay inside both caps."""
    return (
        comparison.absent_count <= max_absent_assets
        and comparison.absent_fraction <= max_absent_fraction
    )


def assess_library(
    comparison: LibraryComparison,
    *,
    max_absent_assets: int,
    max_absent_fraction: float,
) -> IdentityVerdict:
    """Decide whether a library is the right one, complete enough, and migratable."""
    if comparison.recorded == 0:
        return IdentityVerdict(comparison=comparison)

    if comparison.comparable == 0:
        return IdentityVerdict(
            comparison=comparison,
            blocked_reason=(
                f"none of the {comparison.recorded} export-database record(s) carry "
                "an iCloud cloud GUID, so this library cannot be identified"
            ),
        )

    if comparison.matched == 0:
        return IdentityVerdict(
            comparison=comparison,
            blocked_reason=(
                f"not one of the {comparison.comparable} export-database record(s) "
                "exists in this library, so it is a different library"
            ),
        )

    if not within_absence_limits(
        comparison,
        max_absent_assets=max_absent_assets,
        max_absent_fraction=max_absent_fraction,
    ):
        return IdentityVerdict(
            comparison=comparison,
            blocked_reason=(
                f"{comparison.absent_count} of {comparison.comparable} "
                f"export-database record(s) ({comparison.absent_fraction:.3%}) are "
                f"absent from this library, over the limit of {max_absent_assets} "
                f"asset(s) or {max_absent_fraction:.3%}"
            ),
        )

    return IdentityVerdict(
        comparison=comparison,
        migration_required=comparison.uuid_changed > 0,
        deletion_candidates=comparison.absent,
    )
