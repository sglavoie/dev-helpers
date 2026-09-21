from __future__ import annotations

import datetime
import os
import socket
import subprocess
import sys
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from photos_backup.archive.errors import ArchiveUnavailable


def real_is_mount(path: Path) -> bool:
    return os.path.ismount(path)


def real_hostname() -> str:
    """Use the Mac's persistent local name, independent of DNS, DHCP, or VPNs."""
    if sys.platform != "darwin":
        return socket.gethostname()
    try:
        result = subprocess.run(
            ["/usr/sbin/scutil", "--get", "LocalHostName"],
            capture_output=True,
            text=True,
            check=True,
            timeout=5,
        )
    except (OSError, subprocess.SubprocessError) as error:
        raise ArchiveUnavailable(
            f"Could not read this Mac's LocalHostName with scutil: {error}"
        ) from error
    name = result.stdout.strip()
    if not name:
        raise ArchiveUnavailable("scutil returned an empty LocalHostName for this Mac")
    # Preserve the spelling used by existing archives from before a network
    # changed gethostname(). Never fall back to that network-dependent name.
    return f"{name}.local"


def real_now() -> datetime.datetime:
    return datetime.datetime.now(datetime.UTC)


@dataclass(frozen=True)
class SystemProbes:
    """Filesystem, hostname, and clock access, injectable for tests."""

    is_mount: Callable[[Path], bool] = real_is_mount
    hostname: Callable[[], str] = real_hostname
    now: Callable[[], datetime.datetime] = real_now
