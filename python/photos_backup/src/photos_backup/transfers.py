"""Local transfer receipts, independent of the archive and its writer state."""

from __future__ import annotations

import datetime
import fcntl
import hashlib
import json
import os
import tempfile
from collections.abc import Callable
from pathlib import Path
from typing import Any

import click

from photos_backup.config import resolve_config_path
from photos_backup.summary import BackupSummary

# One JSON receipt or attempt document, as read from or written to disk.
Receipt = dict[str, Any]


def history_root() -> Path:
    configured = os.environ.get("XDG_STATE_HOME", "")
    base = Path(configured) if configured else Path.home() / ".local/state"
    if not base.is_absolute():
        base = Path.home() / ".local/state"
    return base / "photos-backup/transfers"


def _key(value: object) -> str:
    return hashlib.sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()


def _now() -> str:
    return datetime.datetime.now(datetime.UTC).isoformat()


def _read(path: Path) -> Receipt:
    document = json.loads(path.read_text(encoding="utf-8"))
    if (
        not isinstance(document, dict)
        or type(document.get("version")) is not int
        or document["version"] != 1
    ):
        raise ValueError("unsupported transfer receipt")
    for field in ("step", "source", "destination"):
        if not isinstance(document.get(field), str):
            raise ValueError(f"invalid {field} in transfer receipt")
    for field in ("last_attempt", "last_success"):
        if field not in document:
            raise ValueError(f"missing {field} in transfer receipt")
        attempt = document.get(field)
        if attempt is None and field == "last_success":
            continue
        attempt = _validate_attempt(attempt, field)
        if field == "last_success" and attempt["status"] != "succeeded":
            raise ValueError("invalid last successful transfer")
    return document


def _validate_attempt(attempt: object, field: str) -> Receipt:
    if not isinstance(attempt, dict):
        raise ValueError(f"invalid {field} in transfer receipt")
    if attempt.get("status") not in ("started", "succeeded", "failed", "interrupted"):
        raise ValueError(f"invalid status in {field}")
    if attempt.get("mode") not in (None, "copy", "mirror"):
        raise ValueError(f"invalid mode in {field}")
    if attempt["status"] != "started" and attempt.get("completed_at") is None:
        raise ValueError(f"missing completion time in {field}")
    for timestamp in ("started_at", "completed_at"):
        value = attempt.get(timestamp)
        if value is None and timestamp == "completed_at":
            continue
        if not isinstance(value, str):
            raise ValueError(f"invalid {timestamp} in {field}")
        if datetime.datetime.fromisoformat(value).tzinfo is None:
            raise ValueError(f"timestamp without timezone in {field}")
    return attempt


def classify_transfers(
    configured: list[Receipt], receipts: list[Receipt]
) -> tuple[list[Receipt], list[Receipt]]:
    """Match exact receipt identities without probing potentially unmounted paths."""

    def identity(row: Receipt) -> tuple[str, str, str]:
        return row["step"], row["source"], row["destination"]

    by_identity = {identity(row): row for row in receipts}
    current = [
        by_identity.get(
            identity(row), {**row, "last_attempt": None, "last_success": None}
        )
        for row in configured
    ]
    active = {identity(row) for row in configured}
    historical = [row for row in receipts if identity(row) not in active]
    return current, historical


def annotate_archive_freshness(
    receipts: list[Receipt],
    archive: Path | None,
    exported_at: datetime.datetime | None,
    attempt: Receipt | None = None,
) -> list[Receipt]:
    """Compare recorded times for exact sources without probing mounted paths.

    A copy's start is the conservative boundary: its completion time alone
    cannot establish that it included an export which happened during the copy.
    """
    return [
        {
            **row,
            "archive_exported_since_copy": (
                exported_at
                > datetime.datetime.fromisoformat(row["last_success"]["started_at"])
                if row["source"] == str(archive)
                and exported_at is not None
                and row["last_success"] is not None
                else None
            ),
            "archive_may_have_changed_since_copy": (
                datetime.datetime.fromisoformat(
                    attempt["completed_at"] or attempt["started_at"]
                )
                > datetime.datetime.fromisoformat(row["last_success"]["started_at"])
                if row["source"] == str(archive)
                and attempt is not None
                and attempt["status"] != "succeeded"
                and row["last_success"] is not None
                else None
            ),
        }
        for row in receipts
    ]


def annotate_upstream_freshness(receipts: list[Receipt]) -> list[Receipt]:
    """Compare current SD/SSD/cloud routes, using lexical paths only."""
    result = []
    for row in receipts:
        upstream = [
            source["last_success"]["completed_at"]
            for source in receipts
            if row["step"] == "Remote"
            and source["step"].startswith("SSD:")
            and source["last_success"] is not None
            and Path(source["destination"]).is_relative_to(Path(row["source"]))
        ]
        imported = [
            source["last_success"]["completed_at"]
            for source in receipts
            if row["step"] == "SSD: SD Card"
            and source["step"] == "SD Card"
            and source["last_success"] is not None
            and Path(source["destination"]).is_relative_to(Path(row["source"]))
        ]
        result.append(
            {
                **row,
                "ssd_copied_since_upload": (
                    max(datetime.datetime.fromisoformat(value) for value in upstream)
                    > datetime.datetime.fromisoformat(row["last_success"]["started_at"])
                    if upstream and row["last_success"] is not None
                    else None
                ),
                "sd_imported_since_copy": (
                    max(datetime.datetime.fromisoformat(value) for value in imported)
                    > datetime.datetime.fromisoformat(row["last_success"]["started_at"])
                    if imported and row["last_success"] is not None
                    else None
                ),
            }
        )
    return result


