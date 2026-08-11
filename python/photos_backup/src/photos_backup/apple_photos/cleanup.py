from __future__ import annotations

import datetime
import json
import os
from dataclasses import dataclass
from enum import Enum
from pathlib import Path
from typing import TYPE_CHECKING, Any

from photos_backup.apple_photos.adapter import PhotosProbes, resolve_export_files
from photos_backup.apple_photos.files import (
    delete_files,
    is_ignored,
    prune_empty_directories,
)
from photos_backup.apple_photos.identity import AssetIdentity, compare_library
from photos_backup.apple_photos.reconcile import (
    ArchiveFile,
    CandidateFile,
    Reconciliation,
    approval_reason,
    plan_reconciliation,
)
from photos_backup.archive.errors import ArchiveUnavailable, ArchiveUnsafe
from photos_backup.archive.paths import safe_name
from photos_backup.errors import ActionRequired

if TYPE_CHECKING:
    from photos_backup.apple_photos.plan import ExportResult
    from photos_backup.archive import Archive
    from photos_backup.archive.paths import ArchivePaths
    from photos_backup.archive.state import ArchiveState
    from photos_backup.config import ApplePhotosConfig

MANIFEST_VERSION = 1
MIRROR_DISABLED = "mirroring is disabled by [apple_photos] mirror"
INCREMENTAL_EXPORT = "an incremental export never reconciles the mirror"
ALREADY_MIRRORED = "the archive already mirrors the library"

_MANIFEST_KEYS = (
    "version",
    "run_id",
    "created_at",
    "hostname",
    "archive",
    "reason",
    "assets",
    "candidates",
    "changed",
    "unknown",
    "ambiguous",
)


class MirrorStatus(Enum):
    SKIPPED = "skipped"
    PREVIEW = "preview"
    CLEAN = "clean"
    APPLIED = "applied"
    PENDING = "pending"


@dataclass(frozen=True)
class MirrorOutcome:
    """What one reconciliation did to the archive, and what it left for a person."""

    status: MirrorStatus
    reason: str
    reconciliation: Reconciliation | None = None
    run_id: str | None = None
    manifest_path: Path | None = None
    deleted: tuple[Path, ...] = ()
    completed_at: datetime.datetime | None = None

    @property
    def pending(self) -> bool:
        return self.status is MirrorStatus.PENDING


@dataclass(frozen=True)
class CleanupManifest:
    """The exact deletions one person is asked to approve, and why they are asked."""

    run_id: str
    created_at: datetime.datetime
    hostname: str
    archive: Path
    reason: str
    assets: tuple[AssetIdentity, ...]
    candidates: tuple[CandidateFile, ...]
    changed: tuple[Path, ...]
    unknown: tuple[Path, ...]
    ambiguous: tuple[Path, ...]
    version: int = MANIFEST_VERSION


@dataclass(frozen=True)
class CleanupApproval:
    """What approving one pending run deleted, and what it deliberately did not."""

    run_id: str
    manifest: CleanupManifest
    deleted: tuple[Path, ...]
    stale: tuple[Path, ...]
    unreviewed: tuple[Path, ...]
    completed_at: datetime.datetime

    @property
    def complete(self) -> bool:
        return not self.unreviewed


@dataclass(frozen=True)
class CleanupDiscard:
    """One pending run a person rejected, and the manifest kept as the record."""

    run_id: str
    manifest: CleanupManifest
    manifest_path: Path
    discarded_at: datetime.datetime


