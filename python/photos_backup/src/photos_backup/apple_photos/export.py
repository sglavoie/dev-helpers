from __future__ import annotations

import csv
import dataclasses
import datetime
import tempfile
import time
from contextlib import nullcontext
from pathlib import Path
from typing import TYPE_CHECKING, Any

import click

from photos_backup.apple_photos.adapter import ExportRunner, run_osxphotos_export
from photos_backup.apple_photos.attempt import ExportAttemptStore
from photos_backup.apple_photos.late_additions import (
    MetadataReader,
    generate_late_photo_additions_report,
    read_spotlight_metadata,
)
from photos_backup.apple_photos.plan import (
    ExportMode,
    ExportPlan,
    ExportResult,
    export_arguments,
    plan_export,
)
from photos_backup.progress import ExportProgress
from photos_backup.summary import read_export_report
from photos_backup.space import print_destination_space

if TYPE_CHECKING:
    from photos_backup.archive import Archive
    from photos_backup.config import ApplePhotosConfig

DRY_RUN_REPORT_NAME = "photos_export.csv"

# These options would bypass the archive's path, identity, or evidence checks.
ARCHIVE_MANAGED_ARGUMENTS = frozenset(
    {
        "dest",
        "db",
        "cli_db",
        "alt_db",
        "exportdb",
        "no_exportdb",
        "ignore_exportdb",
        "ramdb",
        "dry_run",
        "cleanup",
        "cleanup_command",
        "cleanup_command_error",
        "report",
        "append",
        "load_config",
        "save_config",
        "config_only",
    }
)


def validate_export_overrides(arguments: dict[str, Any]) -> None:
    blocked = ARCHIVE_MANAGED_ARGUMENTS.intersection(arguments)
    if blocked:
        flags = ", ".join(f"--{name.replace('_', '-')}" for name in sorted(blocked))
        raise click.UsageError(
            f"Archive-managed option(s) cannot be forwarded: {flags}. "
            "Use photos-backup configuration, --volume, --dry-run, and the "
            "cleanup commands instead."
        )


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
        progress: ExportProgress | None = None,
    ) -> None:
        self.config = config
        self.archive = archive
        self.verbose = verbose
        self.limit = limit
        self.plan = plan
        self.runner = runner or run_osxphotos_export
        self.extra_arguments = dict(extra_arguments or {})
        validate_export_overrides(self.extra_arguments)
        self.metadata_reader = metadata_reader or read_spotlight_metadata
        self.plan_only = plan_only
        self.progress = progress

    def export(self) -> ExportResult:
        now = self.archive.now()
        plan = self.plan or plan_export(
            self.config, self.archive.state_store.load(), now
        )
        print_destination_space(self.archive.paths.archive)
        if self.config.exclude_hidden:
            click.echo(
                "Hidden photos and videos are excluded from this backup (exclude_hidden = true)."
            )
        if self.plan_only:
            return ExportResult(plan=plan, exit_code=0, performed=False)
        if self.archive.dry_run:
            with tempfile.TemporaryDirectory() as scratch:
                return self._run(plan, now, Path(scratch) / DRY_RUN_REPORT_NAME, None)

        paths = self.archive.paths
        hostname = self.archive.hostname
        day = now.astimezone().date()  # Report names carry the local date.
        sequence = paths.next_report_sequence(hostname, day)
        report_path = paths.export_report(hostname, day, sequence)
        return ExportAttemptStore(self.archive).run(
            plan,
            report_path,
            lambda: self._run(
                plan,
                now,
                report_path,
                paths.late_additions_report(hostname, day, sequence),
            ),
            restricted=bool(self.extra_arguments or self.limit),
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

        with (
            self.progress.phase("Generating reports")
            if self.progress
            else nullcontext()
        ):
            report = read_export_report(report_path)
            late_additions_rows = 0
            if report.problem is not None:
                late_additions_path = None
            if late_additions_path is not None:
                try:
                    late_additions_rows = generate_late_photo_additions_report(
                        export_report_path=report_path,
                        output_path=late_additions_path,
                        spouse_device_models=self.config.spouse_device_models,
                        metadata_reader=self.metadata_reader,
                        warning=self.progress.message if self.progress else None,
                    )
                except (OSError, UnicodeError, csv.Error) as error:
                    # This supplementary report is not evidence of export success.
                    warning = (
                        f"Warning: late-additions report '{late_additions_path}' "
                        f"could not be completed and may be partial: {error}. "
                        "Export status is based on the main export report."
                    )
                    if self.progress:
                        self.progress.message(warning)
                    else:
                        click.echo(warning, err=True)
                    late_additions_path = None

        result = ExportResult(
            plan=plan,
            exit_code=exit_code,
            counts=report.counts,
            report_path=report_path,
            late_additions_path=late_additions_path,
            late_additions_rows=late_additions_rows,
            elapsed_seconds=time.monotonic() - start,
            phase_timings=dict(self.progress.timings) if self.progress else {},
            report_problem=report.problem,
        )
        if (
            result.complete
            and not self.archive.dry_run
            and plan.mode is not ExportMode.RECENT
            # Custom osxphotos flags can restrict assets or exported components.
            # A clean report only proves completion of that custom request.
            and not self.extra_arguments
            and not self.limit
        ):
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
