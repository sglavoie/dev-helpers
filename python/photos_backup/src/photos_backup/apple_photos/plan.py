from __future__ import annotations

import datetime
from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path
from typing import TYPE_CHECKING, Any

from photos_backup.summary import BackupSummary

if TYPE_CHECKING:
    from photos_backup.archive.state import ArchiveState
    from photos_backup.config import ApplePhotosConfig

DIRECTORY_TEMPLATE = "{created.year}/{created.mm}"
FILENAME_TEMPLATE = "{created.strftime,%Y-%m-%d-%H%M%S}_{original_name}"
SIDECAR_FORMATS = ("json",)
RETRY_ATTEMPTS = 3

ERROR_COUNT_FIELDS = ("error", "exiftool_error", "sidecar_user_error", "user_error")


class ExportMode(Enum):
    FULL = "full"
    INCREMENTAL = "incremental"
    RECENT = "recent"


@dataclass(frozen=True)
class ExportPlan:
    """Which osxphotos run this cadence calls for, and why."""

    mode: ExportMode
    reason: str
    from_date: datetime.datetime | None = None

    @property
    def is_full(self) -> bool:
        return self.mode is ExportMode.FULL


@dataclass(frozen=True)
class ExportResult:
    """What one export run did, and whether it earned a state advance."""

    plan: ExportPlan
    exit_code: int
    counts: dict[str, int] = field(default_factory=dict)
    report_path: Path | None = None
    late_additions_path: Path | None = None
    late_additions_rows: int = 0
    elapsed_seconds: float = 0.0
    state_advanced: bool = False
    report_problem: str | None = None
    performed: bool = True

    @property
    def error_count(self) -> int:
        return error_count(self.counts)

    @property
    def missing_count(self) -> int:
        return self.counts.get("missing", 0)

    @property
    def files_transferred(self) -> int:
        transferred = self.counts.get("new", 0) + self.counts.get("updated", 0)
        return transferred or self.counts.get("exported", 0)

    @property
    def clean(self) -> bool:
        return self.report_problem is None and is_clean(self.exit_code, self.counts)

    def summary(self) -> BackupSummary:
        return BackupSummary(
            step_name="Apple Photos",
            files_transferred=self.files_transferred,
            elapsed_seconds=self.elapsed_seconds,
            planned=not self.performed,
            error=None if self.clean else self.failure_reason(),
        )

    def failure_reason(self) -> str | None:
        if self.clean:
            return None
        if self.error_count:
            return f"osxphotos reported {self.error_count} export error(s)"
        if self.exit_code != 0:
            return f"osxphotos exited with status {self.exit_code}"
        return self.report_problem


def error_count(counts: dict[str, int]) -> int:
    return sum(counts.get(name, 0) for name in ERROR_COUNT_FIELDS)


def is_clean(exit_code: int, counts: dict[str, int]) -> bool:
    return exit_code == 0 and error_count(counts) == 0


def cadence_start(now: datetime.datetime, weekday: int) -> datetime.datetime:
    """Midnight of the most recent `weekday` on or before `now`."""
    day = now.date() - datetime.timedelta(days=(now.weekday() - weekday) % 7)
    return datetime.datetime.combine(day, datetime.time.min, tzinfo=now.tzinfo)


def plan_export(
    config: ApplePhotosConfig,
    state: ArchiveState,
    now: datetime.datetime,
) -> ExportPlan:
    """Choose a full or incremental export from durable state and the clock."""
    if state.last_full_export_at is None:
        return ExportPlan(ExportMode.FULL, "no full export has completed yet")
    if state.last_successful_export_at is None:
        return ExportPlan(ExportMode.FULL, "no successful export has completed yet")

    start = cadence_start(now, config.full_export_weekday)
    if state.last_full_export_at < start:
        return ExportPlan(ExportMode.FULL, f"no full export since {start.date()}")

    age = now - state.last_full_export_at
    if age >= datetime.timedelta(days=config.full_export_max_age_days):
        return ExportPlan(
            ExportMode.FULL, f"the last full export is {age.days} day(s) old"
        )

    from_date = state.last_successful_export_at - datetime.timedelta(
        days=config.incremental_overlap_days
    )
    return ExportPlan(
        ExportMode.INCREMENTAL,
        f"photos created since {from_date.date()}",
        from_date=from_date,
    )


def export_arguments(
    config: ApplePhotosConfig,
    plan: ExportPlan,
    *,
    dest: Path,
    export_db: Path,
    report_path: Path,
    dry_run: bool = False,
    verbose: bool = False,
    limit: int = 0,
) -> dict[str, Any]:
    """Build the exact osxphotos `export_cli` arguments for one run."""
    return {
        "dest": str(dest),
        "db": str(config.library),
        "exportdb": str(export_db),
        "report": str(report_path),
        "directory": DIRECTORY_TEMPLATE,
        "filename_template": FILENAME_TEMPLATE,
        "sidecar": SIDECAR_FORMATS,
        "album_keyword": True,
        "exiftool": True,
        "export_aae": True,
        "download_missing": True,
        "use_photokit": True,
        "retry": RETRY_ATTEMPTS,
        "update": True,
        "update_errors": True,
        "skip_bursts": False,
        "skip_edited": False,
        "skip_live": False,
        "skip_original_if_edited": False,
        "skip_raw": False,
        "cleanup": False,
        "from_date": plan.from_date,
        "dry_run": dry_run,
        "verbose_flag": verbose,
        "limit": limit,
    }
