"""Read-only setup diagnostics; never start an export or transfer."""

import json
import platform
import shutil
import subprocess
from collections.abc import Callable
from functools import partial
from importlib.metadata import PackageNotFoundError, version
from pathlib import Path
from typing import Any

import rich_click as click

from photos_backup.apple_photos.adapter import (
    export_db_version_supported,
    read_export_db_version,
    unsupported_export_db_reason,
)
from photos_backup.archive import open_archive
from photos_backup.cli.context import (
    apple_photos_config_from,
    config_path_from,
    suggested_command,
)
from photos_backup.config import (
    ApplePhotosConfig,
    MissingSection,
    RcloneConfig,
    SdCardConfig,
    SsdConfig,
    load_rclone_config,
    load_sd_card_config,
    load_ssd_config,
    resolve_rclone_source,
)
from photos_backup.copy_safety import check_copy_path, check_copy_paths
from photos_backup.errors import ACTION_REQUIRED_EXIT_CODE, ActionRequired
from photos_backup.presentation import print_terminal_table
from photos_backup.sd_card.folders import (
    uncovered_camera_folders,
    uncovered_camera_folders_message,
)

# check(label, operation) records PASS with the operation's detail, or its error.
Check = Callable[[str, Callable[[], str | None]], bool]
# report(status, label, detail) records a result without running anything.
Report = Callable[[str, str, str], None]

STATUS_STYLES = {
    "PASS": "green",
    "SKIP": "dim",
    "ACTION": "bold yellow",
    "FAIL": "bold red",
}


@click.command(
    help="Check configuration, local paths, and tools without running backups."
)
@click.option(
    "--json", "as_json", is_flag=True, help="Print versions and checks as JSON."
)
@click.pass_context
def doctor(ctx: click.Context, as_json: bool) -> None:
    failures: list[int] = []
    results: list[tuple[str, str, str]] = []

    def report(status: str, label: str, detail: str = "") -> None:
        results.append((status, label, detail))

    def check(label: str, operation: Callable[[], str | None]) -> bool:
        try:
            detail = operation()
        except (click.ClickException, OSError) as error:
            exit_code = (
                error.exit_code if isinstance(error, click.ClickException) else 1
            )
            failures.append(exit_code)
            # Exit 3 means a person can fix it, often by connecting a drive.
            report(
                "ACTION" if exit_code == ACTION_REQUIRED_EXIT_CODE else "FAIL",
                label,
                str(error),
            )
            return False
        report("PASS", label, detail or "")
        return True

    versions: dict[str, str | None] = {"python": platform.python_version()}
    for package in ("photos_backup", "click", "osxphotos"):
        try:
            versions[package] = version(package)
        except PackageNotFoundError:
            versions[package] = None
            failures.append(1)
            report("FAIL", package, "package metadata unavailable")

    path = config_path_from(ctx)
    configs: dict[str, Any] = {}
    for name, loader in (
        ("apple_photos", lambda: apple_photos_config_from(ctx)),
        ("sd_card", lambda: load_sd_card_config(path)),
        ("ssd", lambda: load_ssd_config(path)),
        ("rclone", lambda: load_rclone_config(path)),
    ):
        try:
            configs[name] = loader()
            report("PASS", f"[{name}] configuration", "")
        except MissingSection:
            report("SKIP", f"[{name}]", "not configured")
        except click.ClickException as error:
            failures.append(error.exit_code)
            report("FAIL", f"[{name}] configuration", str(error))

    apple, sd, ssd, remote = (
        configs.get(key) for key in ("apple_photos", "sd_card", "ssd", "rclone")
    )
    required = []
    if apple:
        required.append("exiftool")
        _check_apple_photos(apple, check)
    if sd or ssd:
        required.append("rsync")
    if remote:
        required.append("rclone")
    installed = {
        executable: check(executable, partial(_tool_version, executable))
        for executable in required
    }
    _check_local_copies(sd, ssd, check, report)
    if remote:
        _check_remote(remote, path, check, report, rclone_installed=installed["rclone"])
    exit_code = next((code for code in (2, 1, 3) if code in failures), 0)
    _print_results(versions, results, exit_code, as_json=as_json)
    if exit_code:
        ctx.exit(exit_code)


def _check_apple_photos(apple: ApplePhotosConfig, check: Check) -> None:
    export_db: Path | None = None

    def archive_check() -> str:
        nonlocal export_db
        with open_archive(apple, dry_run=True) as archive:
            export_db = archive.paths.export_db
            state = archive.state_store.load()
            return (
                "initialized"
                if state.initialized
                else f"not initialized; run {suggested_command('bootstrap')}"
            )

    def export_db_check() -> str:
        # Runs only after archive_check has found the database path.
        assert export_db is not None
        if not export_db.exists():
            return "not created yet; the first export creates it"
        schema = read_export_db_version(export_db)
        if not export_db_version_supported(schema):
            raise ActionRequired(unsupported_export_db_reason(schema))
        return f"schema version {schema}"

    if check("Archive access", archive_check):
        check("Export database", export_db_check)
    check(
        "Photos library directory",
        lambda: check_copy_path(apple.library, workflow="Photos library", source=True),
    )


def _check_remote(
    remote: RcloneConfig,
    config_path: Path | None,
    check: Check,
    report: Report,
    *,
    rclone_installed: bool,
) -> None:
    def remote_source_check() -> str:
        source = resolve_rclone_source(remote, config_path)
        check_copy_path(source, workflow="Remote", source=True)
        return str(source)

    check("Remote source", remote_source_check)
    name = _rclone_remote_name(remote.remote)
    if name is None:
        report("SKIP", "rclone remote", f"'{remote.remote}' names no configured remote")
    elif not rclone_installed:
        report("SKIP", "rclone remote", "needs rclone above")
    else:
        check("rclone remote", lambda: _check_rclone_remote(name))
    report(
        "SKIP", "Cloud connectivity and credentials", "no remote connection attempted"
    )


