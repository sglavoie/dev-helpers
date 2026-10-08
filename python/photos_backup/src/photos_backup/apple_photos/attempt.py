"""Latest export attempt; informational evidence, never an input to cadence."""

from __future__ import annotations

import json
from collections.abc import Callable
from datetime import datetime
from pathlib import Path
from typing import TYPE_CHECKING

import click

from photos_backup.archive.errors import ArchiveUnavailable
from photos_backup.archive.paths import is_archive_relative
from photos_backup.archive.state import write_atomic
from photos_backup.cli.context import export_retry_arguments

if TYPE_CHECKING:
    from photos_backup.apple_photos.plan import ExportPlan, ExportResult
    from photos_backup.archive import Archive


class ExportAttemptStore:
    """Read under the archive read lock, write under its existing writer lock."""

    def __init__(self, archive: Archive):
        self.archive = archive
        self.path = archive.paths.metadata / "last-export-attempt.json"

    def read(self) -> tuple[dict | None, str | None]:
        try:
            if self.path.is_symlink():
                raise ValueError("receipt must not be a symlink")
            document = json.loads(self.path.read_text(encoding="utf-8"))
            _validate(document)
            document["report_path"] = str(
                self.archive.paths.archive / document["report_path"]
            )
            if document.get("download_report_path") is not None:
                document["download_report_path"] = str(
                    self.archive.paths.archive / document["download_report_path"]
                )
            return document, None
        except FileNotFoundError:
            return None, None
        except (OSError, ValueError) as error:
            return None, f"Could not read export attempt '{self.path}': {error}"

    def run(
        self,
        plan: ExportPlan,
        report_path: Path,
        operation: Callable[[], ExportResult],
        *,
        restricted: bool,
    ) -> ExportResult:
        if self.archive.dry_run:
            return operation()
        previous, _ = self.read()
        last_success = previous.get("last_successful_export_at") if previous else None
        if previous and previous["status"] == "succeeded":
            last_success = previous["completed_at"]
        attempt = {
            "last_successful_export_at": last_success,
            "version": 1,
            "started_at": self.archive.now().isoformat(),
            "completed_at": None,
            "hostname": self.archive.hostname,
            "mode": plan.mode.value,
            "restricted": restricted,
            "report_path": str(report_path.relative_to(self.archive.paths.archive)),
            "status": "started",
            "error": None,
            "baseline_advanced": False,
            "missing_count": None,
            "error_count": None,
            "download_report_path": str(
                report_path.with_suffix(".downloads.json").relative_to(
                    self.archive.paths.archive
                )
            ),
            "retry_arguments": None if restricted else export_retry_arguments(),
        }
        self._save(attempt)
        try:
            result = operation()
        except BaseException as error:
            self._save(
                {
                    **attempt,
                    "completed_at": self.archive.now().isoformat(),
                    "status": "failed"
                    if isinstance(error, Exception)
                    else "interrupted",
                    "error": str(error) or type(error).__name__,
                }
            )
            raise
        completed_at = self.archive.now().isoformat()
        self._save(
            {
                **attempt,
                "last_successful_export_at": completed_at
                if result.complete
                else last_success,
                "completed_at": completed_at,
                "status": "succeeded" if result.complete else "failed",
                "error": result.failure_reason(),
                "baseline_advanced": result.state_advanced,
                "missing_count": result.missing_count
                if result.report_problem is None
                else None,
                "error_count": result.error_count
                if result.report_problem is None
                else None,
            }
        )
        return result

    def _save(self, document: dict) -> None:
        """Receipt I/O must not change or hide the export's outcome."""
        try:
            write_atomic(self.path, json.dumps(document, indent=2) + "\n")
        except ArchiveUnavailable as error:
            click.echo(f"Warning: could not save export attempt: {error}", err=True)


def _validate(document: object) -> None:
    if not isinstance(document, dict):
        raise ValueError("export attempt must be an object")
    if type(document.get("version")) is not int or document["version"] != 1:
        raise ValueError("unsupported export attempt version")
    if document.get("status") not in ("started", "succeeded", "failed", "interrupted"):
        raise ValueError("invalid export attempt status")
    if document.get("mode") not in ("full", "incremental", "recent"):
        raise ValueError("invalid export mode")
    for field in ("hostname", "report_path"):
        if not isinstance(document.get(field), str) or not document[field]:
            raise ValueError(f"invalid {field}")
    report = Path(document["report_path"])
    if not is_archive_relative(report):
        raise ValueError("report path must be relative to the archive")
    _validate_details(document)
    if any(
        type(document.get(field)) is not bool
        for field in ("restricted", "baseline_advanced")
    ):
        raise ValueError("restricted and baseline_advanced must be booleans")
    if document.get("error") is not None and not isinstance(document["error"], str):
        raise ValueError("invalid export error")
    for field in ("started_at", "completed_at"):
        value = document.get(field)
        if (
            field == "completed_at"
            and value is None
            and document["status"] == "started"
        ):
            continue
        if not isinstance(value, str) or datetime.fromisoformat(value).tzinfo is None:
            raise ValueError(f"invalid {field}")


def _validate_details(document: dict) -> None:
    success = document.get("last_successful_export_at")
    if success is not None and (
        not isinstance(success, str) or datetime.fromisoformat(success).tzinfo is None
    ):
        raise ValueError("invalid last_successful_export_at")
    download_report = document.get("download_report_path")
    if download_report is not None:
        if not isinstance(download_report, str):
            raise ValueError("invalid download report path")
        path = Path(download_report)
        if not is_archive_relative(path):
            raise ValueError("download report path must be relative to the archive")
    for field in ("missing_count", "error_count"):
        value = document.get(field)
        if value is not None and (type(value) is not int or value < 0):
            raise ValueError(f"invalid {field}")
    retry = document.get("retry_arguments")
    if retry is not None and (
        not isinstance(retry, list)
        or not retry
        or any(not isinstance(argument, str) or not argument for argument in retry)
    ):
        raise ValueError("invalid retry arguments")
