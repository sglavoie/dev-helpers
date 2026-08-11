from __future__ import annotations

import datetime
from dataclasses import dataclass
from typing import TYPE_CHECKING

from photos_backup.apple_photos.adapter import PhotosProbes
from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.identity import CoverageReport, assess_coverage
from photos_backup.apple_photos.late_additions import MetadataReader
from photos_backup.apple_photos.plan import ExportMode, ExportPlan, ExportResult
from photos_backup.apple_photos.takeover import TakeoverCheck, ensure_writer
from photos_backup.archive.paths import METADATA_DIR_NAME, ArchivePaths
from photos_backup.errors import ActionRequired

if TYPE_CHECKING:
    from photos_backup.apple_photos.adapter import ExportRunner
    from photos_backup.archive import Archive
    from photos_backup.archive.state import ArchiveState
    from photos_backup.config import ApplePhotosConfig

FRESH_REASON = "bootstrapping a fresh archive"
RESUME_REASON = "resuming an interrupted bootstrap"

# How many stray names to name before the message stops being useful.
_FOREIGN_SAMPLE = 5


@dataclass(frozen=True)
class BootstrapResult:
    """What one bootstrap attempt did, and whether it earned initialization."""

    takeover: TakeoverCheck
    export: ExportResult
    coverage: CoverageReport
    resumed: bool
    initialized_at: datetime.datetime | None = None

    @property
    def initialized(self) -> bool:
        return self.initialized_at is not None

    def blocking_reason(self) -> str | None:
        """Why this archive may not be initialized yet, or None when it may."""
        if not self.export.clean:
            return f"the export was not clean ({self.export.failure_reason()})"
        if self.export.missing_count:
            return (
                f"{self.export.missing_count} asset(s) could not be downloaded "
                "from iCloud, so the export is not complete"
            )
        if not self.coverage.complete:
            return (
                f"{len(self.coverage.unrecorded)} of {self.coverage.library} library "
                "asset(s) have no export-database record"
            )
        return None


def bootstrap_archive(
    config: ApplePhotosConfig,
    archive: Archive,
    *,
    probes: PhotosProbes | None = None,
    runner: ExportRunner | None = None,
    metadata_reader: MetadataReader | None = None,
) -> BootstrapResult:
    """Fill a fresh archive with one complete export, then initialize it.

    Nothing is imported from the legacy export: the archive is built only from
    the Photos library. An interrupted attempt is resumed by running this again,
    because state only advances on a clean export and `initialized_at` is only
    written once that export also covers the whole library.
    """
    active = probes or PhotosProbes()
    resumed = _require_bootstrappable(archive, archive.state_store.load())
    takeover = ensure_writer(config, archive, probes=active)

    plan = ExportPlan(ExportMode.FULL, RESUME_REASON if resumed else FRESH_REASON)
    export = ApplePhotosExport(
        config=config,
        archive=archive,
        plan=plan,
        runner=runner,
        metadata_reader=metadata_reader,
    ).export()

    coverage = assess_coverage(
        active.read_library(config.library),
        active.read_export_db(archive.paths.export_db),
    )
    result = BootstrapResult(
        takeover=takeover, export=export, coverage=coverage, resumed=resumed
    )
    if archive.dry_run or result.blocking_reason() is not None:
        return result

    now = archive.now()
    archive.state_store.update(initialized_at=now)
    return BootstrapResult(
        takeover=takeover,
        export=export,
        coverage=coverage,
        resumed=resumed,
        initialized_at=now,
    )


def _require_bootstrappable(archive: Archive, state: ArchiveState) -> bool:
    """Accept a new or half-finished archive; return True when resuming one."""
    paths = archive.paths
    if state.initialized:
        raise ActionRequired(
            f"Archive '{paths.archive}' was already initialized at "
            f"{state.initialized_at.isoformat()}; run 'photos-backup daily' to "
            "keep it current or 'photos-backup verify' to check its health"
        )

    recognized = paths.state_file.exists() or paths.export_db.exists()
    if recognized:
        return True

    foreign = _foreign_entries(paths)
    if foreign:
        listed = ", ".join(f"'{name}'" for name in foreign[:_FOREIGN_SAMPLE])
        raise ActionRequired(
            f"Archive '{paths.archive}' already holds {len(foreign)} entry(ies) "
            f"({listed}) but no photos-backup metadata; bootstrap only accepts an "
            "empty archive or one it left incomplete itself, so move the existing "
            "contents aside or point [apple_photos] archive somewhere empty"
        )
    return False


def _foreign_entries(paths: ArchivePaths) -> tuple[str, ...]:
    """Archive entries photos-backup does not own, in a stable order."""
    try:
        entries = sorted(paths.archive.iterdir())
    except OSError:
        return ()
    return tuple(entry.name for entry in entries if entry.name != METADATA_DIR_NAME)
