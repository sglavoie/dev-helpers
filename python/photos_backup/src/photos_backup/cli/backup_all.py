from __future__ import annotations

from collections.abc import Callable
from contextlib import ExitStack
from pathlib import Path
from shutil import which
from typing import TypeVar

import rich_click as click

from photos_backup.apple_photos.downloads import DEFAULT_DOWNLOAD_TIMEOUT
from photos_backup.cli.context import (
    apple_photos_config_from,
    config_path_from,
    suggested_command,
)
from photos_backup.cli.exporting import (
    command_timer,
    export_into_archive,
    print_export_outcome,
)
from photos_backup.cli.outcome import raise_for_summaries
from photos_backup.config import (
    ApplePhotosConfig,
    MissingSection,
    RcloneConfig,
    SdCardConfig,
    SsdConfig,
    load_optional,
    load_rclone_config,
    load_sd_card_config,
    load_ssd_config,
    resolve_rclone_source,
)
from photos_backup.remote.backup import Backup as RemoteBackup
from photos_backup.errors import ActionRequired
from photos_backup.progress import ExportProgress
from photos_backup.transfers import TransferHistory
from photos_backup.sd_card.backup import Backup as SdCardBackup
from photos_backup.ssd.backup import Backup as SsdBackup
from photos_backup.summary import (
    BackupSummary,
    print_pipeline_destinations,
    print_pipeline_summary,
)

T = TypeVar("T")


@click.command(
    name="backup-all",
    help="Export Apple Photos and copy SD card, SSD, and remote backups. "
    "Archive cleanup is not reconciled here; run daily for cleanup reconciliation.",
)
@click.option(
    "--dry-run",
    is_flag=True,
    help="Plan the export and preview every copy without writing anything.",
)
@click.option(
    "--download-timeout",
    type=click.IntRange(min=1),
    default=DEFAULT_DOWNLOAD_TIMEOUT,
    show_default=True,
    help="Seconds allowed for missing-file retrieval per asset, across retries. No total run limit.",
)
@click.option(
    "--delete-remote", is_flag=True, help="Delete remote files absent from the source."
)
@click.option(
    "--delete-ssd",
    "delete",
    is_flag=True,
    help="Delete SSD files absent from their source.",
)
# The older spelling keeps working without competing with --delete-ssd in help.
@click.option("--delete", "delete_alias", is_flag=True, hidden=True)
@click.option("--skip-apple-photos", is_flag=True, help="Skip Apple Photos export.")
@click.option("--skip-sd-card", is_flag=True, help="Skip SD card backup.")
@click.option("--skip-ssd", is_flag=True, help="Skip SSD backup.")
@click.option("--skip-remote", is_flag=True, help="Skip remote backup.")
@click.pass_context
def backup_all(
    ctx: click.Context,
    dry_run: bool,
    download_timeout: int,
    delete: bool,
    delete_alias: bool,
    delete_remote: bool,
    skip_apple_photos: bool,
    skip_sd_card: bool,
    skip_ssd: bool,
    skip_remote: bool,
) -> None:
    with command_timer():
        _backup_all(
            ctx,
            dry_run=dry_run,
            download_timeout=download_timeout,
            delete=delete or delete_alias,
            delete_remote=delete_remote,
            skip_apple_photos=skip_apple_photos,
            skip_sd_card=skip_sd_card,
            skip_ssd=skip_ssd,
            skip_remote=skip_remote,
        )


