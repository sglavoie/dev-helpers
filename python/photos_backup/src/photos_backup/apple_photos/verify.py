from __future__ import annotations

import os
import stat
from collections.abc import Callable
from contextlib import nullcontext
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

from photos_backup.cli.context import suggested_command
from photos_backup.apple_photos.adapter import (
    INTEGRITY_OK,
    ExportedFile,
    PhotosProbes,
    export_db_version_supported,
    unsupported_export_db_reason,
    resolve_export_files,
)
from photos_backup.archive.errors import ArchiveError

if TYPE_CHECKING:
    from photos_backup.archive import Archive
    from photos_backup.archive.state import ArchiveState
    from photos_backup.progress import ExportProgress

EXPORT_DATABASE = "export database"
SIGNATURES = "signatures"
MISSING_ASSETS = "missing assets"
STATE = "state"
OWNERSHIP = "ownership"
PENDING_CLEANUP = "pending cleanup"

UNREADABLE_STATE = "the archive state could not be read"
UNREADABLE_EXPORT_DB = "the export database could not be read, so nothing was checked"

# How many offending paths to name before the message stops being useful.
_PATH_SAMPLE = 3


@dataclass(frozen=True)
class Check:
    """One archive property, whether it holds, and the evidence either way."""

    name: str
    passed: bool
    detail: str
    paths: tuple[Path, ...] = ()


@dataclass(frozen=True)
class VerificationReport:
    """Every check one verify run made, in the order it made them."""

    checks: tuple[Check, ...]

    @property
    def failed(self) -> tuple[Check, ...]:
        return tuple(check for check in self.checks if not check.passed)

    @property
    def passed(self) -> bool:
        return not self.failed


def verify_archive(
    archive: Archive,
    *,
    probes: PhotosProbes | None = None,
    progress: ExportProgress | None = None,
) -> VerificationReport:
    """Report archive health without writing anything.

    Every read that can fail is caught and reported as a failed check, so one
    corrupt file never hides the state of everything else.
    """
    active = probes or PhotosProbes()

    export_db = archive.paths.export_db
    files: tuple[ExportedFile, ...] = ()
    export_db_error: str | None = None
    with progress.phase("Reading export database") if progress else nullcontext():
        try:
            if not export_db.exists():
                export_db_error = f"'{export_db}' does not exist"
            else:
                files = resolve_export_files(
                    active.read_export_files(export_db), archive.paths.archive
                )
        except (ArchiveError, OSError) as error:
            export_db_error = str(error)

    state: ArchiveState | None = None
    state_error: str | None = None
    try:
        state = archive.state_store.load()
    except (ArchiveError, OSError) as error:
        state_error = str(error)

    if progress:
        progress.selection(len(files))
    with progress.phase("Checking exported files") if progress else nullcontext():
        statuses, file_errors = _inspect_files(files, archive.paths.archive, progress)
    with (
        progress.phase("Checking database integrity and archive state")
        if progress
        else nullcontext()
    ):
        return VerificationReport(
            checks=(
                _guard_check(
                    EXPORT_DATABASE,
                    lambda: _check_export_database(
                        archive, active, files, export_db_error
                    ),
                ),
                _check_signatures(files, export_db_error, statuses),
                _check_missing_assets(files, export_db_error, file_errors),
                _guard_check(STATE, lambda: _check_state(state, state_error)),
                _check_ownership(archive, state),
                _check_pending_cleanup(archive, state),
            ),
        )


def _guard_check(name: str, run: Callable[[], Check]) -> Check:
    try:
        return run()
    except (ArchiveError, OSError) as error:
        return Check(name, False, str(error))


def _inspect_files(
    files: tuple[ExportedFile, ...],
    archive: Path,
    progress: ExportProgress | None = None,
) -> tuple[dict[Path, os.stat_result], dict[Path, str]]:
    """Inspect each path once without following symlinks beneath the archive."""
    statuses: dict[Path, os.stat_result] = {}
    errors: dict[Path, str] = {}
    for record in files:
        path = record.path
        current = archive
        try:
            if not path.is_relative_to(archive) or path == archive:
                raise OSError("not a file beneath the archive")
            for part in path.relative_to(archive).parts:
                current = current / part
                status = current.lstat()
                if stat.S_ISLNK(status.st_mode):
                    raise OSError(f"symlink at '{current}'")
                if current != path and not stat.S_ISDIR(status.st_mode):
                    raise OSError(f"not a directory: '{current}'")
            if not stat.S_ISREG(status.st_mode):
                raise OSError("not a regular file")
            statuses[path] = status
        except OSError as error:
            errors[path] = str(error)
        finally:
            if progress:
                progress.asset_done()
    return statuses, errors


def _check_export_database(
    archive: Archive,
    probes: PhotosProbes,
    files: tuple[ExportedFile, ...],
    error: str | None,
) -> Check:
    if error is not None:
        return Check(EXPORT_DATABASE, False, error)

    path = archive.paths.export_db
    integrity = probes.check_integrity(path)
    if integrity != INTEGRITY_OK:
        return Check(
            EXPORT_DATABASE, False, f"sqlite integrity_check reported: {integrity}"
        )

    version = probes.read_export_db_version(path)
    if not export_db_version_supported(version):
        return Check(
            EXPORT_DATABASE,
            False,
            unsupported_export_db_reason(version),
        )

    outside = [
        record
        for record in files
        if not record.path.is_relative_to(archive.paths.archive)
    ]
    if outside:
        return Check(
            EXPORT_DATABASE,
            False,
            f"{len(outside)} record(s) point outside the archive "
            f"({_sample(outside)}), so this database was written for another "
            "destination",
            paths=tuple(record.path for record in outside),
        )

    return Check(
        EXPORT_DATABASE,
        True,
        f"schema version {version}, integrity_check ok, "
        f"{len(files)} exported file record(s)",
    )