def annotate_copy_grace(
    receipts: list[Receipt], max_age_days: int, now: datetime.datetime
) -> list[Receipt]:
    """Mark copies whose last success is recent enough for `status --check` to wait.

    A failed, unfinished, or never-recorded copy is never within the grace period.
    """
    limit = datetime.timedelta(days=max_age_days)
    result = []
    for row in receipts:
        latest, success = row["last_attempt"], row["last_success"]
        result.append(
            {
                **row,
                "within_grace": (
                    success is not None
                    and (latest is None or latest["status"] == "succeeded")
                    and now - datetime.datetime.fromisoformat(success["completed_at"])
                    < limit
                ),
            }
        )
    return result


class TransferHistory:
    def __init__(self, config_path: Path | None, *, root: Path | None = None):
        self.directory = (root if root is not None else history_root()) / _key(
            str(resolve_config_path(config_path).absolute())
        )

    def read(self) -> tuple[list[Receipt], list[str]]:
        """Reading status never creates a directory or a lock file."""
        receipts: list[Receipt] = []
        errors: list[str] = []
        try:
            paths = sorted(self.directory.iterdir())
        except FileNotFoundError:
            return receipts, errors
        except OSError as error:
            return receipts, [f"Could not read transfer history: {error}"]
        for path in paths:
            if path.suffix != ".json":
                continue
            try:
                receipts.append(_read(path))
            except (OSError, ValueError) as error:
                errors.append(f"Could not read transfer receipt '{path}': {error}")
        return sorted(
            receipts, key=lambda row: (row["step"], row["destination"])
        ), errors

    def run(
        self,
        step: str,
        source: Path,
        destination: Path | str,
        operation: Callable[[], BackupSummary],
        *,
        dry_run: bool,
        delete_at_destination: bool | None = None,
    ) -> BackupSummary:
        if dry_run:
            return operation()
        identity = {
            "step": step,
            "source": str(source),
            "destination": str(destination),
        }
        attempt = {"started_at": _now(), "completed_at": None, "status": "started"}
        if delete_at_destination is not None:
            attempt["mode"] = "mirror" if delete_at_destination else "copy"
        self._record(identity, attempt)
        try:
            result = operation()
        except BaseException as error:
            self._record(
                identity,
                {
                    **attempt,
                    "completed_at": _now(),
                    "status": "failed"
                    if isinstance(error, Exception)
                    else "interrupted",
                    "error": str(error) or type(error).__name__,
                },
            )
            raise
        self._record(
            identity,
            {
                **attempt,
                "completed_at": _now(),
                "status": "failed" if result.error else "succeeded",
                "error": result.error,
                "files_transferred": result.files_transferred,
                "total_size": result.total_size,
                "elapsed_seconds": result.elapsed_seconds,
            },
        )
        return result

    def record_preflight_failure(
        self,
        step: str,
        source: Path,
        destination: Path,
        error: BaseException,
        *,
        started_at: str,
        dry_run: bool,
        delete_at_destination: bool | None = None,
    ) -> None:
        """Record a refused copy without losing its last successful transfer."""
        if dry_run:
            return
        self._record(
            {"step": step, "source": str(source), "destination": str(destination)},
            {
                "started_at": started_at,
                "completed_at": _now(),
                "status": "failed" if isinstance(error, Exception) else "interrupted",
                "error": f"Preflight: {str(error) or type(error).__name__}",
                "mode": None
                if delete_at_destination is None
                else "mirror"
                if delete_at_destination
                else "copy",
            },
        )

    def _record(self, identity: Receipt, attempt: Receipt) -> None:
        """Merge under a short lock; a receipt failure never masks a transfer."""
        path = self.directory / f"{_key(identity)}.json"
        temporary = None
        try:
            self.directory.mkdir(parents=True, exist_ok=True)
            with path.with_suffix(".lock").open("a") as lock:
                fcntl.flock(lock, fcntl.LOCK_EX)
                try:
                    document = _read(path)
                except FileNotFoundError:
                    document = {"version": 1, **identity, "last_success": None}
                except ValueError as error:
                    # Reserve a unique name under the same lock, then preserve the
                    # original bytes before starting a receipt with no prior success.
                    with tempfile.NamedTemporaryFile(
                        dir=self.directory, prefix=path.name + ".corrupt-", delete=False
                    ) as handle:
                        quarantined = Path(handle.name)
                    try:
                        os.replace(path, quarantined)
                    except OSError:
                        quarantined.unlink(missing_ok=True)
                        raise
                    click.echo(
                        f"Warning: preserved invalid transfer receipt at '{quarantined}': "
                        f"{error}; starting fresh history for this transfer.",
                        err=True,
                    )
                    document = {"version": 1, **identity, "last_success": None}
                previous = document.get("last_attempt")
                if previous is None or attempt["started_at"] >= previous["started_at"]:
                    document["last_attempt"] = attempt
                success = document["last_success"]
                if attempt["status"] == "succeeded" and (
                    success is None
                    or attempt["completed_at"] >= success["completed_at"]
                ):
                    document["last_success"] = attempt
                with tempfile.NamedTemporaryFile(
                    mode="w", encoding="utf-8", dir=self.directory, delete=False
                ) as handle:
                    temporary = Path(handle.name)
                    handle.write(json.dumps(document, indent=2) + "\n")
                    handle.flush()
                    os.fsync(handle.fileno())
                os.replace(temporary, path)
        except (OSError, ValueError) as error:
            click.echo(
                f"Warning: could not save transfer receipt '{path}': {error}", err=True
            )
        finally:
            if temporary is not None:
                try:
                    temporary.unlink(missing_ok=True)
                except OSError:
                    pass
