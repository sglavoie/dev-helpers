from __future__ import annotations

import subprocess
import re
from collections import deque
from collections.abc import Iterator
from contextlib import contextmanager

import click

_DIAGNOSTIC = re.compile(r"\b(error|fatal|failed|failure)\b|rsync:", re.IGNORECASE)


def _clean_line(line: str) -> str:
    return "".join(
        character for character in click.unstyle(line) if character.isprintable()
    ).strip()


def interactive_transfers() -> bool:
    """Transfer output is streamed to stdout, which may be redirected to a log."""
    return click.get_text_stream("stdout").isatty()


@contextmanager
def transfer_errors(step: str, executable: str) -> Iterator[None]:
    """Translate expected transfer failures while keeping streamed diagnostics."""
    try:
        yield
    except subprocess.CalledProcessError as error:
        raise click.ClickException(
            f"{step}: {transfer_failure(executable, error.returncode, error.output)}"
        ) from error
    except FileNotFoundError as error:
        raise click.ClickException(
            f"{step}: {executable} was not found. Install it and ensure it is on PATH."
        ) from error
    except OSError as error:
        raise click.ClickException(
            f"{step}: could not run {executable}: {error}"
        ) from error


def transfer_failure(
    executable: str, returncode: int, output: str | bytes | None
) -> str:
    """Keep a short diagnostic in the summary, even after progress scrolls away."""
    if isinstance(output, bytes):
        output = output.decode(errors="replace")
    lines = [_clean_line(line) for line in (output or "").splitlines() if line.strip()]
    diagnostics = [line for line in lines if _DIAGNOSTIC.search(line)]
    excerpt = "; ".join(
        line[:240] for line in list(dict.fromkeys(diagnostics or lines))[-3:]
    )
    detail = f" {excerpt}" if excerpt else " See the transfer output above for details."
    return f"{executable} exited with code {returncode}.{detail}"


def stream_command(
    command: list[str], *, check: bool = False
) -> subprocess.CompletedProcess[str]:
    """Show output as it arrives, retaining a bounded tail for transfer statistics.

    Merge stderr into stdout so neither pipe can fill while reading the other.
    Text mode also turns carriage-return progress updates into readable lines.
    Failed commands also retain bounded diagnostics that scrolled out of the tail.
    """
    tail: deque[str] = deque(maxlen=200)
    diagnostics: deque[tuple[int, str]] = deque(maxlen=20)
    line_count = 0
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
                line_count += 1
                cleaned = _clean_line(line)
                if _DIAGNOSTIC.search(cleaned):
                    diagnostics.append((line_count, cleaned[:240] + "\n"))
            returncode = process.wait()
        except BaseException:
            process.kill()
            raise
    earlier_errors = (
        "".join(line for index, line in diagnostics if index <= line_count - len(tail))
        if returncode != 0
        else ""
    )
    result = subprocess.CompletedProcess(
        command, returncode, stdout=earlier_errors + "".join(tail)
    )
    if check:
        result.check_returncode()
    return result
