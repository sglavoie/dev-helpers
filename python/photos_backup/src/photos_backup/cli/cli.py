import signal
import threading
from pathlib import Path
from types import FrameType
from typing import NoReturn

import rich_click as click

from photos_backup.cli.apple_photos import apple_photos
from photos_backup.cli.approve_cleanup import approve_cleanup
from photos_backup.cli.backup_all import backup_all
from photos_backup.cli.bootstrap import bootstrap
from photos_backup.cli.cleanup_local_export import cleanup_local_export
from photos_backup.cli.daily import daily
from photos_backup.cli.doctor import doctor
from photos_backup.cli.remote import remote
from photos_backup.cli.recent import recent
from photos_backup.cli.sd_card import sd_card
from photos_backup.cli.context import CliContext
from photos_backup.cli.ssd import ssd
from photos_backup.cli.status import status
from photos_backup.cli.verify import verify
from photos_backup.config import DEFAULT_CONFIG_PATH, normalize_volume_override
from photos_backup.presentation import TerminalGroup


def _interrupt(signum: int, frame: FrameType | None) -> NoReturn:
    raise KeyboardInterrupt(signal.Signals(signum).name)


def install_termination_handlers() -> None:
    """Stop like Ctrl-C on SIGTERM or SIGHUP.

    The default action would end Python without cleanup, leaving an rsync or
    rclone child running unlocked and its transfer receipt stuck at started.
    """
    if threading.current_thread() is not threading.main_thread():
        return
    for signum in (signal.SIGTERM, signal.SIGHUP):
        signal.signal(signum, _interrupt)


def _volume_override(
    ctx: click.Context, param: click.Parameter, value: str | None
) -> Path | None:
    return normalize_volume_override(value) if value else None


@click.group(
    cls=TerminalGroup,
    context_settings={
        "rich_help_config": click.RichHelpConfiguration(
            text_markup="ansi",
            command_groups={
                "*": [
                    {
                        "name": "Inspect and verify",
                        "commands": ["status", "doctor", "verify"],
                    },
                    {
                        "name": "Back up photos",
                        "commands": [
                            "daily",
                            "backup-all",
                            "recent",
                            "bootstrap",
                            "apple-photos",
                        ],
                    },
                    {
                        "name": "Copy to destinations",
                        "commands": ["sd-card", "ssd", "remote"],
                    },
                    {
                        "name": "Review and clean up",
                        "commands": ["approve-cleanup", "cleanup-local-export"],
                    },
                ]
            },
        )
    },
    epilog="Start with 'photos-backup doctor' to check your setup. "
    "Pass --help to any command to see its options.",
)
@click.version_option(package_name="photos_backup", prog_name="photos-backup")
@click.option(
    "--config",
    "config_path",
    type=click.Path(dir_okay=False, path_type=Path),
    default=None,
    envvar="PHOTOS_BACKUP_CONFIG",
    show_envvar=True,
    help=f"TOML configuration file (default: {DEFAULT_CONFIG_PATH}).",
)
@click.option(
    "--volume",
    "volume",
    default=None,
    callback=_volume_override,
    help="Override [apple_photos] volume for this run; the archive sub-path is "
    "re-rooted under it. Accepts any existing absolute directory, not just a "
    "mount point.",
)
@click.pass_context
def cli(ctx: click.Context, config_path: Path | None, volume: Path | None) -> None:
    """Back up Apple Photos, SD cards, and archives to local and cloud storage."""
    install_termination_handlers()
    ctx.obj = CliContext(config_path=config_path, volume=volume)


cli.add_command(apple_photos)
cli.add_command(approve_cleanup)
cli.add_command(backup_all)
cli.add_command(bootstrap)
cli.add_command(cleanup_local_export)
cli.add_command(daily)
cli.add_command(doctor)
cli.add_command(remote)
cli.add_command(recent)
cli.add_command(sd_card)
cli.add_command(ssd)
cli.add_command(status)
cli.add_command(verify)


if __name__ == "__main__":
    cli()
