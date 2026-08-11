from __future__ import annotations

import dataclasses
import json
import os
import sqlite3
from collections.abc import Callable, Iterable
from contextlib import closing
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Protocol

from osxphotos import PhotosDB
from osxphotos.cli.export import export_cli
from osxphotos.export_db import OSXPHOTOS_EXPORTDB_VERSION
from osxphotos.export_db_utils import export_db_migrate_photos_library

from photos_backup.apple_photos.identity import AssetIdentity
from photos_backup.archive.errors import ArchiveUnsafe

ExportRunner = Callable[[dict[str, Any]], int]
AssetReader = Callable[[Path], tuple[AssetIdentity, ...]]
VersionReader = Callable[[Path], str | None]
IntegrityChecker = Callable[[Path], str]

INTEGRITY_OK = "ok"

# Successive osxphotos export-database layouts for the same per-asset record.
_PHOTOINFO_QUERIES = (
    "SELECT uuid, photoinfo FROM photoinfo",
    "SELECT uuid, json_info FROM info",
)
# Successive layouts for the record of one written destination file.
_EXPORT_FILE_QUERIES = (
    "SELECT filepath, uuid, dest_size, dest_mtime FROM export_data",
    "SELECT filepath, uuid, orig_size, orig_mtime FROM files",
)


@dataclass(frozen=True)
class ExportedFile:
    """One destination file as the export database remembers writing it."""

    path: Path
    uuid: str
    size: int | None
    mtime: float | None


ExportFileReader = Callable[[Path], tuple[ExportedFile, ...]]


class MigrationRunner(Protocol):
    def __call__(
        self, export_db: Path, library: Path, *, dry_run: bool
    ) -> tuple[int, int]: ...


def run_osxphotos_export(arguments: dict[str, Any]) -> int:
    """Run one osxphotos export; returns 0 when osxphotos saw no error."""
    return export_cli(**arguments)


def read_photos_library(library: Path) -> tuple[AssetIdentity, ...]:
    """Enumerate every asset in a Photos library with its iCloud cloud GUID."""
    photosdb = PhotosDB(dbfile=str(library))
    return tuple(
        AssetIdentity(
            uuid=photo.uuid,
            cloud_guid=photo.cloud_guid,
            original_filename=photo.original_filename,
        )
        for photo in photosdb.photos()
    )


def read_export_db_assets(export_db: Path) -> tuple[AssetIdentity, ...]:
    """Read every asset the export database remembers, across database versions."""
    if not export_db.exists():
        return ()
    with _connect(export_db) as connection:
        rows = _select_first(connection, _PHOTOINFO_QUERIES, export_db, "asset records")
    return tuple(_asset_from_photoinfo(uuid, raw) for uuid, raw in rows)


def read_export_db_files(export_db: Path) -> tuple[ExportedFile, ...]:
    """Read every destination file the export database wrote, across versions."""
    if not export_db.exists():
        return ()
    with _connect(export_db) as connection:
        rows = _select_first(
            connection, _EXPORT_FILE_QUERIES, export_db, "exported file records"
        )
    return tuple(
        ExportedFile(path=Path(filepath), uuid=uuid, size=size, mtime=mtime)
        for filepath, uuid, size, mtime in rows
    )


def resolve_export_files(
    files: Iterable[ExportedFile], archive: Path
) -> tuple[ExportedFile, ...]:
    """Anchor export-database paths, which osxphotos stores relative to the export root.

    A path that escapes the archive is normalized rather than dropped, so a
    caller sees it and refuses it instead of never noticing it.
    """
    return tuple(
        dataclasses.replace(record, path=_anchor(record.path, archive))
        for record in files
    )


def _anchor(path: Path, archive: Path) -> Path:
    return Path(os.path.normpath(path if path.is_absolute() else archive / path))


def check_export_db_integrity(export_db: Path) -> str:
    """Return sqlite's `integrity_check` verdict; `ok` means the file is sound."""
    if not export_db.exists():
        return f"'{export_db}' does not exist"
    with _connect(export_db) as connection:
        try:
            row = connection.execute("PRAGMA integrity_check").fetchone()
        except sqlite3.Error as error:
            return str(error)
    if not row or not row[0]:
        return "integrity_check returned nothing"
    return str(row[0])


def read_export_db_version(export_db: Path) -> str | None:
    """Return the export-database schema version, or None when it cannot be read."""
    if not export_db.exists():
        return None
    with _connect(export_db) as connection:
        try:
            row = connection.execute(
                "SELECT exportdb FROM version ORDER BY id DESC LIMIT 1"
            ).fetchone()
        except sqlite3.Error:
            return None
    if not row or not row[0]:
        return None
    return str(row[0])


def export_db_version_supported(version: str | None) -> bool:
    """True when this build of osxphotos understands that database version."""
    if version is None:
        return False
    try:
        return float(version) <= float(OSXPHOTOS_EXPORTDB_VERSION)
    except ValueError:
        return False


def run_export_db_migration(
    export_db: Path, library: Path, *, dry_run: bool
) -> tuple[int, int]:
    """Repoint the export database at another Photos library; (matched, unmatched)."""
    return export_db_migrate_photos_library(
        dbfile=str(export_db), photos_library=str(library), dry_run=dry_run
    )


@dataclass(frozen=True)
class PhotosProbes:
    """Photos library and export-database access, injectable for tests."""

    read_library: AssetReader = read_photos_library
    read_export_db: AssetReader = read_export_db_assets
    read_export_db_version: VersionReader = read_export_db_version
    read_export_files: ExportFileReader = read_export_db_files
    check_integrity: IntegrityChecker = check_export_db_integrity
    migrate: MigrationRunner = run_export_db_migration


def _connect(export_db: Path) -> closing[sqlite3.Connection]:
    try:
        return closing(sqlite3.connect(str(export_db)))
    except sqlite3.Error as error:
        raise ArchiveUnsafe(
            f"Could not open export database '{export_db}': {error}"
        ) from error


def _select_first(
    connection: sqlite3.Connection,
    queries: tuple[str, ...],
    export_db: Path,
    what: str,
) -> list[tuple]:
    """Run the first query the database understands, refusing a file with none."""
    last: sqlite3.Error | None = None
    for query in queries:
        try:
            return connection.execute(query).fetchall()
        except sqlite3.Error as error:
            last = error
    raise ArchiveUnsafe(
        f"Export database '{export_db}' holds no readable {what}: {last}"
    )


def _asset_from_photoinfo(uuid: str, raw: str | None) -> AssetIdentity:
    try:
        document = json.loads(raw or "")
    except (TypeError, ValueError):
        document = {}
    if not isinstance(document, dict):
        document = {}
    return AssetIdentity(
        uuid=uuid,
        cloud_guid=document.get("cloud_guid") or None,
        original_filename=document.get("original_filename") or "",
    )