def _backup_all(
    ctx: click.Context,
    *,
    dry_run: bool,
    download_timeout: int,
    delete: bool,
    delete_remote: bool,
    skip_apple_photos: bool,
    skip_sd_card: bool,
    skip_ssd: bool,
    skip_remote: bool,
) -> None:
    # A --volume override re-points the Apple Photos step only; the SSD and
    # remote steps keep reading their own configured sources.
    config_path = config_path_from(ctx)
    # Resolve enabled configurations and their dependencies before any effects.
    try:
        config = None if skip_apple_photos else apple_photos_config_from(ctx)
    except MissingSection as error:
        raise MissingSection(
            f"{error}. For a copy-only run, use "
            f"`{suggested_command('backup-all', '--skip-apple-photos')}`."
        ) from error
    ssd_config = _load_optional(skip_ssd, lambda: load_ssd_config(config_path))
    sd_config = _load_optional(
        skip_sd_card and ssd_config is None, lambda: load_sd_card_config(config_path)
    )
    remote_config = _load_optional(skip_remote, lambda: load_rclone_config(config_path))
    remote_source = (
        resolve_rclone_source(remote_config, config_path)
        if remote_config is not None
        else None
    )
    remote: tuple[RcloneConfig, Path] | None = (
        (remote_config, remote_source)
        if remote_config is not None and remote_source is not None
        else None
    )
    _check_executables(
        apple_photos=config is not None and not dry_run,
        local_copy=ssd_config is not None
        or (sd_config is not None and not skip_sd_card),
        remote=remote_config is not None,
    )
    print_pipeline_destinations(
        _pipeline_destinations(
            config,
            sd_config,
            ssd_config,
            remote_config,
            remote_source,
            skip_sd_card=skip_sd_card,
            delete=delete,
            delete_remote=delete_remote,
        ),
        dry_run=dry_run,
    )
    summaries: list[BackupSummary] = []

    if skip_apple_photos:
        summaries.append(
            BackupSummary(
                step_name="Apple Photos", skipped=True, skip_reason="Skipped by request"
            )
        )
    else:
        try:
            assert config is not None
            summaries.append(
                _export_apple_photos(
                    config, dry_run=dry_run, download_timeout=download_timeout
                )
            )
        except ActionRequired as error:
            summaries.append(
                BackupSummary(
                    step_name="Apple Photos", error=str(error), action_required=True
                )
            )
        except Exception as error:
            summaries.append(BackupSummary(step_name="Apple Photos", error=str(error)))

    summaries.extend(
        _optional_step(
            "SD Card",
            sd_config,
            lambda config: [
                SdCardBackup(
                    config=config, dry_run=dry_run, history=TransferHistory(config_path)
                ).backup()
            ],
            skip=skip_sd_card,
            section="sd_card",
        )
    )
    ssd_summaries = _optional_step(
        "SSD",
        ssd_config,
        lambda config: SsdBackup(
            config=config,
            delete_at_destination=delete,
            dry_run=dry_run,
            sd_card=sd_config,
            history=TransferHistory(config_path),
        ).backup(),
        skip=skip_ssd,
        section="ssd",
    )
    summaries.extend(ssd_summaries)
    remote_skip_reason = None
    if (
        not skip_remote
        and remote_source is not None
        and ssd_config is not None
        and any(summary.error for summary in ssd_summaries)
    ):
        try:
            source = remote_source.resolve()
            destination = ssd_config.destination.resolve()
            if source.is_relative_to(destination) or destination.is_relative_to(source):
                remote_skip_reason = (
                    "SSD copy did not complete; remote source overlaps its destination"
                )
        except (OSError, RuntimeError) as error:
            remote_skip_reason = (
                "SSD copy did not complete; could not check remote source independence: "
                f"{error}"
            )
    if remote_skip_reason:
        summaries.append(
            BackupSummary(
                step_name="Remote",
                skipped=True,
                skip_reason=remote_skip_reason,
            )
        )
    else:
        summaries.extend(
            _optional_step(
                "Remote",
                remote,
                lambda route: [
                    RemoteBackup(
                        config=route[0],
                        source=route[1],
                        dry_run=dry_run,
                        delete_at_destination=delete_remote,
                        history=TransferHistory(config_path),
                    ).backup()
                ],
                skip=skip_remote,
                section="rclone",
            )
        )

    print_pipeline_summary(summaries)
    raise_for_summaries(summaries, name_failed_steps=True)


def _pipeline_destinations(
    config: ApplePhotosConfig | None,
    sd_config: SdCardConfig | None,
    ssd_config: SsdConfig | None,
    remote_config: RcloneConfig | None,
    remote_source: Path | None,
    *,
    skip_sd_card: bool,
    delete: bool,
    delete_remote: bool,
) -> list[tuple[str, Path | str, Path | str, bool]]:
    destinations: list[tuple[str, Path | str, Path | str, bool]] = []
    if config is not None:
        destinations.append(("Apple Photos", config.library, config.archive, False))
    if sd_config is not None and not skip_sd_card:
        destinations.append(
            (
                "SD Card",
                sd_config.source,
                sd_config.destination / sd_config.source.name,
                False,
            )
        )
    if ssd_config is not None:
        destinations.append(
            (
                "SSD: All Photos",
                ssd_config.source,
                ssd_config.destination / ssd_config.source.name,
                delete,
            )
        )
        if sd_config is not None:
            destinations.append(
                (
                    "SSD: SD Card",
                    sd_config.destination,
                    ssd_config.destination / sd_config.destination.name,
                    delete,
                )
            )
    if remote_config is not None and remote_source is not None:
        destinations.append(
            ("Remote", remote_source, remote_config.remote, delete_remote)
        )
    return destinations


def _check_executables(*, apple_photos: bool, local_copy: bool, remote: bool) -> None:
    required = [
        executable
        for executable, enabled in (
            ("exiftool", apple_photos),
            ("rsync", local_copy),
            ("rclone", remote),
        )
        if enabled
    ]
    missing = [executable for executable in required if which(executable) is None]
    if missing:
        raise click.ClickException(
            f"Required executable(s) not found on PATH: {', '.join(missing)}. "
            "Install them and ensure they are on PATH before rerunning backup-all. "
            "No backup steps were started."
        )


def _export_apple_photos(
    config: ApplePhotosConfig, *, dry_run: bool, download_timeout: int
) -> BackupSummary:
    with ExportProgress() as progress, ExitStack() as stack:
        _, takeover, result = export_into_archive(
            stack,
            progress,
            config,
            dry_run=dry_run,
            download_timeout=download_timeout,
        )
    # After the live status line has finished, so this starts on its own line.
    print_export_outcome(takeover, result, dry_run=dry_run)
    return result.summary()


def _load_optional(skip: bool, load: Callable[[], T]) -> T | None:
    return None if skip else load_optional(load)


def _optional_step(
    step_name: str,
    config: T | None,
    run: Callable[[T], list[BackupSummary]],
    *,
    skip: bool,
    section: str,
) -> list[BackupSummary]:
    """Run a prevalidated workflow, or report its absence as skipped."""
    if skip or config is None:
        return [
            BackupSummary(
                step_name=step_name,
                skipped=True,
                skip_reason="Skipped by request"
                if skip
                else f"Not configured: [{section}]",
            )
        ]
    try:
        return run(config)
    except ActionRequired as error:
        return [
            BackupSummary(step_name=step_name, error=str(error), action_required=True)
        ]
    except Exception as error:
        return [BackupSummary(step_name=step_name, error=str(error))]
