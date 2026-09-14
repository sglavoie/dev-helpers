from __future__ import annotations

import os
import tomllib
from dataclasses import dataclass
from pathlib import Path
from typing import NoReturn

import click

DEFAULT_CONFIG_PATH = Path("~/.config/osxphotos-backup/photos-backup.toml")

WEEKDAYS = (
    "monday",
    "tuesday",
    "wednesday",
    "thursday",
    "friday",
    "saturday",
    "sunday",
)

APPLE_PHOTOS_KEYS = (
    "volume",
    "archive",
    "library",
    "legacy_export",
    "limit_export",
    "spouse_device_models",
    "incremental_overlap_days",
    "full_export_weekday",
    "full_export_max_age_days",
    "mirror",
    "cleanup_max_assets",
    "cleanup_max_fraction",
)
SD_CARD_KEYS = ("source", "destination", "exclude_file")
SSD_KEYS = ("source", "destination", "exclude_file")
RCLONE_KEYS = ("remote", "source")


class MissingSection(click.UsageError):
    """Raised when a section an optional workflow depends on is absent."""


@dataclass(frozen=True)
class ApplePhotosConfig:
    volume: Path
    archive: Path
    library: Path
    legacy_export: Path | None
    limit_export: int
    spouse_device_models: tuple[str, ...]
    incremental_overlap_days: int
    full_export_weekday: int
    full_export_max_age_days: int
    mirror: bool
    cleanup_max_assets: int
    cleanup_max_fraction: float
    # False only when --volume re-pointed the run at a directory the user named
    # explicitly, which need not be a mount point.
    require_mounted_volume: bool = True


@dataclass(frozen=True)
class SdCardConfig:
    source: Path
    destination: Path
    exclude_file: Path | None


@dataclass(frozen=True)
class SsdConfig:
    source: Path
    destination: Path
    exclude_file: Path | None


@dataclass(frozen=True)
class RcloneConfig:
    remote: str
    source: Path | None


def resolve_config_path(config_path: Path | None = None) -> Path:
    return Path(config_path or DEFAULT_CONFIG_PATH).expanduser()


def load_apple_photos_config(
    config_path: Path | None = None, *, volume: Path | None = None
) -> ApplePhotosConfig:
    """Load the section, optionally re-rooting the archive under `volume`.

    The configured pair is validated either way, so `--volume` relaxes where the
    archive lives but never what the configuration file is allowed to say.
    """
    section = _read_section(config_path, "apple_photos", APPLE_PHOTOS_KEYS)
    configured_volume = section.required_path("volume")
    configured_archive = section.required_path("archive")
    library = section.required_path("library")
    legacy_export = section.optional_path("legacy_export")

    if configured_archive == configured_volume or not configured_archive.is_relative_to(
        configured_volume
    ):
        section.fail(
            "archive",
            f"must be inside volume '{configured_volume}' (got '{configured_archive}')",
        )

    effective_volume = configured_volume
    archive = configured_archive
    if volume is not None:
        effective_volume = volume
        archive = volume / configured_archive.relative_to(configured_volume)

    if legacy_export is not None and legacy_export.is_relative_to(archive):
        section.fail(
            "legacy_export",
            f"must not be inside archive '{archive}' (got '{legacy_export}')",
        )

    return ApplePhotosConfig(
        volume=effective_volume,
        archive=archive,
        library=library,
        legacy_export=legacy_export,
        limit_export=section.integer("limit_export", default=0, minimum=0),
        spouse_device_models=section.string_list("spouse_device_models"),
        incremental_overlap_days=section.integer(
            "incremental_overlap_days", default=14, minimum=0, maximum=365
        ),
        full_export_weekday=section.weekday("full_export_weekday", default="monday"),
        full_export_max_age_days=section.integer(
            "full_export_max_age_days", default=7, minimum=1, maximum=365
        ),
        mirror=section.boolean("mirror", default=True),
        cleanup_max_assets=section.integer("cleanup_max_assets", default=10, minimum=0),
        cleanup_max_fraction=section.fraction("cleanup_max_fraction", default=0.001),
        require_mounted_volume=volume is None,
    )


def normalize_volume_override(raw: str) -> Path:
    """Expand and validate a `--volume` value the way config paths are handled."""
    expanded = Path(os.path.expandvars(raw)).expanduser()
    if not expanded.is_absolute():
        raise click.BadParameter(f"must be an absolute path (got '{expanded}')")
    return expanded


def load_sd_card_config(config_path: Path | None = None) -> SdCardConfig:
    section = _read_section(config_path, "sd_card", SD_CARD_KEYS)
    return SdCardConfig(
        source=section.required_path("source"),
        destination=section.required_path("destination"),
        exclude_file=section.optional_path("exclude_file"),
    )


