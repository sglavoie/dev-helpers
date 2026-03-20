import json
from pathlib import Path

import click

CONFIG_PATH = Path.home() / ".gitbrief.json"

VALID_BACKENDS = {"claude", "copilot"}

DEFAULT_CONFIG = {
    "projects": {},
    "settings": {"backend": "claude", "timeout": 120, "retries": 2},
}


def _normalize_projects(projects: dict) -> dict:
    """Normalize project entries to {path: str, backend: str | None}."""
    normalized = {}
    for alias, value in projects.items():
        if isinstance(value, str):
            normalized[alias] = {"path": value, "backend": None}
        else:
            normalized[alias] = value
    return normalized


def load_config() -> dict:
    if not CONFIG_PATH.exists():
        return json.loads(json.dumps(DEFAULT_CONFIG))
    with open(CONFIG_PATH) as f:
        config = json.load(f)
    config["projects"] = _normalize_projects(config.get("projects", {}))
    return config


def save_config(config: dict) -> None:
    with open(CONFIG_PATH, "w") as f:
        json.dump(config, f, indent=2)
        f.write("\n")


def add_project(alias: str, path: str, backend: str | None = None) -> None:
    resolved = Path(path).resolve()
    git_dir = resolved / ".git"
    if not git_dir.exists():
        raise click.ClickException(f"Not a git repository: {resolved}")

    if backend is not None and backend not in VALID_BACKENDS:
        raise click.ClickException(
            f"Invalid backend '{backend}'. Must be one of: {', '.join(sorted(VALID_BACKENDS))}"
        )

    config = load_config()
    if alias in config["projects"]:
        raise click.ClickException(f"Alias '{alias}' already exists. Remove it first.")
    config["projects"][alias] = {"path": str(resolved), "backend": backend}
    save_config(config)


def set_project_backend(alias: str, backend: str | None) -> None:
    if backend is not None and backend not in VALID_BACKENDS:
        raise click.ClickException(
            f"Invalid backend '{backend}'. Must be one of: {', '.join(sorted(VALID_BACKENDS))}"
        )
    config = load_config()
    if alias not in config["projects"]:
        raise click.ClickException(f"Alias '{alias}' not found.")
    config["projects"][alias]["backend"] = backend
    save_config(config)


def remove_project(alias: str) -> None:
    config = load_config()
    if alias not in config["projects"]:
        raise click.ClickException(f"Alias '{alias}' not found.")
    del config["projects"][alias]
    save_config(config)


def get_project_backend(alias: str) -> str | None:
    config = load_config()
    project = config.get("projects", {}).get(alias)
    if project is None:
        return None
    return project.get("backend")


def get_setting(key: str) -> str | None:
    config = load_config()
    value = config.get("settings", {}).get(key)
    return str(value) if value is not None else None


def get_setting_int(key: str, default: int) -> int:
    config = load_config()
    value = config.get("settings", {}).get(key)
    if value is None:
        return default
    try:
        return int(value)
    except (ValueError, TypeError):
        return default


def set_setting(key: str, value: str) -> None:
    if key == "backend" and value not in VALID_BACKENDS:
        raise click.ClickException(
            f"Invalid backend '{value}'. Must be one of: {', '.join(sorted(VALID_BACKENDS))}"
        )
    store_value: str | int = value
    if key == "timeout":
        try:
            int_val = int(value)
        except ValueError:
            raise click.ClickException(
                f"'timeout' must be a positive integer, got: {value!r}"
            )
        if int_val <= 0:
            raise click.ClickException(
                f"'timeout' must be a positive integer, got: {int_val}"
            )
        store_value = int_val
    if key == "retries":
        try:
            int_val = int(value)
        except ValueError:
            raise click.ClickException(
                f"'retries' must be an integer between 0 and 5, got: {value!r}"
            )
        if not (0 <= int_val <= 5):
            raise click.ClickException(
                f"'retries' must be between 0 and 5, got: {int_val}"
            )
        store_value = int_val
    config = load_config()
    config.setdefault("settings", {})[key] = store_value
    save_config(config)
