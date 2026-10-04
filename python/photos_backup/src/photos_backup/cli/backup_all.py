from __future__ import annotations

from collections.abc import Callable
from typing import TypeVar

import click

from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.takeover import ensure_writer
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from, config_path_from
from photos_backup.cli.ssd import load_optional_sd_card_config
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
    summaries: list[BackupSummary] = []
    action_required: str | None = None

    if skip_apple_photos:
        summaries.append(BackupSummary(step_name="Apple Photos", skipped=True))
    else:
        try:
            config = apple_photos_config_from(ctx)
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
            action_required = str(error)
            summaries.append(BackupSummary(step_name="Apple Photos", error=str(error)))
        except Exception as error:
            summaries.append(BackupSummary(step_name="Apple Photos", error=str(error)))

    summaries.extend(
        _optional_step(
            "SD Card",
            skip_sd_card,
            lambda: load_sd_card_config(config_path),
            lambda config: [SdCardBackup(config=config, dry_run=dry_run).backup()],
        )
    )
    summaries.extend(
        _optional_step(
            "SSD",
            skip_ssd,
            lambda: load_ssd_config(config_path),
            lambda config: SsdBackup(
                config=config,
                delete_at_destination=delete,
                dry_run=dry_run,
                sd_card=load_optional_sd_card_config(config_path),
            ).backup(),
        )
    )
    summaries.extend(
        _optional_step(
            "Remote",
            skip_remote,
            lambda: load_rclone_config(config_path),
            lambda config: [
                RemoteBackup(
                    config=config,
                    source=resolve_rclone_source(config, config_path),
                    dry_run=dry_run,
                ).backup()
            ],
        )
    )

    print_pipeline_summary(summaries)
    failed = [
        summary.step_name
        for summary in summaries
        if summary.error
        and not (action_required and summary.step_name == "Apple Photos")
    ]
    if failed:
        raise click.ClickException(f"Step(s) failed: {', '.join(failed)}")
    if action_required:
        raise ActionRequired(action_required)


def _optional_step(
    step_name: str,
    skip: bool,
    load: Callable[[], T],
    run: Callable[[T], list[BackupSummary]],
) -> list[BackupSummary]:
    """Run a workflow, skipping it when its configuration section is absent."""
    if skip:
        return [BackupSummary(step_name=step_name, skipped=True)]
    try:
        config = load()
    except MissingSection:
        return [BackupSummary(step_name=step_name, skipped=True)]
    try:
        return run(config)
    except Exception as error:
        return [BackupSummary(step_name=step_name, error=str(error))]
