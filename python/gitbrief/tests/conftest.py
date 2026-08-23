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
def isolated_config(tmp_path: Path, monkeypatch: pytest.MonkeyPatch, use_config_paths):
    """Redirect config and history paths to a tmp directory."""
    import gitbrief.history as history_mod

    gitbrief_dir = tmp_path / ".gitbrief"
    gitbrief_dir.mkdir()
    cfg_path = gitbrief_dir / "config.json"

    use_config_paths(tmp_path / ".gitbrief.json", cfg_path, gitbrief_dir)
    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", gitbrief_dir)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", gitbrief_dir / "history")

    return cfg_path


@pytest.fixture
def fake_git_repo(tmp_path: Path) -> Path:
    """Create a minimal fake git repo (just a .git dir — no real commits)."""
    repo = tmp_path / "repo"
    repo.mkdir()
    (repo / ".git").mkdir()
    return repo


@pytest.fixture
def sample_commits() -> list[dict]:
    """One commit as extract_commits would return it."""
    return [
        {
            "sha": "abc12345",
            "subject": "feat: add feature",
            "body": "",
            "refs": [],
            "files_changed": 2,
            "insertions": 10,
            "deletions": 1,
        }
    ]


@pytest.fixture
def history_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Redirect gitbrief history at a temporary directory and return its path.

    The directory is not created; the history module creates it on demand.
    """
    import gitbrief.history as history_mod

    hist_dir = tmp_path / "history"
    monkeypatch.setattr(history_mod, "GITBRIEF_DIR", tmp_path)
    monkeypatch.setattr(history_mod, "HISTORY_DIR", hist_dir)
    return hist_dir


@pytest.fixture
def write_history(history_dir: Path):
    """Return a helper writing one history record into the temp history dir.

    The stem doubles as the timestamp unless one is passed explicitly.
    """

    def _write(stem: str, **overrides) -> Path:
        record = {
            "timestamp": f"{stem[:10]}T{stem[11:13]}:{stem[13:15]}:{stem[15:17]}",
            "projects": ["proj1"],
            "since": "2026-02-01",
            "until": None,
            "backend": "claude",
            "commit_count": 1,
            "summary": "summary",
        }
        record.update(overrides)
        history_dir.mkdir(parents=True, exist_ok=True)
        path = history_dir / f"{stem}.json"
        path.write_text(json.dumps(record))
        return path

    return _write


@pytest.fixture
def use_config_paths(monkeypatch: pytest.MonkeyPatch):
    """Return a helper pointing gitbrief.config at temporary paths."""

    def _use(old_path: Path, new_path: Path, gitbrief_dir: Path) -> None:
        import gitbrief.config as config_mod

        monkeypatch.setattr(config_mod, "OLD_CONFIG_PATH", old_path)
        monkeypatch.setattr(config_mod, "NEW_CONFIG_PATH", new_path)
        monkeypatch.setattr(config_mod, "GITBRIEF_DIR", gitbrief_dir)
        monkeypatch.setattr(config_mod, "CONFIG_PATH", new_path)

    return _use


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
