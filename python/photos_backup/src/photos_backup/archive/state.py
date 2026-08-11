from __future__ import annotations

import datetime
import json
import os
from dataclasses import dataclass, replace
from pathlib import Path
from typing import Any

from photos_backup.archive.errors import ArchiveUnavailable, ArchiveUnsafe
from photos_backup.archive.paths import ArchivePaths

STATE_VERSION = 1

_TIMESTAMP_FIELDS = (
    "initialized_at",
    "last_successful_export_at",
    "last_full_export_at",
    "last_mirror_completed_at",
)
_TEXT_FIELDS = ("writer_hostname", "pending_cleanup_run_id")
_PATH_FIELDS = ("last_report_path",)
_KNOWN_FIELDS = ("version", *_TIMESTAMP_FIELDS, *_TEXT_FIELDS, *_PATH_FIELDS)


@dataclass(frozen=True)
class ArchiveState:
    """Durable facts every command needs before it may touch the archive."""

    version: int = STATE_VERSION
    initialized_at: datetime.datetime | None = None
    last_successful_export_at: datetime.datetime | None = None
    last_full_export_at: datetime.datetime | None = None
    last_mirror_completed_at: datetime.datetime | None = None
    writer_hostname: str | None = None
    last_report_path: Path | None = None
    pending_cleanup_run_id: str | None = None

    @property
    def initialized(self) -> bool:
        return self.initialized_at is not None


class ArchiveStateStore:
    """Versioned archive state, replaced atomically and never written in dry runs."""

    def __init__(self, paths: ArchivePaths, *, dry_run: bool = False) -> None:
        self._paths = paths
        self._dry_run = dry_run

    @property
    def path(self) -> Path:
        return self._paths.state_file

    @property
    def dry_run(self) -> bool:
        return self._dry_run

    def load(self) -> ArchiveState:
        try:
            raw = self.path.read_text(encoding="utf-8")
        except FileNotFoundError:
            return ArchiveState()
        except OSError as error:
            raise ArchiveUnavailable(
                f"Could not read archive state '{self.path}': {error}"
            ) from error
        try:
            document = json.loads(raw)
        except json.JSONDecodeError as error:
            raise ArchiveUnsafe(
                f"Archive state '{self.path}' is not valid JSON: {error}"
            ) from error
        return _decode(document, self.path)

    def save(self, state: ArchiveState) -> ArchiveState:
        if self._dry_run:
            return state
        payload = json.dumps(_encode(state), indent=2, sort_keys=True) + "\n"
        _write_atomic(self.path, payload)
        return state

    def update(self, **changes: Any) -> ArchiveState:
        return self.save(replace(self.load(), **changes))


def _encode(state: ArchiveState) -> dict:
    document: dict = {"version": state.version}
    for name in _TIMESTAMP_FIELDS:
        value = getattr(state, name)
        document[name] = None if value is None else value.isoformat()
    for name in _TEXT_FIELDS:
        document[name] = getattr(state, name)
    for name in _PATH_FIELDS:
        value = getattr(state, name)
        document[name] = None if value is None else str(value)
    return document


def _decode(document: Any, path: Path) -> ArchiveState:
    if not isinstance(document, dict):
        raise ArchiveUnsafe(f"Archive state '{path}' must be a JSON object")
    version = document.get("version")
    if version != STATE_VERSION:
        raise ArchiveUnsafe(
            f"Archive state '{path}' has version {version!r}; "
            f"this build only understands version {STATE_VERSION}"
        )
    unknown = sorted(set(document) - set(_KNOWN_FIELDS))
    if unknown:
        raise ArchiveUnsafe(
            f"Archive state '{path}' has unknown key(s) {', '.join(unknown)}"
        )
    values: dict = {"version": version}
    for name in _TIMESTAMP_FIELDS:
        values[name] = _decode_timestamp(document.get(name), name, path)
    for name in _TEXT_FIELDS:
        values[name] = _decode_text(document.get(name), name, path)
    for name in _PATH_FIELDS:
        text = _decode_text(document.get(name), name, path)
        values[name] = None if text is None else Path(text)
    return ArchiveState(**values)


def _decode_timestamp(raw: Any, name: str, path: Path) -> datetime.datetime | None:
    if raw is None:
        return None
    if not isinstance(raw, str):
        raise ArchiveUnsafe(f"Archive state '{path}' key {name} must be a timestamp")
    try:
        value = datetime.datetime.fromisoformat(raw)
    except ValueError as error:
        raise ArchiveUnsafe(
            f"Archive state '{path}' key {name} is not an ISO timestamp: {raw!r}"
        ) from error
    if value.tzinfo is None:
        raise ArchiveUnsafe(
            f"Archive state '{path}' key {name} must carry a timezone: {raw!r}"
        )
    return value


def _decode_text(raw: Any, name: str, path: Path) -> str | None:
    if raw is None:
        return None
    if not isinstance(raw, str) or not raw.strip():
        raise ArchiveUnsafe(
            f"Archive state '{path}' key {name} must be a non-empty string"
        )
    return raw


def _write_atomic(path: Path, payload: str) -> None:
    temporary = path.with_name(f".{path.name}.{os.getpid()}.tmp")
    try:
        with temporary.open("w", encoding="utf-8") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        _fsync_directory(path.parent)
    except OSError as error:
        temporary.unlink(missing_ok=True)
        raise ArchiveUnavailable(f"Could not write '{path}': {error}") from error


def _fsync_directory(directory: Path) -> None:
    descriptor = os.open(directory, os.O_RDONLY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)
