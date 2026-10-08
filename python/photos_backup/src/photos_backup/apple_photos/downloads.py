"""Bound PhotoKit staging without interrupting archive writes.

osxphotos can block inside a synchronous native PhotoKit call before reaching
its Python timeout. Only the missing-file staging method is isolated here;
the child never opens the export database or writes to the archive.
"""

from __future__ import annotations

import json
import select
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import traceback
from collections.abc import Iterator
from contextlib import contextmanager, nullcontext
from pathlib import Path
from types import SimpleNamespace
from typing import TYPE_CHECKING, Any, BinaryIO, TypedDict
from unittest.mock import patch

import click
from osxphotos.exportoptions import ExportOptions
from osxphotos.photoexporter import PhotoExporter, StagedFiles
from osxphotos.photokit import PhotoLibrary, Photos

from photos_backup.progress import ExportProgress

if TYPE_CHECKING:
    from osxphotos import PhotoInfo

DEFAULT_DOWNLOAD_TIMEOUT = 120
MAX_CONTROL_RESPONSE_BYTES = 65536

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


class DownloadFailure(TypedDict):
    uuid: str
    filename: str
    reason: str


class WorkerRequest(TypedDict):
    id: int
    request: str
    response: str


StageKey = tuple[bool, ...]


class DownloadBudget:
    """One budget per asset, shared by its versions and retries, for this run."""

    def __init__(self, seconds: float, progress: ExportProgress | None = None) -> None:
        if seconds <= 0:
            raise ValueError("download timeout must be positive")
        self.seconds = seconds
        self.spent: dict[str, float] = {}
        self.failures: dict[str, DownloadFailure] = {}
        self._issues: dict[str, dict[StageKey, str]] = {}
        self.attempts: dict[str, int] = {}
        self.worker = DownloadWorker()
        self.progress = progress

    def stage(self, exporter: PhotoExporter, options: ExportOptions) -> StagedFiles:
        photo = exporter.photo
        key: StageKey = tuple(getattr(options, name) for name in _OPTION_FIELDS)
        remaining = self.seconds - self.spent.get(photo.uuid, 0)
        if remaining <= 0:
            if photo.uuid not in self.failures:
                self._failure(
                    photo,
                    key,
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
        self.attempts[photo.uuid] = self.attempts.get(photo.uuid, 0) + 1
        detail = (
            f"{photo.original_filename} ({photo.uuid}); "
            f"attempt {self.attempts[photo.uuid]}"
        )
        if self.progress is None:
            click.echo(
                f"Retrieving missing files: {detail}; "
                f"{remaining:.0f}s download budget remaining",
                err=True,
            )
        started = time.monotonic()
        keep = False
        try:
            with (
                self.progress.phase(
                    "Retrieving missing files", detail, budget=remaining
                )
                if self.progress
                else nullcontext()
            ):
                self.worker.run(request, response, remaining)
            staged = StagedFiles(**json.loads(response.read_text(encoding="utf-8")))
            keep = True
            _validate_staging(staged, photo, options)
            if staged.error:
                self._failure(
                    photo, key, "; ".join(str(error) for error in staged.error)
                )
            else:
                issues = self._issues.get(photo.uuid, {})
                issues.pop(key, None)
                if not issues:
                    self.failures.pop(photo.uuid, None)
                else:
                    self.failures[photo.uuid]["reason"] = "; ".join(issues.values())
                if self.progress:
                    self.progress.unresolved = len(self.failures)
            return staged
        except subprocess.TimeoutExpired:
            self.worker.close()
            # Exhaust the UUID's budget, so upstream retries cannot restart it.
            self.spent[photo.uuid] = self.seconds
            self._failure(
                photo, key, f"Missing-file download timed out after {self.seconds:g}s"
            )
            return StagedFiles()
        except (OSError, ValueError, TypeError, subprocess.CalledProcessError) as error:
            self.worker.close()
            self._failure(photo, key, f"Missing-file download failed: {error}")
            return StagedFiles()
        finally:
            self.spent[photo.uuid] = min(
                self.seconds,
                self.spent.get(photo.uuid, 0) + time.monotonic() - started,
            )
            if not keep:
                shutil.rmtree(root, ignore_errors=True)

    def _failure(self, photo: PhotoInfo, key: StageKey, reason: str) -> None:
        self._issues.setdefault(photo.uuid, {})[key] = reason
        self.failures[photo.uuid] = {
            "uuid": photo.uuid,
            "filename": photo.original_filename,
            "reason": "; ".join(self._issues[photo.uuid].values()),
        }
        message = f"Unresolved download: {photo.original_filename}: {reason}"
        if self.progress:
            self.progress.unresolved = len(self.failures)
            self.progress.message(message)
        else:
            click.echo(message, err=True)

    def write_failures(self, report: Path) -> None:
        if not self.failures:
            return
        path = report.with_suffix(".downloads.json")
        try:
            path.write_text(
                json.dumps(list(self.failures.values()), indent=2) + "\n",
                encoding="utf-8",
            )
        except OSError as error:
            # Called while an export error may be propagating; never mask it.
            message = (
                f"Warning: could not record {len(self.failures)} incomplete "
                f"download(s) in '{path}': {error}"
            )
        else:
            message = f"Incomplete downloads (retry on the next run): {path}"
        if self.progress:
            self.progress.message(message)
        else:
            click.echo(message, err=True)


def _validate_staging(
    staged: StagedFiles, photo: PhotoInfo, options: ExportOptions
) -> None:
    """A nominal success with absent requested components is still unresolved."""
    required = ["edited" if options.edited else "original"]
    if photo.live_photo and options.live_photo:
        required.append("edited_live" if options.edited else "original_live")
    if photo.has_raw and options.raw_photo:
        required.append("raw")
    missing = [name for name in required if not getattr(staged, name)]
    if missing:
        staged.error.append(
            (
                photo.original_filename,
                f"PhotoKit returned no {', '.join(missing)}",
            )
        )


class DownloadWorker:
    """One serial child, replaced after a timeout or protocol failure.

    A private socket carries control messages; native stdout/stderr cannot corrupt
    the protocol. The child only stages files, never opening the archive database.
    """

    def __init__(self) -> None:
        self.process: subprocess.Popen[bytes] | None = None
        self.channel: socket.socket | None = None
        self.diagnostics: BinaryIO | None = None
        self.sequence = 0

    def _start(self) -> None:
        parent, child = socket.socketpair()
        self.channel = parent
        self.diagnostics = tempfile.TemporaryFile()
        try:
            self.process = subprocess.Popen(
                [
                    sys.executable,
                    "-m",
                    "photos_backup.apple_photos.downloads",
                    "--serve",
                    str(child.fileno()),
                ],
                pass_fds=(child.fileno(),),
                stdin=subprocess.DEVNULL,
                stdout=self.diagnostics,
                stderr=self.diagnostics,
            )
        finally:
            child.close()

    def run(self, request: Path, response: Path, timeout: float) -> None:
        deadline = time.monotonic() + timeout
        try:
            if self.process is None:
                self._start()
            assert self.diagnostics is not None and self.channel is not None
            self.diagnostics.seek(0)
            self.diagnostics.truncate()
            self.sequence += 1
            message: WorkerRequest = {
                "id": self.sequence,
                "request": str(request),
                "response": str(response),
            }
            self.channel.settimeout(max(0.001, deadline - time.monotonic()))
            self.channel.sendall((json.dumps(message) + "\n").encode())
            received = b""
            while b"\n" not in received:
                remaining = deadline - time.monotonic()
                if (
                    remaining <= 0
                    or not select.select([self.channel], [], [], remaining)[0]
                ):
                    raise subprocess.TimeoutExpired("PhotoKit worker", timeout)
                chunk = self.channel.recv(4096)
                if not chunk:
                    raise OSError("PhotoKit worker exited before replying")
                received += chunk
                if len(received) > MAX_CONTROL_RESPONSE_BYTES:
                    raise ValueError("Oversized PhotoKit worker response")
            reply = json.loads(received)
            if not isinstance(reply, dict) or reply.get("id") != self.sequence:
                raise ValueError("Mismatched PhotoKit worker response")
            if reply.get("error"):
                raise OSError(reply["error"])
            if reply.get("ok") is not True:
                raise ValueError("Invalid PhotoKit worker response")
        except BaseException as error:
            self.close()
            if isinstance(error, TimeoutError):
                raise subprocess.TimeoutExpired("PhotoKit worker", timeout) from error
            raise

    def close(self) -> None:
        if self.channel is not None:
            self.channel.close()
            self.channel = None
        if self.process is not None:
            if self.process.poll() is None:
                self.process.kill()
            self.process.wait()
            self.process = None
        if self.diagnostics is not None:
            self.diagnostics.close()
            self.diagnostics = None


@contextmanager
def bounded_downloads(
    seconds: float = DEFAULT_DOWNLOAD_TIMEOUT, progress: ExportProgress | None = None
) -> Iterator[DownloadBudget]:
    """Temporarily adapt osxphotos's private staging seam for one serial export."""
    budget = DownloadBudget(seconds, progress)
    original = PhotoExporter._stage_photo_for_export_with_photokit

    def stage(exporter: PhotoExporter, options: ExportOptions) -> StagedFiles:
        if options.dry_run:
            return original(exporter, options)
        return budget.stage(exporter, options)

    try:
        with patch.object(
            PhotoExporter, "_stage_photo_for_export_with_photokit", stage
        ):
            yield budget
    finally:
        budget.worker.close()


class _WorkerStagedFiles(StagedFiles):
    def __init__(self, *args: Any, **kwargs: Any) -> None:
        super().__init__(*args, **kwargs)
        if not hasattr(self, "uuids"):
            self.uuids: dict[str, str] = {}


def _stage_missing_live_resource(
    staged: StagedFiles, photo: Any, options: ExportOptions, directory: Path
) -> None:
    """Recover paired video when PhotoKit exports a Live Photo as a still.

    Some assets retain paired-video resources without the PhotoLive subtype.
    Upstream selects PhotoAsset for these and silently ignores video=True.
    Read the exact requested resource in the bounded worker; never substitute
    an original video for an edited one or suppress an upstream error.
    """
    component = "edited_live" if options.edited else "original_live"
    if (
        not photo.live_photo
        or not options.live_photo
        or staged.error
        or getattr(staged, component)
    ):
        return
    try:
        asset = PhotoLibrary().fetch_uuid(photo.uuid)
        resource_type = (
            Photos.PHAssetResourceTypeFullSizePairedVideo
            if options.edited
            else Photos.PHAssetResourceTypePairedVideo
        )
        resources = [r for r in asset._resources() if r.type() == resource_type]
        if len(resources) != 1:
            raise ValueError(
                f"Expected one {component} resource, found {len(resources)}"
            )
        data = asset._request_resource_data(resources[0])
        if not data:
            raise ValueError(f"PhotoKit returned empty {component} data")
        destination = directory / f"{component}.mov"
        destination.write_bytes(data)
        setattr(staged, component, str(destination))
    except Exception as error:
        staged.error.append(
            (photo.original_filename, f"Paired-video retrieval failed: {error}")
        )


def _worker(request: Path, response: Path) -> None:
    document = json.loads(request.read_text(encoding="utf-8"))
    photo = SimpleNamespace(**document["photo"], _verbose=lambda _: None)
    exporter = PhotoExporter(photo)
    exporter._temp_dir_path = request.parent
    options = ExportOptions(**document["options"])
    # The pinned osxphotos's RAW branch writes StagedFiles.uuids although its
    # constructor omits it. Supply the unused bookkeeping map only in the child;
    # the parent still uses the ordinary StagedFiles wire format.
    with patch("osxphotos.photoexporter.StagedFiles", _WorkerStagedFiles):
        staged = exporter._stage_photo_for_export_with_photokit(options)
    _stage_missing_live_resource(staged, photo, options, request.parent)
    response.write_text(json.dumps(staged.asdict()), encoding="utf-8")


def _serve(descriptor: int) -> None:
    with socket.socket(fileno=descriptor) as channel, channel.makefile("rb") as stream:
        for line in stream:
            message = json.loads(line)
            try:
                _worker(Path(message["request"]), Path(message["response"]))
                reply = {"id": message["id"], "ok": True}
            except Exception as error:
                traceback.print_exc()
                reply = {"id": message["id"], "error": str(error)}
            channel.sendall((json.dumps(reply) + "\n").encode())


if __name__ == "__main__":
    # DownloadWorker starts this module only as `--serve FD`.
    if sys.argv[1:-1] != ["--serve"]:
        sys.exit("usage: python -m photos_backup.apple_photos.downloads --serve FD")
    _serve(int(sys.argv[2]))
