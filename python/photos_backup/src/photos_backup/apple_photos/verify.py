from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING

from photos_backup.apple_photos.adapter import (
    INTEGRITY_OK,
    ExportedFile,
    PhotosProbes,
    export_db_version_supported,
    resolve_export_files,
)
from photos_backup.archive.errors import ArchiveError

if TYPE_CHECKING:
    from photos_backup.archive import Archive
    from photos_backup.archive.state import ArchiveState

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
    archive: Archive, *, probes: PhotosProbes | None = None
) -> VerificationReport:
    """Report archive health without writing anything.

    Every read that can fail is caught and reported as a failed check, so one
    corrupt file never hides the state of everything else.
    """
    active = probes or PhotosProbes()

    export_db = archive.paths.export_db
    files: tuple[ExportedFile, ...] = ()
    export_db_error: str | None = None
    if not export_db.exists():
        export_db_error = f"'{export_db}' does not exist"
    else:
        try:
            files = resolve_export_files(
                active.read_export_files(export_db), archive.paths.archive
            )
        except ArchiveError as error:
            export_db_error = str(error)

    state: ArchiveState | None = None
    state_error: str | None = None
    try:
        state = archive.state_store.load()
    except ArchiveError as error:
        state_error = str(error)

    return VerificationReport(
        checks=(
            _check_export_database(archive, active, files, export_db_error),
            _check_signatures(files, export_db_error),
            _check_missing_assets(files, export_db_error),
            _check_state(state, state_error),
            _check_ownership(archive, state),
            _check_pending_cleanup(archive, state),
        )
    )


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
            f"schema version {version or 'unknown'} is not one this osxphotos "
            "can read; upgrade photos-backup",
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
        )

    return Check(
        EXPORT_DATABASE,
        True,
        f"schema version {version}, integrity_check ok, "
        f"{len(files)} exported file record(s)",
    )


def _check_signatures(files: tuple[ExportedFile, ...], error: str | None) -> Check:
    if error is not None:
        return Check(SIGNATURES, False, UNREADABLE_EXPORT_DB)

    comparable = [
        record
        for record in files
        if record.size is not None
        and record.mtime is not None
        and record.path.is_file()
    ]
    mismatched = [record for record in comparable if not _signature_matches(record)]
    if mismatched:
        return Check(
            SIGNATURES,
            False,
            f"{len(mismatched)} of {len(comparable)} exported file(s) no longer "
            f"match their recorded size and modification time ({_sample(mismatched)})",
        )
    return Check(
        SIGNATURES,
        True,
        f"{len(comparable)} of {len(files)} exported file(s) match their recorded "
        "size and modification time",
    )


def _check_missing_assets(files: tuple[ExportedFile, ...], error: str | None) -> Check:
    if error is not None:
        return Check(MISSING_ASSETS, False, UNREADABLE_EXPORT_DB)

    absent = [record for record in files if not record.path.exists()]
    if absent:
        return Check(
            MISSING_ASSETS,
            False,
            f"{len(absent)} of {len(files)} exported file(s) are gone from the "
            f"archive ({_sample(absent)})",
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
            "the archive has never been initialized; run 'photos-backup bootstrap'",
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
            "no Mac has claimed the archive; run 'photos-backup bootstrap'",
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
        f"'photos-backup approve-cleanup {run_id}'",
    )


def _signature_matches(record: ExportedFile) -> bool:
    status = record.path.stat()
    return status.st_size == record.size and int(status.st_mtime) == int(record.mtime)


def _sample(records: list[ExportedFile]) -> str:
    names = [str(record.path) for record in records[:_PATH_SAMPLE]]
    remaining = len(records) - len(names)
    if remaining > 0:
        names.append(f"and {remaining} more")
    return ", ".join(names)
