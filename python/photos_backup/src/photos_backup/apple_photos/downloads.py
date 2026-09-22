"""Bound PhotoKit staging without interrupting archive writes.

osxphotos can block inside a synchronous native PhotoKit call before reaching
its Python timeout. Only the missing-file staging method is isolated here;
the child never opens the export database or writes to the archive.
"""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
import tempfile
import time
from contextlib import contextmanager
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import click
from osxphotos.exportoptions import ExportOptions
from osxphotos.photoexporter import PhotoExporter, StagedFiles

DEFAULT_DOWNLOAD_TIMEOUT = 120

# The upstream staging method needs these scalar properties, not a PhotosDB.
_PHOTO_FIELDS = (
    "uuid",
    "original_filename",
    "hasadjustments",
    "live_photo",
    "shared",
    "has_raw",
    "uti_edited",
    "uti",
    "burst",
    "path_derivatives",
)
_OPTION_FIELDS = (
    "edited",
    "live_photo",
    "overwrite",
    "update",
    "force_update",
    "raw_photo",
    "preview",
)


class DownloadBudget:
    """One budget per asset, shared by its versions and retries, for this run."""

    def __init__(self, seconds: float) -> None:
        if seconds <= 0:
            raise ValueError("download timeout must be positive")
        self.seconds = seconds
        self.spent: dict[str, float] = {}
        self.failures: dict[str, dict] = {}

    def stage(self, exporter: PhotoExporter, options: ExportOptions) -> StagedFiles:
        photo = exporter.photo
        remaining = self.seconds - self.spent.get(photo.uuid, 0)
        if remaining <= 0:
            if photo.uuid not in self.failures:
                self._failure(
                    photo,
                    f"Missing-file download budget exhausted after {self.seconds:g}s",
                )
            return StagedFiles()

        root = Path(tempfile.mkdtemp(prefix="download_", dir=exporter._temp_dir_path))
        request = root / "request.json"
        response = root / "response.json"
        data = {name: getattr(photo, name) for name in _PHOTO_FIELDS}
        data["_info"] = {"burstUUID": photo._info.get("burstUUID")}
        request.write_text(
            json.dumps(
                {
                    "photo": data,
                    "options": {
                        name: getattr(options, name) for name in _OPTION_FIELDS
                    },
                }
            ),
            encoding="utf-8",
        )
        click.echo(
            f"Retrieving missing files: {photo.original_filename} ({photo.uuid}); "
            f"{remaining:.0f}s download budget remaining",
            err=True,
        )
        started = time.monotonic()
        keep = False
        try:
            _run_worker(request, response, remaining)
            staged = StagedFiles(**json.loads(response.read_text(encoding="utf-8")))
            keep = True
            if staged.error:
                self._failure(photo, "; ".join(str(error) for error in staged.error))
            return staged
        except subprocess.TimeoutExpired:
            # Exhaust the UUID's budget, so upstream retries cannot restart it.
            self.spent[photo.uuid] = self.seconds
            self._failure(
                photo, f"Missing-file download timed out after {self.seconds:g}s"
            )
            return StagedFiles()
        except (OSError, ValueError, TypeError, subprocess.CalledProcessError) as error:
            self._failure(photo, f"Missing-file download failed: {error}")
            return StagedFiles()
        finally:
            self.spent[photo.uuid] = self.spent.get(photo.uuid, 0) + (
                time.monotonic() - started
            )
            if not keep:
                shutil.rmtree(root)

    def _failure(self, photo, reason: str) -> None:
        self.failures[photo.uuid] = {
            "uuid": photo.uuid,
            "filename": photo.original_filename,
            "reason": reason,
        }
        click.echo(f"Still unbacked-up: {photo.original_filename}: {reason}", err=True)

    def write_failures(self, report: Path) -> None:
        if not self.failures:
            return
        path = report.with_suffix(".downloads.json")
        path.write_text(
            json.dumps(list(self.failures.values()), indent=2) + "\n", encoding="utf-8"
        )
        click.echo(f"Incomplete downloads (retry on the next run): {path}", err=True)


def _run_worker(request: Path, response: Path, timeout: float) -> None:
    # subprocess.run kills and reaps the child on timeout or interruption.
    # A fresh interpreter avoids forking Apple's multithreaded frameworks.
    subprocess.run(
        [
            sys.executable,
            "-m",
            "photos_backup.apple_photos.downloads",
            str(request),
            str(response),
        ],
        check=True,
        capture_output=True,
        text=True,
        timeout=timeout,
    )


@contextmanager
def bounded_downloads(seconds: float = DEFAULT_DOWNLOAD_TIMEOUT):
    """Temporarily adapt osxphotos's private staging seam for one serial export."""
    budget = DownloadBudget(seconds)
    original = PhotoExporter._stage_photo_for_export_with_photokit

    def stage(exporter, options):
        if options.dry_run:
            return original(exporter, options)
        return budget.stage(exporter, options)

    with patch.object(PhotoExporter, "_stage_photo_for_export_with_photokit", stage):
        yield budget


def _worker(request: Path, response: Path) -> None:
    document = json.loads(request.read_text(encoding="utf-8"))
    photo = SimpleNamespace(**document["photo"], _verbose=lambda _: None)
    exporter = PhotoExporter(photo)
    exporter._temp_dir_path = request.parent
    staged = exporter._stage_photo_for_export_with_photokit(
        ExportOptions(**document["options"])
    )
    response.write_text(json.dumps(staged.asdict()), encoding="utf-8")


if __name__ == "__main__":
    _worker(Path(sys.argv[1]), Path(sys.argv[2]))
