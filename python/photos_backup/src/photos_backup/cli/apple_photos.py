import importlib
from collections.abc import Iterator
from contextlib import contextmanager
from typing import Any

import rich_click as click

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
    help="Export Apple Photos manually. Extra flags are forwarded to osxphotos export.",
    epilog="Example: photos-backup --volume /Volumes/T7 apple-photos "
    "--album Holiday --limit 50",
    options_metavar="[OPTIONS] [OSXPHOTOS EXPORT OPTIONS]...",
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


def _parse_extra_args(args: list[str]) -> dict[str, Any]:
    """Parse forwarded flags with osxphotos's own export options.

    The result holds only the options given, typed as `export_cli` expects:
    repeated options become tuples, dates become datetimes, and `--album 2024`
    stays a string. Archive-managed spellings are refused before parsing so a
    flag osxphotos does not define still gets the archive explanation.
    """
    validate_export_overrides(
        {
            arg[2:].partition("=")[0].replace("-", "_"): None
            for arg in args
            if arg.startswith("--")
        }
    )
    command = importlib.import_module("osxphotos.cli.export").export
    parse_context = click.Context(command, info_name="osxphotos export")
    with _osxphotos_usage():
        values, _, order = command.make_parser(parse_context).parse_args(list(args))
    # Skip osxphotos's own --help; it is never an export_cli argument.
    given = [param for param in dict.fromkeys(order) if param in command.params]
    for param in given:
        if not isinstance(param, click.Option) and isinstance(values[param.name], str):
            # osxphotos's only positional is the archive-managed DEST.
            raise click.UsageError(f"Unexpected argument: {values[param.name]}")
    options = [param for param in given if isinstance(param, click.Option)]
    # Aliases such as --library resolve to archive-managed names only now.
    validate_export_overrides({param.name: None for param in options})
    with _osxphotos_usage():
        return {
            param.name: param.type_cast_value(parse_context, values[param.name])
            for param in options
        }


@contextmanager
def _osxphotos_usage() -> Iterator[None]:
    try:
        yield
    except click.UsageError as error:
        raise click.UsageError(f"osxphotos export: {error.format_message()}") from error