def reconcile_mirror(
    config: ApplePhotosConfig,
    archive: Archive,
    result: ExportResult,
    *,
    probes: PhotosProbes | None = None,
) -> MirrorOutcome:
    """Delete what the library provably lost, or leave it pending for a person.

    Deletion is only ever automatic after a clean, complete full export whose
    candidates all map to one export-database asset each and stay inside the
    configured caps. Anything else becomes an immutable manifest to approve.
    """
    state = archive.state_store.load()
    pending = state.pending_cleanup_run_id
    if pending is not None:
        return MirrorOutcome(
            status=MirrorStatus.PENDING,
            reason=f"cleanup run '{pending}' is still waiting for approval",
            run_id=pending,
            manifest_path=archive.paths.cleanup_manifest(pending),
        )

    skipped = mirror_skip_reason(config, result)
    if skipped is not None:
        return MirrorOutcome(status=MirrorStatus.SKIPPED, reason=skipped)

    reconciliation, reason = _reconcile(config, archive, probes)
    if archive.dry_run:
        return MirrorOutcome(
            status=MirrorStatus.PREVIEW,
            reason=reason or "these deletions would need no approval",
            reconciliation=reconciliation,
        )
    if reason is not None:
        return _defer(archive, reconciliation, reason)

    deleted, now = _delete(archive, reconciliation.candidates)
    archive.state_store.update(last_mirror_completed_at=now)
    return MirrorOutcome(
        status=MirrorStatus.APPLIED if deleted else MirrorStatus.CLEAN,
        reason=(
            f"{len(deleted)} archive file(s) the library no longer has were deleted"
            if deleted
            else ALREADY_MIRRORED
        ),
        reconciliation=reconciliation,
        deleted=deleted,
        completed_at=now,
    )


def mirror_skip_reason(config: ApplePhotosConfig, result: ExportResult) -> str | None:
    """Why this export may not reconcile the mirror at all, or None when it may."""
    if not config.mirror:
        return MIRROR_DISABLED
    if not result.plan.is_full:
        return INCREMENTAL_EXPORT
    if not result.clean:
        return f"the export was not clean ({result.failure_reason()})"
    if result.missing_count:
        return (
            f"{result.missing_count} asset(s) are missing from iCloud, so the "
            "export does not describe the whole library"
        )
    return None


def approve_cleanup(
    config: ApplePhotosConfig,
    archive: Archive,
    run_id: str,
    *,
    probes: PhotosProbes | None = None,
) -> CleanupApproval:
    """Delete exactly the reviewed candidates that are still candidates today.

    The manifest is never trusted on its own: every candidate in it must still
    be a deletion candidate right now carrying the exact path, UUID, size and
    modification time a person reviewed, and every candidate that is not in it
    is left for a later run to propose.
    """
    state = archive.state_store.load()
    manifest = _require_pending(archive, state, run_id, require_owner=True)
    reconciliation, _ = _reconcile(config, archive, probes)

    # A CandidateFile compares by path, UUID, size and mtime, so a file whose
    # content and export-database signature were both refreshed after the
    # manifest was written is not the file that was reviewed.
    reviewed = set(manifest.candidates)
    deletable = tuple(item for item in reconciliation.candidates if item in reviewed)
    unreviewed = tuple(
        item.path for item in reconciliation.candidates if item not in reviewed
    )
    deletable_paths = {item.path for item in deletable}

    deleted, now = _delete(archive, deletable)
    changes: dict[str, Any] = {"pending_cleanup_run_id": None}
    if not unreviewed:
        changes["last_mirror_completed_at"] = now
    archive.state_store.update(**changes)

    return CleanupApproval(
        run_id=run_id,
        manifest=manifest,
        deleted=deleted,
        stale=tuple(
            sorted(
                item.path
                for item in manifest.candidates
                if item.path not in deletable_paths
            )
        ),
        unreviewed=unreviewed,
        completed_at=now,
    )


def discard_cleanup(archive: Archive, run_id: str) -> CleanupDiscard:
    """Reject a pending run so later exports may propose deletions again.

    Nothing is deleted and the manifest stays on disk as the record of what was
    rejected, so this needs no library and no proof that this Mac owns the archive.
    """
    state = archive.state_store.load()
    manifest = _require_pending(archive, state, run_id, require_owner=False)
    archive.state_store.update(pending_cleanup_run_id=None)
    return CleanupDiscard(
        run_id=run_id,
        manifest=manifest,
        manifest_path=archive.paths.cleanup_manifest(run_id),
        discarded_at=archive.now(),
    )