def load_ssd_config(config_path: Path | None = None) -> SsdConfig:
    section = _read_section(config_path, "ssd", SSD_KEYS)
    return SsdConfig(
        source=section.required_path("source"),
        destination=section.required_path("destination"),
        exclude_file=section.optional_path("exclude_file"),
    )


def load_rclone_config(config_path: Path | None = None) -> RcloneConfig:
    section = _read_section(config_path, "rclone", RCLONE_KEYS)
    return RcloneConfig(
        remote=section.required_string("remote"),
        source=section.optional_path("source"),
    )


def resolve_rclone_source(
    config: RcloneConfig, config_path: Path | None = None
) -> Path:
    if config.source is not None:
        return config.source
    return load_ssd_config(config_path).destination


@dataclass(frozen=True)
class _Section:
    path: Path
    name: str
    values: dict

    def fail(self, key: str, message: str) -> NoReturn:
        raise click.UsageError(f"{self.path} [{self.name}] {key}: {message}")

    def required_string(self, key: str) -> str:
        if key not in self.values:
            self.fail(key, "is required")
        return self._string(key)

    def required_path(self, key: str) -> Path:
        return self._to_path(key, self.required_string(key))

    def optional_path(self, key: str) -> Path | None:
        if key not in self.values:
            return None
        return self._to_path(key, self._string(key))

    def _string(self, key: str) -> str:
        raw = self.values.get(key)
        if not isinstance(raw, str) or not raw.strip():
            self.fail(key, f"must be a non-empty string (got {raw!r})")
        return raw.strip()

    def integer(
        self,
        key: str,
        *,
        default: int,
        minimum: int = 0,
        maximum: int = 2**31 - 1,
    ) -> int:
        raw = self.values.get(key, default)
        if isinstance(raw, bool) or not isinstance(raw, int):
            self.fail(key, f"must be an integer (got {raw!r})")
        if not minimum <= raw <= maximum:
            self.fail(key, f"must be between {minimum} and {maximum} (got {raw})")
        return raw

    def fraction(
        self,
        key: str,
        *,
        default: float,
        minimum: float = 0.0,
        maximum: float = 1.0,
    ) -> float:
        raw = self.values.get(key, default)
        if isinstance(raw, bool) or not isinstance(raw, (int, float)):
            self.fail(key, f"must be a number (got {raw!r})")
        value = float(raw)
        if not minimum <= value <= maximum:
            self.fail(key, f"must be between {minimum} and {maximum} (got {value})")
        return value

    def boolean(self, key: str, *, default: bool) -> bool:
        raw = self.values.get(key, default)
        if not isinstance(raw, bool):
            self.fail(key, f"must be true or false (got {raw!r})")
        return raw

    def string_list(self, key: str) -> tuple[str, ...]:
        raw = self.values.get(key, [])
        if not isinstance(raw, list):
            self.fail(key, f"must be an array of strings (got {raw!r})")
        for item in raw:
            if not isinstance(item, str) or not item.strip():
                self.fail(key, f"must contain only non-empty strings (got {item!r})")
        return tuple(item.strip() for item in raw)

    def weekday(self, key: str, *, default: str) -> int:
        raw = self.values.get(key, default)
        if not isinstance(raw, str) or raw.strip().lower() not in WEEKDAYS:
            self.fail(key, f"must be one of {', '.join(WEEKDAYS)} (got {raw!r})")
        return WEEKDAYS.index(raw.strip().lower())

    def _to_path(self, key: str, raw: str) -> Path:
        expanded = Path(os.path.expandvars(raw)).expanduser()
        if not expanded.is_absolute():
            self.fail(key, f"must be an absolute path (got '{expanded}')")
        return expanded


def _read_section(
    config_path: Path | None, name: str, known_keys: tuple[str, ...]
) -> _Section:
    path = resolve_config_path(config_path)
    if not path.is_file():
        raise click.UsageError(
            f"Could not find configuration file {path}; "
            "create it or pass --config with another path"
        )

    try:
        document = tomllib.loads(path.read_text())
    except tomllib.TOMLDecodeError as error:
        raise click.UsageError(f"{path} is not valid TOML: {error}") from error
    except OSError as error:
        raise click.UsageError(f"Could not read {path}: {error}") from error

    values = document.get(name)
    if values is None:
        raise MissingSection(f"{path} is missing the required [{name}] section")
    if not isinstance(values, dict):
        raise click.UsageError(f"{path}: [{name}] must be a table")

    unknown = sorted(set(values) - set(known_keys))
    if unknown:
        raise click.UsageError(
            f"{path} [{name}]: unknown key(s) {', '.join(unknown)}; "
            f"known keys are {', '.join(known_keys)}"
        )

    return _Section(path=path, name=name, values=values)
