"""Read-only setup diagnostics; never start an export or transfer."""

import platform
import shutil
import subprocess
from importlib.metadata import PackageNotFoundError, version

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
    MissingSection,
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

STATUS_STYLES = {
    "PASS": "green",
    "SKIP": "dim",
    "ACTION": "bold yellow",
    "FAIL": "bold red",
}


@click.command(
    help="Check configuration, local paths, and tools without running backups."
)
@click.pass_context
def doctor(ctx: click.Context) -> None:
    failures: list[int] = []
    results: list[tuple[str, str, str]] = []

    def report(status: str, label: str, detail: str = "") -> None:
        results.append((status, label, detail))

    def check(label, operation) -> bool:
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

    click.echo(f"Python: {platform.python_version()}")
    for package in ("photos_backup", "click", "osxphotos"):
        try:
            click.echo(f"{package}: {version(package)}")
        except PackageNotFoundError:
            failures.append(1)
            report("FAIL", package, "package metadata unavailable")

    path = config_path_from(ctx)
    configs = {}
    for name, loader in (
        ("apple_photos", lambda: apple_photos_config_from(ctx)),
        ("sd_card", lambda: load_sd_card_config(path)),
        ("ssd", lambda: load_ssd_config(path)),
        ("rclone", lambda: load_rclone_config(path)),
    ):
        try:
            configs[name] = loader()
            report("PASS", f"[{name}] configuration")
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
    for executable in required:
        check(executable, lambda executable=executable: _tool_version(executable))
    _check_local_copies(sd, ssd, check, report)
    if remote:
        _check_remote(remote, path, check, report)
    _print_results(results)
    click.echo(
        "Setup diagnostics only; media contents and destination copies were not verified."
    )
    if failures:
        ctx.exit(next(code for code in (2, 1, 3) if code in failures))


def _check_apple_photos(apple, check):
    export_db = None

    def archive_check():
        nonlocal export_db
        with open_archive(apple, dry_run=True) as archive:
            export_db = archive.paths.export_db
            state = archive.state_store.load()
            return (
                "initialized"
                if state.initialized
                else f"not initialized; run {suggested_command('bootstrap')}"
            )

    def export_db_check():
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


def _check_remote(remote, config_path, check, report):
    def remote_source_check():
        source = resolve_rclone_source(remote, config_path)
        check_copy_path(source, workflow="Remote", source=True)
        return str(source)

    check("Remote source", remote_source_check)
    report(
        "SKIP", "Cloud connectivity and credentials", "no remote connection attempted"
    )


def _check_local_copies(sd, ssd, check, report):
    for name, config in (("SD Card", sd), ("SSD", ssd)):
        if config is None:
            continue
        sources = (config.source,)
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
                lambda source=source: check_copy_path(
                    source, workflow=name, source=True
                ),
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
            check(f"{name} exclusions", lambda: _check_exclusion(config.exclude_file))


def _check_destination(path, *, workflow):
    check_copy_path(path, workflow=workflow)
    return "" if path.exists() else "created on the first copy"


def _print_results(results: list[tuple[str, str, str]]) -> None:
    if print_terminal_table(
        "Setup check",
        ("Status", "Check", "Detail"),
        results,
        styles=[STATUS_STYLES[status] for status, _, _ in results],
    ):
        return
    for status, label, detail in results:
        click.echo(f"{status} {label}" + (f": {detail}" if detail else ""))


def _check_camera_folders(source):
    if folders := uncovered_camera_folders(source):
        raise ActionRequired(uncovered_camera_folders_message(source, folders))
    return ""


def _check_exclusion(path):
    if not path.is_file():
        raise ActionRequired(
            f"'{path}' is missing or not a regular file; copies omit these exclusions "
            "and SSD mirror deletions are blocked"
        )
    return str(path)


def _tool_version(executable: str) -> str:
    path = shutil.which(executable)
    if path is None:
        raise click.ClickException("not found on PATH; install it before backing up")
    try:
        result = subprocess.run(
            [path, "-ver" if executable == "exiftool" else "--version"],
            capture_output=True,
            text=True,
            errors="replace",
            timeout=5,
            check=True,
        )
    except (OSError, subprocess.SubprocessError) as error:
        raise click.ClickException(
            f"could not read version from {path}: {error}"
        ) from error
    lines = result.stdout.strip().splitlines()
    return f"{path} — {lines[0] if lines else 'version not reported'}"
