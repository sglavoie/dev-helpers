"""The immutable cleanup manifest: its model, its file format, and its I/O."""

from __future__ import annotations

import datetime
import json
import os
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from photos_backup.apple_photos.identity import AssetIdentity
from photos_backup.apple_photos.reconcile import CandidateFile
from photos_backup.archive.errors import ArchiveUnavailable, ArchiveUnsafe
from photos_backup.archive.paths import safe_name
from photos_backup.errors import ActionRequired

MANIFEST_VERSION = 1

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


def _encode(manifest: CleanupManifest) -> dict[str, Any]:
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