def _check_local_copies(
    sd: SdCardConfig | None, ssd: SsdConfig | None, check: Check, report: Report
) -> None:
    configs: tuple[tuple[str, SdCardConfig | SsdConfig | None], ...] = (
        ("SD Card", sd),
        ("SSD", ssd),
    )
    for name, config in configs:
        if config is None:
            continue
        sources: tuple[Path, ...] = (config.source,)
        if name == "SSD" and sd:
            sources += (sd.destination,)
        paths_ok = True
        for source in sources:
            label = f"{name} source {source}"
            if (
                name == "SSD"
                and sd
                and source == sd.destination
                and not source.exists()
            ):
                # The SD card copy creates this directory on its first run.
                paths_ok = False
                report(
                    "SKIP",
                    label,
                    "not created yet; run "
                    f"`{suggested_command('sd-card')}` before copying it to the SSD",
                )
                continue
            paths_ok &= check(
                label,
                partial(check_copy_path, source, workflow=name, source=True),
            )
        paths_ok &= check(
            f"{name} destination {config.destination}",
            lambda: _check_destination(config.destination, workflow=name),
        )
        if paths_ok:
            check(
                f"{name} copy layout",
                lambda: check_copy_paths(sources, config.destination, workflow=name),
            )
        else:
            report("SKIP", f"{name} copy layout", "needs the paths above")
        if name == "SD Card" and paths_ok:
            check(
                "SD Card camera folders", lambda: _check_camera_folders(config.source)
            )
        if config.exclude_file is not None:
            check(f"{name} exclusions", partial(_check_exclusion, config.exclude_file))


def _check_destination(path: Path, *, workflow: str) -> str:
    check_copy_path(path, workflow=workflow)
    return "" if path.exists() else "created on the first copy"


def _print_results(
    versions: dict[str, str | None],
    results: list[tuple[str, str, str]],
    exit_code: int,
    *,
    as_json: bool,
) -> None:
    if as_json:
        document = {
            "versions": versions,
            "checks": [
                {"status": status, "check": label, "detail": detail}
                for status, label, detail in results
            ],
            "exit_code": exit_code,
        }
        click.echo(json.dumps(document, indent=2))
        return
    click.echo(f"Python: {versions['python']}")
    for package, package_version in versions.items():
        if package != "python" and package_version is not None:
            click.echo(f"{package}: {package_version}")
    if not print_terminal_table(
        "Setup check",
        ("Status", "Check", "Detail"),
        results,
        styles=[STATUS_STYLES[status] for status, _, _ in results],
    ):
        for status, label, detail in results:
            click.echo(f"{status} {label}" + (f": {detail}" if detail else ""))
    click.echo(
        "Setup diagnostics only; media contents and destination copies were not "
        "verified."
    )


def _check_camera_folders(source: Path) -> str:
    if folders := uncovered_camera_folders(source):
        raise ActionRequired(uncovered_camera_folders_message(source, folders))
    return ""


def _check_exclusion(path: Path) -> str:
    if not path.is_file():
        raise ActionRequired(
            f"'{path}' is missing or not a regular file; copies omit these exclusions "
            "and SSD mirror deletions are blocked"
        )
    return str(path)


def _rclone_remote_name(remote: str) -> str | None:
    """Return the config-file remote a destination like `b2:photos` uses.

    On-the-fly `:backend:` remotes need no rclone.conf entry.
    """
    name, _, _ = remote.partition(":")
    if not name:
        return None
    # Connection-string overrides follow a comma: `b2,hard_delete=true:photos`.
    return name.split(",", 1)[0]


def _check_rclone_remote(name: str) -> str:
    # Reads the local rclone.conf only; never prompt for its password.
    try:
        _, output = _run_tool("rclone", ["listremotes", "--ask-password=false"])
    except click.ClickException as error:
        raise ActionRequired(
            f"{error.message}; if the rclone configuration is encrypted, "
            "set RCLONE_CONFIG_PASS"
        ) from error
    remotes = {line.strip().rstrip(":") for line in output.splitlines()}
    if name not in remotes:
        raise ActionRequired(
            f"'{name}:' is not in the rclone configuration; add it with "
            "`rclone config` or fix [rclone] remote"
        )
    return f"'{name}:' is configured; credentials were not checked"


def _tool_version(executable: str) -> str:
    path, output = _run_tool(
        executable, ["-ver" if executable == "exiftool" else "--version"]
    )
    lines = output.strip().splitlines()
    detail = f"{path} — {lines[0] if lines else 'version not reported'}"
    if executable == "rsync" and lines and lines[0].startswith("openrsync"):
        # Copies work with it, but people often expect Homebrew's GNU rsync.
        detail += " (macOS built-in openrsync, not GNU rsync, is first on PATH)"
    return detail


def _run_tool(executable: str, arguments: list[str]) -> tuple[str, str]:
    path = shutil.which(executable)
    if path is None:
        raise click.ClickException("not found on PATH; install it before backing up")
    try:
        result = subprocess.run(
            [path, *arguments],
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            errors="replace",
            timeout=5,
            check=True,
        )
    except (OSError, subprocess.SubprocessError) as error:
        raise click.ClickException(
            f"could not run {path} {' '.join(arguments)}: {error}"
        ) from error
    return path, result.stdout
