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
from photos_backup.config import DEFAULT_CONFIG_PATH


@click.group()
@click.option(
    "--config",
    "config_path",
    type=click.Path(dir_okay=False, path_type=Path),
    default=None,
    help=f"TOML configuration file (default: {DEFAULT_CONFIG_PATH}).",
)
@click.pass_context
def cli(ctx: click.Context, config_path: Path | None) -> None:
    """Pass `--help` to any command to see its usage."""
    ctx.obj = CliContext(config_path=config_path)


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
