from __future__ import annotations

import subprocess
from collections import deque

import click


def stream_command(
    command: list[str], *, check: bool = False
) -> subprocess.CompletedProcess:
    """Show output as it arrives, retaining a bounded tail for transfer statistics.

    Merge stderr into stdout so neither pipe can fill while reading the other.
    Text mode also turns carriage-return progress updates into readable lines.
    """
    tail: deque[str] = deque(maxlen=200)
    with subprocess.Popen(
        command,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        errors="replace",
    ) as process:
        try:
            assert process.stdout is not None
            for line in process.stdout:
                click.echo(line, nl=False)
                tail.append(line)
            returncode = process.wait()
        except BaseException:
            process.kill()
            raise
    result = subprocess.CompletedProcess(command, returncode, stdout="".join(tail))
    if check:
        result.check_returncode()
    return result
