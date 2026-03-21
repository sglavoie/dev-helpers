"""Shared fixtures for gitbrief tests."""

import json
import subprocess
from pathlib import Path

import pytest


@pytest.fixture
def config_path(tmp_path: Path) -> Path:
    """Return a temporary config file path (does not create the file)."""
    return tmp_path / ".gitbrief.json"


@pytest.fixture
def minimal_config(config_path: Path) -> Path:
    """Write a minimal valid config and return its path."""
    config = {
        "projects": {},
        "settings": {
            "backend": "claude",
            "timeout": 120,
            "retries": 2,
            "max_commits": 100,
        },
    }
    config_path.write_text(json.dumps(config))
    return config_path


@pytest.fixture
def sample_git_log() -> str:
    fixtures = Path(__file__).parent / "fixtures" / "sample_git_log.txt"
    return fixtures.read_text()


@pytest.fixture
def real_git_repo(tmp_path: Path) -> Path:
    """Create a real git repository with a few known commits."""
    repo = tmp_path / "repo"
    repo.mkdir()

    def _git(*args: str) -> None:
        subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True)

    _git("init")
    _git("config", "user.email", "test@example.com")
    _git("config", "user.name", "Test User")
    _git("config", "commit.gpgsign", "false")

    commits = [
        ("README.md", "# Hello", "docs: initial readme"),
        ("main.py", "print('hello')", "feat: add main module"),
        ("utils.py", "def helper():\n    pass", "feat: add utils"),
    ]
    for filename, content, msg in commits:
        (repo / filename).write_text(content)
        _git("add", filename)
        _git("commit", "-m", msg)

    return repo
