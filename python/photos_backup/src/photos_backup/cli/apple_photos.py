import click

from photos_backup.apple_photos.export import (
    ApplePhotosExport,
    validate_export_overrides,
)
from photos_backup.apple_photos.identity import WriterStatus
from photos_backup.apple_photos.takeover import ensure_writer
from photos_backup.archive import open_archive
from photos_backup.cli.context import apple_photos_config_from
from photos_backup.summary import print_export_result, print_takeover_check


@click.command(
    name="apple-photos",
    help="Export Apple Photos manually. Extra flags are forwarded to osxphotos.",
    context_settings={"ignore_unknown_options": True, "allow_extra_args": True},
)
@click.option(
    "--testing",
    is_flag=True,
    help="Run a limited, verbose osxphotos simulation without writing assets.",
)
@click.option(
    "--dry-run", is_flag=True, help="Show the plan without invoking osxphotos."
)
@click.pass_context
def apple_photos(ctx: click.Context, testing: bool, dry_run: bool) -> None:
    extra_arguments = _parse_extra_args(ctx.args)
    validate_export_overrides(extra_arguments)
    config = apple_photos_config_from(ctx)
    readonly = testing or dry_run
    with open_archive(config, dry_run=readonly) as archive:
        takeover = ensure_writer(config, archive)
        if takeover.status is not WriterStatus.UNCHANGED:
            print_takeover_check(takeover, dry_run=readonly)
        result = ApplePhotosExport(
            config=config,
            archive=archive,
            verbose=testing,
            limit=config.limit_export if testing else 0,
            extra_arguments=extra_arguments,
            plan_only=dry_run,
        ).export()
    print_export_result(result, dry_run=readonly)
    if extra_arguments and not readonly:
        click.echo("Custom export: daily/full backup timestamps were not advanced.")
    if not result.complete:
        raise click.ClickException(str(result.failure_reason()))


def _parse_extra_args(args: list[str]) -> dict:
    """Parse CLI-style args (e.g. --use-photokit --limit 10) into kwargs."""
    kwargs: dict = {}
    i = 0
    while i < len(args):
        arg = args[i]
        if not arg.startswith("--"):
            raise click.BadParameter(f"Unexpected argument: {arg}")
        option, separator, inline_value = arg.partition("=")
        key = option.lstrip("-").replace("-", "_")
        # Check if next arg is a value (not another flag)
        if separator or (i + 1 < len(args) and not args[i + 1].startswith("--")):
            value = inline_value if separator else args[i + 1]
            try:
                value = int(value)
            except ValueError:
                try:
                    value = float(value)
                except ValueError:
                    pass
            kwargs[key] = value
            i += 1 if separator else 2
        else:
            kwargs[key] = True
            i += 1
    return kwargs
