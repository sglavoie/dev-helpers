from __future__ import annotations

import datetime
import re
from dataclasses import dataclass
from pathlib import Path

METADATA_DIR_NAME = ".photos-backup"
EXPORT_DB_NAME = "export.db"
EXPORT_DB_BACKUP_NAME = "export.db.last-known-good"
STATE_FILE_NAME = "state.json"
LOCK_FILE_NAME = "archive.lock"

_UNSAFE_NAME = re.compile(r"[^A-Za-z0-9_-]+")


def _sequence_suffix(sequence: int) -> str:
    return "" if sequence == 1 else f"_{sequence}"


def safe_name(raw: str) -> str:
    """Reduce a hostname or run identifier to a safe single filename part."""
    cleaned = _UNSAFE_NAME.sub("-", raw).strip("-")
    return cleaned or "unknown"


@dataclass(frozen=True)
class ArchivePaths:
    """Every path photos-backup owns inside the archive, derived from one root."""

    volume: Path
    archive: Path

    @property
    def metadata(self) -> Path:
        return self.archive / METADATA_DIR_NAME

    @property
    def reports(self) -> Path:
        return self.metadata / "reports"

    @property
    def migrations(self) -> Path:
        return self.metadata / "migrations"

    @property
    def cleanup(self) -> Path:
        return self.metadata / "cleanup"

    @property
    def export_db(self) -> Path:
        return self.metadata / EXPORT_DB_NAME

    @property
    def export_db_backup(self) -> Path:
        return self.migrations / EXPORT_DB_BACKUP_NAME

    @property
    def state_file(self) -> Path:
        return self.metadata / STATE_FILE_NAME

    @property
    def lock_file(self) -> Path:
        return self.metadata / LOCK_FILE_NAME

    @property
    def managed_directories(self) -> tuple[Path, ...]:
        return (self.metadata, self.reports, self.migrations, self.cleanup)

    def export_report(
        self, hostname: str, when: datetime.date, sequence: int = 1
    ) -> Path:
        stem = f"photos_export_{safe_name(hostname)}_{when}"
        return self.reports / f"{stem}{_sequence_suffix(sequence)}.csv"

    def late_additions_report(
        self, hostname: str, when: datetime.date, sequence: int = 1
    ) -> Path:
        stem = f"late_photo_additions_{safe_name(hostname)}_{when}"
        return self.reports / f"{stem}{_sequence_suffix(sequence)}.csv"

    def next_report_sequence(self, hostname: str, when: datetime.date) -> int:
        """The first sequence this host and day has written no report for.

        An export must not be able to read a report an earlier run left behind,
        so every invocation is given report paths that hold nothing yet.
        """
        sequence = 1
        while (
            self.export_report(hostname, when, sequence).exists()
            or self.late_additions_report(hostname, when, sequence).exists()
        ):
            sequence += 1
        return sequence

    def cleanup_manifest(self, run_id: str) -> Path:
        return self.cleanup / f"{safe_name(run_id)}.json"
