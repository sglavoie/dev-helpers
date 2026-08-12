from __future__ import annotations

import dataclasses
import datetime
import tempfile
import time
from pathlib import Path
from typing import TYPE_CHECKING, Any

from photos_backup.apple_photos.adapter import ExportRunner, run_osxphotos_export
from photos_backup.apple_photos.late_additions import (
    MetadataReader,
    generate_late_photo_additions_report,
    read_spotlight_metadata,
)
from photos_backup.apple_photos.plan import (
    ExportPlan,
    ExportResult,
    export_arguments,
    plan_export,
)
from photos_backup.summary import read_export_report

if TYPE_CHECKING:
    from photos_backup.archive import Archive
    from photos_backup.config import ApplePhotosConfig

DRY_RUN_REPORT_NAME = "photos_export.csv"


class ApplePhotosExport:
    """Export Apple Photos straight into the shared archive under its lock."""

    def __init__(
        self,
        config: ApplePhotosConfig,
        archive: Archive,
        *,
        verbose: bool = False,
        limit: int = 0,
        plan: ExportPlan | None = None,
        runner: ExportRunner | None = None,
        extra_arguments: dict[str, Any] | None = None,
        metadata_reader: MetadataReader | None = None,
        plan_only: bool = False,
    ) -> None:
        self.config = config
        self.archive = archive
        self.verbose = verbose
        self.limit = limit
        self.plan = plan
        self.runner = runner or run_osxphotos_export
        self.extra_arguments = dict(extra_arguments or {})
        self.metadata_reader = metadata_reader or read_spotlight_metadata
        self.plan_only = plan_only

    def export(self) -> ExportResult:
        now = self.archive.now()
        plan = self.plan or plan_export(
            self.config, self.archive.state_store.load(), now
        )
        if self.plan_only:
            return ExportResult(plan=plan, exit_code=0, performed=False)
        if self.archive.dry_run:
            with tempfile.TemporaryDirectory() as scratch:
                return self._run(plan, now, Path(scratch) / DRY_RUN_REPORT_NAME, None)

        paths = self.archive.paths
        hostname = self.archive.hostname
        day = now.date()
        sequence = paths.next_report_sequence(hostname, day)
        return self._run(
            plan,
            now,
            paths.export_report(hostname, day, sequence),
            paths.late_additions_report(hostname, day, sequence),
        )

    def _run(
        self,
        plan: ExportPlan,
        now: datetime.datetime,
        report_path: Path,
        late_additions_path: Path | None,
    ) -> ExportResult:
        arguments = export_arguments(
            self.config,
            plan,
            dest=self.archive.paths.archive,
            export_db=self.archive.paths.export_db,
            report_path=report_path,
            dry_run=self.archive.dry_run,
            verbose=self.verbose,
            limit=self.limit,
        )
        arguments.update(self.extra_arguments)

        start = time.monotonic()
        exit_code = self.runner(arguments)
        elapsed = time.monotonic() - start

        report = read_export_report(report_path)
        late_additions_rows = 0
        if late_additions_path is not None:
            late_additions_rows = generate_late_photo_additions_report(
                export_report_path=report_path,
                output_path=late_additions_path,
                spouse_device_models=self.config.spouse_device_models,
                metadata_reader=self.metadata_reader,
            )

        result = ExportResult(
            plan=plan,
            exit_code=exit_code,
            counts=report.counts,
            report_path=report_path,
            late_additions_path=late_additions_path,
            late_additions_rows=late_additions_rows,
            elapsed_seconds=elapsed,
            report_problem=report.problem,
        )
        if result.clean and not self.archive.dry_run:
            self._advance_state(plan, now, report_path)
            result = dataclasses.replace(result, state_advanced=True)
        return result

    def _advance_state(
        self,
        plan: ExportPlan,
        now: datetime.datetime,
        report_path: Path,
    ) -> None:
        changes: dict[str, Any] = {
            "last_successful_export_at": now,
            "last_report_path": report_path,
        }
        if plan.is_full:
            changes["last_full_export_at"] = now
        self.archive.state_store.update(**changes)
