from __future__ import annotations

import os
import sqlite3
from contextlib import closing
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.apple_photos.adapter import (
    PhotosProbes,
    export_db_version_supported,
    unsupported_export_db_reason,
)
from photos_backup.apple_photos.identity import (
    AssetIdentity,
    IdentityVerdict,
    WriterStatus,
    assess_library,
    compare_library,
)
from photos_backup.apple_photos.files import is_ignored
from photos_backup.archive.errors import ArchiveUnavailable, ArchiveUnsafe
from photos_backup.archive.paths import METADATA_DIR_NAME, ArchivePaths
from photos_backup.errors import ActionRequired

if TYPE_CHECKING:
    from photos_backup.archive import Archive
    from photos_backup.config import ApplePhotosConfig

SQLITE_SIDECAR_SUFFIXES = ("-wal", "-shm")

# How many stray names to name before the message stops being useful.
_FOREIGN_SAMPLE = 5


@dataclass(frozen=True)
class MigrationOutcome:
    """What repointing the export database at another library did."""

    performed: bool
    matched: int
    unmatched: int
    backup_path: Path | None = None


@dataclass(frozen=True)
class TakeoverCheck:
    """Whether this Mac may write the archive, and what it had to do to get there."""

    status: WriterStatus
    hostname: str
    previous_hostname: str | None = None
    verdict: IdentityVerdict | None = None
    migration: MigrationOutcome | None = None

    @property
    def deletion_candidates(self) -> tuple[AssetIdentity, ...]:
        if self.verdict is None:
            return ()
        return self.verdict.deletion_candidates


def ensure_writer(
    config: ApplePhotosConfig,
    archive: Archive,
    *,
    probes: PhotosProbes | None = None,
) -> TakeoverCheck:
    """Let this Mac write the archive, migrating the export database if it must.

    An unchanged writer costs one state read and a schema-version read, and
    never opens the Photos library.
    """
    active = probes or PhotosProbes()
    previous = archive.state_store.load().writer_hostname
    host = archive.hostname
    export_db = archive.paths.export_db
    if previous == host:
        if export_db.exists():
            version = active.read_export_db_version(export_db)
            if not export_db_version_supported(version):
                raise ActionRequired(
                    f"Export database '{export_db}': "
                    f"{unsupported_export_db_reason(version)} before exporting again"
                )
        return TakeoverCheck(
            status=WriterStatus.UNCHANGED, hostname=host, previous_hostname=previous
        )

    if not archive.paths.state_file.exists() and not export_db.exists():
        # Never claim, and then export into, a folder that holds someone else's files.
        refuse_foreign_archive(archive.paths, "photos-backup")

    recorded = active.read_export_db(export_db)
    if not recorded:
        archive.state_store.update(writer_hostname=host)
        return TakeoverCheck(
            status=WriterStatus.CLAIMED, hostname=host, previous_hostname=previous
        )

    version = active.read_export_db_version(export_db)
    if not export_db_version_supported(version):
        raise ActionRequired(
            f"Export database '{export_db}': {unsupported_export_db_reason(version)} "
            f"on '{host}' before taking over the archive"
        )

    library_assets = active.read_library(config.library)
    verdict = assess_library(
        compare_library(recorded, library_assets),
        max_absent_assets=config.cleanup_max_assets,
        max_absent_fraction=config.cleanup_max_fraction,
    )
    if verdict.blocked:
        raise ActionRequired(
            f"'{host}' may not take over the archive from "
            f"'{previous or 'an unknown host'}': {verdict.blocked_reason}"
        )

    migration = None
    if verdict.migration_required:
        migration = migrate_export_db(
            archive, config.library, library_assets=library_assets, probes=active
        )

    archive.state_store.update(writer_hostname=host)
    return TakeoverCheck(
        status=WriterStatus.TAKEOVER if previous else WriterStatus.CLAIMED,
        hostname=host,
        previous_hostname=previous,
        verdict=verdict,
        migration=migration,
    )


def refuse_foreign_archive(paths: ArchivePaths, command: str) -> None:
    """Refuse an archive with contents but no photos-backup metadata."""
    foreign = _foreign_entries(paths)
    if foreign:
        listed = ", ".join(f"'{name}'" for name in foreign[:_FOREIGN_SAMPLE])
        raise ActionRequired(
            f"Archive '{paths.archive}' already holds {len(foreign)} entry(ies) "
            f"({listed}) but no photos-backup metadata; {command} only accepts an "
            "empty archive or one it left incomplete itself, so move the existing "
            "contents aside or point [apple_photos] archive somewhere empty"
        )


def _foreign_entries(paths: ArchivePaths) -> tuple[str, ...]:
    """Archive entries photos-backup does not own, in a stable order."""
    try:
        entries = sorted(paths.archive.iterdir())
    except FileNotFoundError:
        return ()
    except OSError as error:
        # An unreadable archive must not pass for an empty one.
        raise ArchiveUnavailable(
            f"Could not list archive '{paths.archive}': {error}"
        ) from error
    return tuple(
        entry.name
        for entry in entries
        if entry.name != METADATA_DIR_NAME and not is_ignored(entry)
    )


def migrate_export_db(
    archive: Archive,
    library: Path,
    *,
    library_assets: tuple[AssetIdentity, ...],
    probes: PhotosProbes | None = None,
) -> MigrationOutcome:
    """Repoint the export database at `library` behind one restorable backup."""
    active = probes or PhotosProbes()
    export_db = archive.paths.export_db
    if archive.dry_run:
        matched, unmatched = active.migrate(export_db, library, dry_run=True)
        return MigrationOutcome(performed=False, matched=matched, unmatched=unmatched)

    backup = archive.paths.export_db_backup
    copy_database(export_db, backup)
    try:
        matched, unmatched = active.migrate(export_db, library, dry_run=False)
        stale = compare_library(
            active.read_export_db(export_db), library_assets
        ).uuid_changed
        if stale:
            raise ArchiveUnsafe(
                f"{stale} record(s) still point at the previous library"
            )
    except BaseException as error:
        # Restore on Ctrl-C too, so an interrupted migration never leaves the
        # export database half-repointed; only ordinary failures are reworded.
        copy_database(backup, export_db)
        if not isinstance(error, Exception):
            raise
        raise ActionRequired(
            f"Migrating export database '{export_db}' to library '{library}' "
            f"failed and was restored from '{backup}': {error}"
        ) from error

    return MigrationOutcome(
        performed=True, matched=matched, unmatched=unmatched, backup_path=backup
    )


def copy_database(source: Path, destination: Path) -> None:
    """Replace `destination` with a consistent copy of a live SQLite database."""
    destination.parent.mkdir(parents=True, exist_ok=True)
    temporary = destination.with_name(f".{destination.name}.{os.getpid()}.tmp")
    _remove(temporary)
    try:
        with (
            closing(sqlite3.connect(str(source))) as origin,
            closing(sqlite3.connect(str(temporary))) as copy,
        ):
            origin.backup(copy)
        os.replace(temporary, destination)
        _remove_sidecars(destination)
    except (OSError, sqlite3.Error) as error:
        _remove(temporary)
        raise ArchiveUnavailable(
            f"Could not copy export database '{source}' to '{destination}': {error}"
        ) from error


def _remove(path: Path) -> None:
    path.unlink(missing_ok=True)
    _remove_sidecars(path)


def _remove_sidecars(path: Path) -> None:
    for suffix in SQLITE_SIDECAR_SUFFIXES:
        path.with_name(path.name + suffix).unlink(missing_ok=True)
