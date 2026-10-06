"""Read-only setup diagnostics; never start an export or transfer."""

import platform
import shutil
import subprocess
from importlib.metadata import PackageNotFoundError, version

import click

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
from photos_backup.errors import ActionRequired


@click.command(
    help="Check configuration, local paths, and tools without running backups."
)
@click.pass_context
def doctor(ctx: click.Context) -> None:
    failures: list[int] = []

    def check(label, operation):
        try:
            detail = operation()
        except (click.ClickException, OSError) as error:
            failures.append(
                error.exit_code if isinstance(error, click.ClickException) else 1
            )
            click.echo(f"FAIL {label}: {error}")
            return
        click.echo(f"PASS {label}" + (f": {detail}" if detail else ""))

    click.echo(f"Python: {platform.python_version()}")
    for package in ("photos_backup", "click", "osxphotos"):
        try:
            click.echo(f"{package}: {version(package)}")
        except PackageNotFoundError:
            failures.append(1)
            click.echo(f"FAIL {package}: package metadata unavailable")

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
            click.echo(f"PASS [{name}] configuration")
        except MissingSection:
            click.echo(f"SKIP [{name}]: not configured")
        except click.ClickException as error:
            failures.append(error.exit_code)
            click.echo(f"FAIL [{name}] configuration: {error}")

    apple, sd, ssd, remote = (
        configs.get(key) for key in ("apple_photos", "sd_card", "ssd", "rclone")
    )
    required = []
    if apple:
        required.append("exiftool")

        def archive_check():
            with open_archive(apple, dry_run=True) as archive:
                state = archive.state_store.load()
                return (
                    "initialized"
                    if state.initialized
                    else f"not initialized; run {suggested_command('bootstrap')}"
                )

        check("Archive access", archive_check)
        check(
            "Photos library directory",
            lambda: check_copy_path(
                apple.library, workflow="Photos library", source=True
            ),
        )
    if sd or ssd:
        required.append("rsync")
    if remote:
        required.append("rclone")
    for executable in required:
        check(executable, lambda executable=executable: _tool_version(executable))
    _check_local_copies(sd, ssd, check)
    if remote:

        def remote_source_check():
            source = resolve_rclone_source(remote, path)
            check_copy_path(source, workflow="Remote", source=True)
            return str(source)

        check("Remote source", remote_source_check)
        click.echo(
            "SKIP Cloud connectivity and credentials: no remote connection attempted"
        )
    click.echo(
        "Setup diagnostics only; media contents and destination copies were not verified."
    )
    if failures:
        ctx.exit(next(code for code in (2, 1, 3) if code in failures))


def _check_local_copies(sd, ssd, check):
    for name, config in (("SD Card", sd), ("SSD", ssd)):
        if config is None:
            continue
        sources = (config.source,)
        if name == "SSD" and sd:
            sources += (sd.destination,)
        for source in sources:
            check(
                f"{name} source {source}",
                lambda source=source: check_copy_path(
                    source, workflow=name, source=True
                ),
            )
        check(
            f"{name} destination {config.destination}",
            lambda: check_copy_path(config.destination, workflow=name),
        )
        check(
            f"{name} copy layout",
            lambda: check_copy_paths(sources, config.destination, workflow=name),
        )
        if config.exclude_file is not None:
            check(f"{name} exclusions", lambda: _check_exclusion(config.exclude_file))


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
