"""Latest export attempt; informational evidence, never an input to cadence."""

from __future__ import annotations

import json
import os
import tempfile
from collections.abc import Callable
from datetime import datetime
from pathlib import Path
from typing import TYPE_CHECKING

import click

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
        attempt = {
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
        self._save(
            {
                **attempt,
                "completed_at": self.archive.now().isoformat(),
                "status": "succeeded" if result.complete else "failed",
                "error": result.failure_reason(),
                "baseline_advanced": result.state_advanced,
            }
        )
        return result

    def _save(self, document: dict) -> None:
        """Receipt I/O must not change or hide the export's outcome."""
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(
                mode="w", encoding="utf-8", dir=self.path.parent, delete=False
            ) as handle:
                temporary = Path(handle.name)
                handle.write(json.dumps(document, indent=2) + "\n")
                handle.flush()
                os.fsync(handle.fileno())
            os.replace(temporary, self.path)
        except OSError as error:
            click.echo(
                f"Warning: could not save export attempt '{self.path}': {error}",
                err=True,
            )
        finally:
            if temporary is not None:
                try:
                    temporary.unlink(missing_ok=True)
                except OSError:
                    pass


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
    if report.is_absolute() or ".." in report.parts or report == Path("."):
        raise ValueError("report path must be relative to the archive")
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
