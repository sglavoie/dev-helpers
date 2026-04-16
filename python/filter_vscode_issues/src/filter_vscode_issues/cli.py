"""Trim VS Code Problems-panel JSON down to the essentials."""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
from typing import Any

FIELDS: tuple[str, ...] = (
    "resource",
    "message",
    "startLineNumber",
    "endLineNumber",
)


def extract(entries: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Project, dedupe, and sort issue entries."""
    seen: set[tuple[Any, ...]] = set()
    out: list[dict[str, Any]] = []
    for entry in entries:
        item = {field: entry.get(field) for field in FIELDS}
        key = tuple(item[f] for f in FIELDS)
        if key in seen:
            continue
        seen.add(key)
        out.append(item)
    out.sort(key=lambda e: (e["resource"] or "", e["startLineNumber"] or 0))
    return out


def emit(payload: str) -> None:
    """Pretty-print payload via jq when available, else stdlib indent."""
    if shutil.which("jq"):
        subprocess.run(["jq", "."], input=payload, text=True, check=True)
        return
    print(
        "filter-vscode-issues: jq not found, falling back to plain JSON",
        file=sys.stderr,
    )
    print(payload)


def main() -> int:
    raw = sys.stdin.read().strip()
    if not raw:
        print("filter-vscode-issues: no input on stdin", file=sys.stderr)
        return 1
    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        print(f"filter-vscode-issues: invalid JSON: {exc}", file=sys.stderr)
        return 1
    if isinstance(data, dict):
        data = [data]
    if not isinstance(data, list):
        print(
            "filter-vscode-issues: expected JSON array or object",
            file=sys.stderr,
        )
        return 1
    filtered = extract(data)
    emit(json.dumps(filtered, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
