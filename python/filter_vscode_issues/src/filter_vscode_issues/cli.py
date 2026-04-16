"""Trim VS Code Problems-panel JSON down to the essentials."""

from __future__ import annotations

import argparse
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
import tomllib
from pathlib import Path
from typing import Any

FIELDS: tuple[str, ...] = (
    "resource",
    "message",
    "startLineNumber",
    "endLineNumber",
)

CONFIG_FILENAME = "config.toml"
APP_DIR = "filter-vscode-issues"

DEFAULT_TEMPLATE = """\
# filter-vscode-issues configuration
#
# Under [exclude], each key is an issue field name and each value is a list
# of Python regex patterns. An issue is dropped if any pattern matches the
# corresponding field value (re.search — substring match, case-sensitive;
# use (?i) inline flag for case-insensitive matching).
#
# Example — drop cspell "Unknown word" warnings and node_modules issues:
#
# [exclude]
# message  = [': Unknown word\\.$']
# resource = ['/node_modules/']
"""


# ---------------------------------------------------------------------------
# Config helpers
# ---------------------------------------------------------------------------


def config_path() -> Path:
    """Return the resolved config file path, honouring XDG_CONFIG_HOME."""
    xdg = os.environ.get("XDG_CONFIG_HOME", "").strip()
    base = Path(xdg) if xdg else Path.home() / ".config"
    return base / APP_DIR / CONFIG_FILENAME


def load_exclusions() -> dict[str, list[re.Pattern[str]]]:
    """Load and compile exclusion patterns from config; return {} if absent."""
    path = config_path()
    if not path.exists():
        return {}
    try:
        with open(path, "rb") as fh:
            data = tomllib.load(fh)
    except OSError as exc:
        print(f"filter-vscode-issues: cannot read config: {exc}", file=sys.stderr)
        sys.exit(1)
    except tomllib.TOMLDecodeError as exc:
        print(f"filter-vscode-issues: malformed TOML config: {exc}", file=sys.stderr)
        sys.exit(1)

    exclusions: dict[str, list[re.Pattern[str]]] = {}
    for field, patterns in data.get("exclude", {}).items():
        compiled: list[re.Pattern[str]] = []
        for pat in patterns:
            try:
                compiled.append(re.compile(pat))
            except re.error as exc:
                print(
                    f"filter-vscode-issues: bad regex {pat!r} in config: {exc}",
                    file=sys.stderr,
                )
                sys.exit(1)
        exclusions[field] = compiled
    return exclusions


def ensure_config_file() -> Path:
    """Create parent dirs and seed the template if the config doesn't exist."""
    path = config_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    if not path.exists():
        path.write_text(DEFAULT_TEMPLATE)
    return path


def launch_editor(path: Path) -> int:
    """Open *path* in $VISUAL / $EDITOR / vi; return the editor's exit code."""
    editor_str = os.environ.get("VISUAL") or os.environ.get("EDITOR") or "vi"
    tokens = shlex.split(editor_str)
    result = subprocess.run([*tokens, str(path)])
    return result.returncode


# ---------------------------------------------------------------------------
# Core pipeline
# ---------------------------------------------------------------------------


def extract(
    entries: list[dict[str, Any]],
    exclusions: dict[str, list[re.Pattern[str]]],
) -> list[dict[str, Any]]:
    """Filter, project, dedupe, and sort issue entries."""
    seen: set[tuple[Any, ...]] = set()
    out: list[dict[str, Any]] = []
    for entry in entries:
        # Exclusion check
        excluded = False
        for field, patterns in exclusions.items():
            value = str(entry.get(field) or "")
            for pat in patterns:
                if pat.search(value):
                    excluded = True
                    break
            if excluded:
                break
        if excluded:
            continue

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


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def main() -> int:
    parser = argparse.ArgumentParser(
        prog="filter-vscode-issues",
        description="Trim VS Code Problems-panel JSON down to the essentials.",
    )
    subparsers = parser.add_subparsers(dest="command")

    config_parser = subparsers.add_parser("config", help="Manage configuration.")
    config_sub = config_parser.add_subparsers(dest="config_command")
    config_sub.add_parser("edit", help="Open the config file in $EDITOR.")
    config_sub.add_parser("path", help="Print the config file path.")

    args = parser.parse_args()

    if args.command is None:
        # Default filter pipeline
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
        exclusions = load_exclusions()
        filtered = extract(data, exclusions)
        emit(json.dumps(filtered, ensure_ascii=False, indent=2))
        return 0

    if args.command == "config":
        if args.config_command == "path":
            print(config_path())
            return 0
        if args.config_command == "edit":
            path = ensure_config_file()
            return launch_editor(path)
        config_parser.print_help()
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
