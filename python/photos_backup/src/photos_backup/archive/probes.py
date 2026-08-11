from __future__ import annotations

import datetime
import os
import socket
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path


def real_is_mount(path: Path) -> bool:
    return os.path.ismount(path)


def real_hostname() -> str:
    return socket.gethostname()


def real_now() -> datetime.datetime:
    return datetime.datetime.now(datetime.UTC)


@dataclass(frozen=True)
class SystemProbes:
    """Filesystem, hostname, and clock access, injectable for tests."""

    is_mount: Callable[[Path], bool] = real_is_mount
    hostname: Callable[[], str] = real_hostname
    now: Callable[[], datetime.datetime] = real_now
