import click

from photos_backup.apple_photos.export import ApplePhotosExport
from photos_backup.archive import open_archive
from photos_backup.cli.context import config_path_from
from photos_backup.config import load_apple_photos_config
from photos_backup.summary import print_export_result


@click.command(
    name="apple-photos",
    help="Export Apple Photos manually. Extra flags are forwarded to osxphotos.",
    context_settings={"ignore_unknown_options": True, "allow_extra_args": True},
)
@click.option(
    "--testing",
    is_flag=True,
    help="Set useful flags when testing, along with --dry-run.",
)
@click.pass_context
def apple_photos(ctx: click.Context, testing: bool) -> None:
    config = load_apple_photos_config(config_path_from(ctx))
    extra_arguments = _parse_extra_args(ctx.args)
    with open_archive(config, dry_run=testing) as archive:
        result = ApplePhotosExport(
            config=config,
            archive=archive,
            verbose=testing,
            limit=config.limit_export if testing else 0,
            extra_arguments=extra_arguments,
        ).export()
    print_export_result(result, dry_run=testing)
    if not result.clean:
        raise click.ClickException(str(result.failure_reason()))


def _parse_extra_args(args: list[str]) -> dict:
    """Parse CLI-style args (e.g. --use-photokit --limit 10) into kwargs."""
    kwargs: dict = {}
    i = 0
    while i < len(args):
        arg = args[i]
        if not arg.startswith("--"):
            raise click.BadParameter(f"Unexpected argument: {arg}")
        key = arg.lstrip("-").replace("-", "_")
        # Check if next arg is a value (not another flag)
        if i + 1 < len(args) and not args[i + 1].startswith("--"):
            value = args[i + 1]
            try:
                value = int(value)
            except ValueError:
                try:
                    value = float(value)
                except ValueError:
                    pass
            kwargs[key] = value
            i += 2
        else:
            kwargs[key] = True
            i += 1
    return kwargs
