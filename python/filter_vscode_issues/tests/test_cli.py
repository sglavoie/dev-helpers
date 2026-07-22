"""Tests for filter-vscode-issues."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from filter_vscode_issues.cli import extract, load_exclusions


class DefaultExclusionsTest(unittest.TestCase):
    def test_unknown_word_diagnostics_are_excluded_without_config(self) -> None:
        entries = [
            {
                "resource": "/project/diffusion-source.test.ts",
                "message": '"worktree": Unknown word.',
                "startLineNumber": 965,
                "endLineNumber": 965,
            },
            {
                "resource": "/project/diffusion-source.test.ts",
                "message": "Type 'string' is not assignable to type 'number'.",
                "startLineNumber": 966,
                "endLineNumber": 966,
            },
        ]

        with tempfile.TemporaryDirectory() as config_home:
            with patch.dict("os.environ", {"XDG_CONFIG_HOME": config_home}):
                result = extract(entries, load_exclusions())

        self.assertEqual(result, [entries[1]])

    def test_configured_exclusions_are_added_to_defaults(self) -> None:
        entries = [
            {
                "resource": "/project/source.ts",
                "message": '"worktree": Unknown word.',
                "startLineNumber": 1,
                "endLineNumber": 1,
            },
            {
                "resource": "/project/node_modules/package/source.ts",
                "message": "A type error.",
                "startLineNumber": 2,
                "endLineNumber": 2,
            },
        ]

        with tempfile.TemporaryDirectory() as config_home:
            config_dir = Path(config_home) / "filter-vscode-issues"
            config_dir.mkdir()
            (config_dir / "config.toml").write_text(
                "[exclude]\nresource = ['/node_modules/']\n"
            )
            with patch.dict("os.environ", {"XDG_CONFIG_HOME": config_home}):
                result = extract(entries, load_exclusions())

        self.assertEqual(result, [])


if __name__ == "__main__":
    unittest.main()
