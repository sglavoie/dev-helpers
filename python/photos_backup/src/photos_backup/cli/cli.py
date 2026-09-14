from pathlib import Path

import click

from photos_backup.cli.apple_photos import apple_photos
from photos_backup.cli.approve_cleanup import approve_cleanup
from photos_backup.cli.backup_all import backup_all
from photos_backup.cli.bootstrap import bootstrap
from photos_backup.cli.cleanup_local_export import cleanup_local_export
from photos_backup.cli.daily import daily
from photos_backup.cli.remote import remote
from photos_backup.cli.sd_card import sd_card
from photos_backup.cli.context import CliContext
from photos_backup.cli.ssd import ssd
from photos_backup.cli.verify import verify
from photos_backup.config import DEFAULT_CONFIG_PATH, normalize_volume_override


def _volume_override(
    ctx: click.Context, param: click.Parameter, value: str | None
) -> Path | None:
    return normalize_volume_override(value) if value else None


@click.group()
@click.option(
    "--config",
    "config_path",
    type=click.Path(dir_okay=False, path_type=Path),
    default=None,
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
    """Pass `--help` to any command to see its usage."""
    ctx.obj = CliContext(config_path=config_path, volume=volume)


cli.add_command(apple_photos)
cli.add_command(approve_cleanup)
cli.add_command(backup_all)
cli.add_command(bootstrap)
cli.add_command(cleanup_local_export)
cli.add_command(daily)
cli.add_command(remote)
cli.add_command(sd_card)
cli.add_command(ssd)
cli.add_command(verify)


if __name__ == "__main__":
    cli()