def archive_files(paths: ArchivePaths) -> tuple[ArchiveFile, ...]:
    """Every real file in the archive except the metadata photos-backup owns."""
    metadata = paths.metadata
    found: list[ArchiveFile] = []
    for path in sorted(paths.archive.rglob("*")):
        if metadata in path.parents or is_ignored(path) or not path.is_file():
            continue
        status = path.stat()
        found.append(ArchiveFile(path=path, size=status.st_size, mtime=status.st_mtime))
    return tuple(found)


def new_run_id(hostname: str, now: datetime.datetime) -> str:
    """A run identifier that survives `safe_name` unchanged and stays unique."""
    return safe_name(f"{now:%Y%m%dT%H%M%S}-{hostname}")


def write_manifest(path: Path, manifest: CleanupManifest) -> Path:
    """Write a manifest that may never be rewritten, so approval reviews one set."""
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(_encode(manifest), indent=2, sort_keys=True) + "\n"
    try:
        with path.open("x", encoding="utf-8") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
    except FileExistsError as error:
        raise ArchiveUnsafe(
            f"Cleanup manifest '{path}' already exists; a pending manifest is "
            "never rewritten"
        ) from error
    except OSError as error:
        raise ArchiveUnavailable(
            f"Could not write cleanup manifest '{path}': {error}"
        ) from error
    return path


def read_manifest(path: Path) -> CleanupManifest:
    """Read a manifest, refusing anything this build does not fully understand."""
    try:
        raw = path.read_text(encoding="utf-8")
    except FileNotFoundError as error:
        raise ActionRequired(
            f"Cleanup manifest '{path}' is gone, so its deletions cannot be "
            "reviewed or approved"
        ) from error
    except OSError as error:
        raise ArchiveUnavailable(
            f"Could not read cleanup manifest '{path}': {error}"
        ) from error
    try:
        return _decode(json.loads(raw))
    except (AttributeError, KeyError, TypeError, ValueError) as error:
        raise ArchiveUnsafe(
            f"Cleanup manifest '{path}' is malformed: {error}"
        ) from error


def _reconcile(
    config: ApplePhotosConfig, archive: Archive, probes: PhotosProbes | None
) -> tuple[Reconciliation, str | None]:
    active = probes or PhotosProbes()
    export_db = archive.paths.export_db
    root = archive.paths.archive
    comparison = compare_library(
        active.read_export_db(export_db), active.read_library(config.library)
    )
    # A record that resolves outside the archive claims nothing here, so the
    # archive file it should have explained stays unexplained and needs approval.
    files = tuple(
        record
        for record in resolve_export_files(active.read_export_files(export_db), root)
        if record.path.is_relative_to(root)
    )
    reconciliation = plan_reconciliation(
        comparison, files, archive_files(archive.paths)
    )
    return reconciliation, approval_reason(
        reconciliation,
        comparison,
        max_absent_assets=config.cleanup_max_assets,
        max_absent_fraction=config.cleanup_max_fraction,
    )


def _defer(
    archive: Archive, reconciliation: Reconciliation, reason: str
) -> MirrorOutcome:
    """Persist the deletions a person has to approve, and stop touching them."""
    now = archive.now()
    run_id = _unused_run_id(archive, new_run_id(archive.hostname, now))
    manifest_path = write_manifest(
        archive.paths.cleanup_manifest(run_id),
        CleanupManifest(
            run_id=run_id,
            created_at=now,
            hostname=archive.hostname,
            archive=archive.paths.archive,
            reason=reason,
            assets=reconciliation.absent,
            candidates=reconciliation.candidates,
            changed=reconciliation.changed,
            unknown=reconciliation.unknown,
            ambiguous=reconciliation.ambiguous,
        ),
    )
    archive.state_store.update(pending_cleanup_run_id=run_id)
    return MirrorOutcome(
        status=MirrorStatus.PENDING,
        reason=reason,
        reconciliation=reconciliation,
        run_id=run_id,
        manifest_path=manifest_path,
    )


def _unused_run_id(archive: Archive, base: str) -> str:
    """A run id no manifest already holds, because a manifest is never rewritten.

    Two runs in the same second on the same Mac share a base id, which a
    discarded manifest makes reachable rather than merely theoretical.
    """
    run_id = base
    attempt = 1
    while archive.paths.cleanup_manifest(run_id).exists():
        attempt += 1
        run_id = f"{base}-{attempt}"
    return run_id