def _check_signatures(
    files: tuple[ExportedFile, ...],
    error: str | None,
    statuses: dict[Path, os.stat_result],
) -> Check:
    if error is not None:
        return Check(SIGNATURES, False, UNREADABLE_EXPORT_DB)

    comparable = [
        record
        for record in files
        if record.size is not None
        and record.mtime is not None
        and record.path in statuses
    ]
    mismatched = [
        record
        for record in comparable
        if not _signature_matches(record, statuses[record.path])
    ]
    unsigned = sum(record.size is None or record.mtime is None for record in files)
    unavailable = len(files) - unsigned - len(comparable)
    coverage = (
        f"{len(comparable) - len(mismatched)} matched, "
        f"{len(mismatched)} changed, {unsigned} missing signatures, "
        f"{unavailable} unavailable for comparison. "
        "Size and modification time only; file contents were not checksummed."
    )
    if unsigned or unavailable:
        coverage = "Coverage incomplete. " + coverage
    if mismatched:
        return Check(
            SIGNATURES,
            False,
            f"{len(mismatched)} of {len(comparable)} exported file(s) no longer "
            f"match their recorded size and modification time ({_sample(mismatched)}). "
            + coverage,
            paths=tuple(record.path for record in mismatched),
        )
    return Check(
        SIGNATURES,
        True,
        coverage,
    )


def _check_missing_assets(
    files: tuple[ExportedFile, ...],
    error: str | None,
    file_errors: dict[Path, str],
) -> Check:
    if error is not None:
        return Check(MISSING_ASSETS, False, UNREADABLE_EXPORT_DB)

    absent = [record for record in files if record.path in file_errors]
    if absent:
        details = [
            f"{record.path}: {file_errors[record.path]}"
            for record in absent[:_PATH_SAMPLE]
        ]
        if len(absent) > _PATH_SAMPLE:
            details.append(f"and {len(absent) - _PATH_SAMPLE} more")
        return Check(
            MISSING_ASSETS,
            False,
            f"{len(absent)} of {len(files)} exported file(s) are missing, invalid, "
            f"or unreadable ({'; '.join(details)})",
            paths=tuple(record.path for record in absent),
        )
    return Check(
        MISSING_ASSETS, True, f"all {len(files)} exported file(s) are still present"
    )


def _check_state(state: ArchiveState | None, error: str | None) -> Check:
    if state is None:
        return Check(STATE, False, error or UNREADABLE_STATE)
    if not state.initialized:
        return Check(
            STATE,
            False,
            f"the archive has never been initialized; run `{suggested_command('bootstrap')}`",
        )

    problems = []
    if state.last_successful_export_at is None:
        problems.append("no successful export is recorded")
    if state.last_full_export_at is None:
        problems.append("no full export is recorded")
    elif (
        state.last_successful_export_at is not None
        and state.last_full_export_at > state.last_successful_export_at
    ):
        problems.append("the last full export is newer than the last successful export")
    if state.last_report_path is not None and not state.last_report_path.exists():
        problems.append(f"the last report '{state.last_report_path}' is gone")

    if problems:
        return Check(STATE, False, "; ".join(problems))
    return Check(
        STATE,
        True,
        f"initialized {state.initialized_at.date()}, last successful export "
        f"{state.last_successful_export_at.date()}, last full export "
        f"{state.last_full_export_at.date()}",
    )


def _check_ownership(archive: Archive, state: ArchiveState | None) -> Check:
    if state is None:
        return Check(OWNERSHIP, False, UNREADABLE_STATE)
    if state.writer_hostname is None:
        return Check(
            OWNERSHIP,
            False,
            f"no Mac has claimed the archive; run `{suggested_command('bootstrap')}`",
        )
    if state.writer_hostname == archive.hostname:
        return Check(
            OWNERSHIP, True, f"this Mac ('{archive.hostname}') is the archive writer"
        )
    return Check(
        OWNERSHIP,
        True,
        f"'{state.writer_hostname}' is the archive writer; '{archive.hostname}' "
        "would have to take it over before its next export",
    )


def _check_pending_cleanup(archive: Archive, state: ArchiveState | None) -> Check:
    if state is None:
        return Check(PENDING_CLEANUP, False, UNREADABLE_STATE)

    run_id = state.pending_cleanup_run_id
    if run_id is None:
        return Check(PENDING_CLEANUP, True, "no cleanup is waiting for approval")
    return Check(
        PENDING_CLEANUP,
        False,
        f"cleanup run '{run_id}' is waiting for approval "
        f"({archive.paths.cleanup_manifest(run_id)}); review it and run "
        f"`{suggested_command('approve-cleanup', run_id)}`",
    )


def _signature_matches(record: ExportedFile, status: os.stat_result) -> bool:
    return status.st_size == record.size and int(status.st_mtime) == int(record.mtime)


def _sample(records: list[ExportedFile]) -> str:
    names = [str(record.path) for record in records[:_PATH_SAMPLE]]
    remaining = len(records) - len(names)
    if remaining > 0:
        names.append(f"and {remaining} more")
    return ", ".join(names)
