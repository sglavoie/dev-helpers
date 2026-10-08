"""Optional local verification receipts; never modify the archive."""

import datetime
import fcntl
import hashlib
import json
import os
import tempfile
from pathlib import Path
from typing import Any

import click

from photos_backup import transfers
from photos_backup.config import resolve_config_path


class VerificationHistory:
    def __init__(self, config_path: Path | None, archive: Path):
        self.archive = archive
        identity = [str(resolve_config_path(config_path).absolute()), str(archive)]
        key = hashlib.sha256(json.dumps(identity).encode()).hexdigest()
        self.path = transfers.history_root().parent / "verifications" / f"{key}.json"

    def read(self) -> tuple[dict[str, Any] | None, str | None]:
        try:
            document = json.loads(self.path.read_text(encoding="utf-8"))
            if (
                not isinstance(document, dict)
                or type(document.get("version")) is not int
                or document["version"] != 1
                or document.get("archive") != str(self.archive)
                or type(document.get("passed")) is not bool
                or not isinstance(document.get("failed_checks"), list)
                or any(not isinstance(item, str) for item in document["failed_checks"])
                or (
                    document.get("archive_error") is not None
                    and not isinstance(document["archive_error"], str)
                )
            ):
                raise ValueError("invalid verification receipt")
            for field in ("started_at", "completed_at"):
                value = document.get(field)
                if (
                    not isinstance(value, str)
                    or datetime.datetime.fromisoformat(value).tzinfo is None
                ):
                    raise ValueError(f"invalid {field}")
            return document, None
        except FileNotFoundError:
            return None, None
        except (OSError, ValueError) as error:
            return None, f"Could not read verification receipt '{self.path}': {error}"

    def record(self, report: dict[str, Any]) -> None:
        temporary = None
        try:
            archive = self.archive.resolve()
            if self.path.parent.resolve().is_relative_to(
                archive
            ) or self.path.resolve().is_relative_to(archive):
                raise ValueError("verification receipts must be outside the archive")
            self.path.parent.mkdir(parents=True, exist_ok=True)
            with self.path.with_suffix(".lock").open("a") as lock:
                fcntl.flock(lock, fcntl.LOCK_EX)
                previous, _ = self.read()
                if previous and datetime.datetime.fromisoformat(
                    previous["started_at"]
                ) > datetime.datetime.fromisoformat(report["started_at"]):
                    return
                document = {
                    "version": 1,
                    "archive": str(self.archive),
                    "passed": report["passed"],
                    "started_at": report["started_at"],
                    "completed_at": report["completed_at"],
                    "failed_checks": [
                        check["name"]
                        for check in report["checks"]
                        if not check["passed"]
                    ],
                    "archive_error": report["archive_error"],
                }
                with tempfile.NamedTemporaryFile(
                    mode="w", encoding="utf-8", dir=self.path.parent, delete=False
                ) as handle:
                    temporary = Path(handle.name)
                    handle.write(json.dumps(document, indent=2) + "\n")
                    handle.flush()
                    os.fsync(handle.fileno())
                os.replace(temporary, self.path)
            click.echo(f"Verification receipt: {self.path}", err=True)
        except (OSError, ValueError) as error:
            click.echo(
                f"Warning: could not save verification receipt: {error}", err=True
            )
        finally:
            if temporary is not None:
                try:
                    temporary.unlink(missing_ok=True)
                except OSError:
                    pass


def annotate_verification(
    receipt: dict[str, Any] | None,
    exported_at: datetime.datetime | None,
    attempt: dict[str, Any] | None,
    state_available: bool,
) -> dict[str, Any] | None:
    if receipt is None:
        return None
    started = datetime.datetime.fromisoformat(receipt["started_at"])
    incomplete_at = (
        datetime.datetime.fromisoformat(
            attempt["completed_at"] or attempt["started_at"]
        )
        if attempt and attempt["status"] != "succeeded"
        else None
    )
    return {
        **receipt,
        "archive_exported_since_verification": exported_at > started
        if exported_at
        else None,
        "archive_may_have_changed_since_verification": incomplete_at > started
        if incomplete_at
        else None,
        "archive_state_available": state_available,
    }