def _require_pending(
    archive: Archive, state: ArchiveState, run_id: str, *, require_owner: bool
) -> CleanupManifest:
    """Refuse to resolve anything but the one pending run, on the owning Mac.

    Ownership is only required when the caller is about to delete: rejecting a
    manifest touches no file and is safe from any Mac that can read the archive.
    """
    pending = state.pending_cleanup_run_id
    if pending is None:
        raise ActionRequired(
            f"No cleanup is waiting for approval in archive "
            f"'{archive.paths.archive}', so run '{run_id}' cannot be approved"
        )
    if pending != run_id:
        raise ActionRequired(
            f"Cleanup run '{run_id}' is not the pending run; '{pending}' is. "
            f"Review '{archive.paths.cleanup_manifest(pending)}' and approve that one"
        )
    if require_owner and state.writer_hostname != archive.hostname:
        raise ActionRequired(
            f"'{state.writer_hostname or 'an unknown host'}' owns this archive, not "
            f"'{archive.hostname}'; run 'photos-backup daily' here first so this Mac "
            "proves it holds the same library before it deletes anything"
        )

    manifest = read_manifest(archive.paths.cleanup_manifest(run_id))
    if manifest.run_id != run_id or manifest.archive != archive.paths.archive:
        raise ArchiveUnsafe(
            f"Cleanup manifest for run '{run_id}' describes run "
            f"'{manifest.run_id}' of archive '{manifest.archive}'"
        )
    return manifest


def _delete(
    archive: Archive, candidates: tuple[CandidateFile, ...]
) -> tuple[tuple[Path, ...], datetime.datetime]:
    deleted = delete_files(candidate.path for candidate in candidates)
    prune_empty_directories(archive.paths.archive, keep=(archive.paths.metadata,))
    return deleted, archive.now()


def _encode(manifest: CleanupManifest) -> dict:
    return {
        "version": manifest.version,
        "run_id": manifest.run_id,
        "created_at": manifest.created_at.isoformat(),
        "hostname": manifest.hostname,
        "archive": str(manifest.archive),
        "reason": manifest.reason,
        "assets": [
            {
                "uuid": asset.uuid,
                "cloud_guid": asset.cloud_guid,
                "original_filename": asset.original_filename,
            }
            for asset in manifest.assets
        ],
        "candidates": [
            {
                "path": str(candidate.path),
                "uuid": candidate.uuid,
                "size": candidate.size,
                "mtime": candidate.mtime,
            }
            for candidate in manifest.candidates
        ],
        "changed": [str(path) for path in manifest.changed],
        "unknown": [str(path) for path in manifest.unknown],
        "ambiguous": [str(path) for path in manifest.ambiguous],
    }


def _decode(document: Any) -> CleanupManifest:
    version = document["version"]
    if version != MANIFEST_VERSION:
        raise ValueError(
            f"version {version!r} is not the version {MANIFEST_VERSION} this "
            "build writes"
        )
    unknown = sorted(set(document) - set(_MANIFEST_KEYS))
    if unknown:
        raise ValueError(f"unknown key(s) {', '.join(unknown)}")
    return CleanupManifest(
        version=version,
        run_id=document["run_id"],
        created_at=datetime.datetime.fromisoformat(document["created_at"]),
        hostname=document["hostname"],
        archive=Path(document["archive"]),
        reason=document["reason"],
        assets=tuple(AssetIdentity(**asset) for asset in document["assets"]),
        candidates=tuple(
            CandidateFile(
                path=Path(candidate["path"]),
                uuid=candidate["uuid"],
                size=candidate["size"],
                mtime=candidate["mtime"],
            )
            for candidate in document["candidates"]
        ),
        changed=tuple(Path(path) for path in document["changed"]),
        unknown=tuple(Path(path) for path in document["unknown"]),
        ambiguous=tuple(Path(path) for path in document["ambiguous"]),
    )
