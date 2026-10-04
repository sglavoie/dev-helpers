from __future__ import annotations

from collections.abc import Callable
from typing import TypeVar

import click

from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.takeover import ensure_writer
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from, config_path_from
from photos_backup.config import (
    MissingSection,
    load_rclone_config,
    load_sd_card_config,
    load_ssd_config,
    resolve_rclone_source,
)
from photos_backup.remote.backup import Backup as RemoteBackup
from photos_backup.errors import ActionRequired
from photos_backup.sd_card.backup import Backup as SdCardBackup
from photos_backup.ssd.backup import Backup as SsdBackup
from photos_backup.summary import (
    BackupSummary,
    print_export_result,
    print_pipeline_summary,
    print_takeover_check,
)

T = TypeVar("T")


@click.command(name="backup-all", help="Run the full backup pipeline.")
@click.option("--dry-run", is_flag=True, help="Dry run for all steps.")
@click.option(
    "--delete",
    is_flag=True,
    help="Delete extra files on SSD destination.",
)
@click.option("--skip-apple-photos", is_flag=True, help="Skip Apple Photos export.")
@click.option("--skip-sd-card", is_flag=True, help="Skip SD card backup.")
@click.option("--skip-ssd", is_flag=True, help="Skip SSD backup.")
@click.option("--skip-remote", is_flag=True, help="Skip remote backup.")
@click.pass_context
def backup_all(
    ctx: click.Context,
    dry_run: bool,
    delete: bool,
    skip_apple_photos: bool,
    skip_sd_card: bool,
    skip_ssd: bool,
    skip_remote: bool,
) -> None:
    # A --volume override re-points the Apple Photos step only; the SSD and
    # remote steps keep reading their own configured sources.
    config_path = config_path_from(ctx)
    # Resolve enabled configurations and their dependencies before any effects.
    config = None if skip_apple_photos else apple_photos_config_from(ctx)
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
            with open_archive(config, dry_run=dry_run) as archive:
                takeover = ensure_writer(config, archive)
                if takeover.status is not WriterStatus.UNCHANGED:
                    print_takeover_check(takeover, dry_run=dry_run)
                result = ApplePhotosExport(
                    config=config,
                    archive=archive,
                    verbose=dry_run,
                    limit=config.limit_export if dry_run else 0,
                    plan_only=dry_run,
                ).export()
                print_export_result(result, dry_run=dry_run)
                summaries.append(result.summary())
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
            lambda config: [SdCardBackup(config=config, dry_run=dry_run).backup()],
            skip=skip_sd_card,
            section="sd_card",
        )
    )
    summaries.extend(
        _optional_step(
            "SSD",
            ssd_config,
            lambda config: SsdBackup(
                config=config,
                delete_at_destination=delete,
                dry_run=dry_run,
                sd_card=sd_config,
            ).backup(),
            skip=skip_ssd,
            section="ssd",
        )
    )
    summaries.extend(
        _optional_step(
            "Remote",
            remote_config,
            lambda config: [
                RemoteBackup(
                    config=config,
                    source=remote_source,
                    dry_run=dry_run,
                ).backup()
            ],
            skip=skip_remote,
            section="rclone",
        )
    )

    print_pipeline_summary(summaries)
    failed = [
        summary.step_name
        for summary in summaries
        if summary.error and not summary.action_required
    ]
    if failed:
        raise click.ClickException(f"Step(s) failed: {', '.join(failed)}")
    actions = [
        summary.error
        for summary in summaries
        if summary.action_required and summary.error
    ]
    if actions:
        raise ActionRequired("; ".join(actions))


def _load_optional(skip: bool, load: Callable[[], T]) -> T | None:
    if skip:
        return None
    try:
        return load()
    except MissingSection:
        return None


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
