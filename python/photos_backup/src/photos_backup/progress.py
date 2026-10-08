"""Live status for exports and verification; the heartbeat never touches state."""

from __future__ import annotations

import threading
import time
from collections.abc import Callable, Iterator
from contextlib import contextmanager

import click


class ExportProgress:
    def __init__(
        self,
        *,
        clock: Callable[[], float] = time.monotonic,
        sink: Callable[[str], None] | None = None,
        terminal: bool | None = None,
        item_label: str = "assets processed",
        show_downloads: bool = True,
    ) -> None:
        self.clock = clock
        self.item_label = item_label
        self.show_downloads = show_downloads
        self.terminal = (
            click.get_text_stream("stderr").isatty() if terminal is None else terminal
        )
        self.sink = sink or (
            lambda line: click.echo(line, err=True, nl=not self.terminal)
        )
        self.interval = 1 if self.terminal else 10
        self.timings: dict[str, float] = {}
        self.processed = 0
        self.selected: int | None = None
        self.unresolved = 0
        self._phase = "Starting"
        self._detail = ""
        self._budget: float | None = None
        self._started = clock()
        self._last = self._started
        self._lock = threading.RLock()
        self._stop = threading.Event()
        self._thread: threading.Thread | None = None

    def __enter__(self) -> ExportProgress:
        self._thread = threading.Thread(target=self._heartbeat, daemon=True)
        self._thread.start()
        return self

    def __exit__(self, *_exc) -> None:
        self._stop.set()
        if self._thread is not None:
            self._thread.join()
        if self.terminal:
            self.sink("\n")

    def _heartbeat(self) -> None:
        while not self._stop.wait(self.interval):
            self.tick()

    def tick(self, *, force: bool = False) -> None:
        with self._lock:
            now = self.clock()
            if not force and now - self._last < self.interval:
                return
            elapsed = max(0, now - self._started)
            count = (
                f" | {self.item_label} {self.processed}/{self.selected}"
                if self.selected is not None
                else ""
            )
            budget = (
                f" | budget remaining {max(0, self._budget - elapsed):.0f}s"
                if self._budget is not None
                else ""
            )
            failures = (
                f" | unresolved downloads {self.unresolved}"
                if self.show_downloads
                else ""
            )
            label = f"{self._phase}: {self._detail}" if self._detail else self._phase
            line = f"{label} | elapsed {elapsed:.1f}s{count}{budget}{failures}"
            if self.terminal:
                line = "\r\033[2K" + line
            self.sink(line)
            self._last = now

    def message(self, message: str) -> None:
        with self._lock:
            self.sink(
                ("\r\033[2K" if self.terminal else "")
                + message
                + ("\n" if self.terminal else "")
            )

    @contextmanager
    def phase(
        self,
        name: str,
        detail: str = "",
        *,
        budget: float | None = None,
        announce: bool = True,
    ) -> Iterator[None]:
        with self._lock:
            previous = (self._phase, self._detail, self._started, self._budget)
            started = self.clock()
            self._phase, self._detail, self._started, self._budget = (
                name,
                detail,
                started,
                budget,
            )
            # Logs get the closing line with its duration; the heartbeat
            # reports a long phase, so a zero-elapsed opening line is noise.
            self.tick(force=announce and self.terminal)
        try:
            yield
        finally:
            with self._lock:
                elapsed = self.clock() - started
                self.timings[name] = self.timings.get(name, 0) + elapsed
                self.tick(force=announce)
                self._phase, self._detail, self._started, self._budget = previous

    def selection(self, count: int) -> None:
        with self._lock:
            self.selected = count
            self.processed = 0

    def asset_done(self) -> None:
        with self._lock:
            self.processed += 1
            self.tick()
