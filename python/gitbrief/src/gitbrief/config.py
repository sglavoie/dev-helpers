import json
from pathlib import Path

import click

GITBRIEF_DIR = Path.home() / ".gitbrief"
NEW_CONFIG_PATH = GITBRIEF_DIR / "config.json"
OLD_CONFIG_PATH = Path.home() / ".gitbrief.json"
CONFIG_PATH = NEW_CONFIG_PATH  # canonical save location

VALID_BACKENDS = {"claude", "copilot"}

DEFAULT_CONFIG = {
    "projects": {},
    "groups": {},
    "settings": {"backend": "claude", "timeout": 120, "retries": 2, "max_commits": 100},
    "last_summary": {},
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
    # Check new location first, then migrate from old location
    if NEW_CONFIG_PATH.exists():
        path = NEW_CONFIG_PATH
    elif OLD_CONFIG_PATH.exists():
        path = OLD_CONFIG_PATH
    else:
        return json.loads(json.dumps(DEFAULT_CONFIG))

    with open(path) as f:
        config = json.load(f)
    config["projects"] = _normalize_projects(config.get("projects", {}))
    config.setdefault("groups", {})
    config.setdefault("last_summary", {})

    # Migrate from old path to new path transparently
    if path == OLD_CONFIG_PATH:
        GITBRIEF_DIR.mkdir(parents=True, exist_ok=True)
        with open(NEW_CONFIG_PATH, "w") as f:
            json.dump(config, f, indent=2)
            f.write("\n")
        OLD_CONFIG_PATH.unlink()

    return config


def save_config(config: dict) -> None:
    GITBRIEF_DIR.mkdir(parents=True, exist_ok=True)
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
    if key == "max_commits":
        try:
            int_val = int(value)
        except ValueError:
            raise click.ClickException(
                f"'max_commits' must be an integer between 10 and 1000, got: {value!r}"
            )
        if not (10 <= int_val <= 1000):
            raise click.ClickException(
                f"'max_commits' must be between 10 and 1000, got: {int_val}"
            )
        store_value = int_val
    config = load_config()
    config.setdefault("settings", {})[key] = store_value
    save_config(config)


def get_last_summary(alias: str) -> str | None:
    """Return the ISO timestamp of the last summary for a project, or None."""
    config = load_config()
    return config.get("last_summary", {}).get(alias)


def set_last_summary(alias: str, timestamp: str) -> None:
    """Record the timestamp of the latest summary for a project."""
    config = load_config()
    config.setdefault("last_summary", {})[alias] = timestamp
    save_config(config)


def create_group(config: dict, name: str, aliases: list[str]) -> None:
    """Create a named group of project aliases."""
    if name in config.get("groups", {}):
        raise click.ClickException(f"Group '{name}' already exists.")
    projects = config.get("projects", {})
    for alias in aliases:
        if alias not in projects:
            raise click.ClickException(
                f"Unknown project alias '{alias}'. Register it first."
            )
    config.setdefault("groups", {})[name] = list(aliases)


def delete_group(config: dict, name: str) -> None:
    """Delete a named group."""
    if name not in config.get("groups", {}):
        raise click.ClickException(f"Group '{name}' not found.")
    del config["groups"][name]


def add_to_group(config: dict, group_name: str, alias: str) -> None:
    """Add a project alias to an existing group."""
    if group_name not in config.get("groups", {}):
        raise click.ClickException(f"Group '{group_name}' not found.")
    if alias not in config.get("projects", {}):
        raise click.ClickException(
            f"Unknown project alias '{alias}'. Register it first."
        )
    if alias in config["groups"][group_name]:
        raise click.ClickException(
            f"'{alias}' is already in group '{group_name}'."
        )
    config["groups"][group_name].append(alias)


def remove_from_group(config: dict, group_name: str, alias: str) -> None:
    """Remove a project alias from a group."""
    if group_name not in config.get("groups", {}):
        raise click.ClickException(f"Group '{group_name}' not found.")
    if alias not in config["groups"][group_name]:
        raise click.ClickException(
            f"'{alias}' is not in group '{group_name}'."
        )
    config["groups"][group_name].remove(alias)


def resolve_group(config: dict, name: str) -> list[str]:
    """Return the list of project aliases in a group."""
    groups = config.get("groups", {})
    if name not in groups:
        raise click.ClickException(f"Group '{name}' not found.")
    return list(groups[name])


def list_groups(config: dict) -> dict:
    """Return all groups as a dict of {name: [aliases]}."""
    return dict(config.get("groups", {}))
