import click

from photos_backup.apple_photos.export import ApplePhotosExport


@click.command(
    name="apple-photos",
    help="Work with Apple Photos. Extra flags are forwarded to osxphotos export.",
    context_settings={"ignore_unknown_options": True, "allow_extra_args": True},
)
@click.option(
    "--testing",
    is_flag=True,
    help="Set useful flags when testing, along with --dry-run.",
)
@click.pass_context
def apple_photos(ctx: click.Context, testing: bool) -> None:
    extra_kwargs = _parse_extra_args(ctx.args)
    ApplePhotosExport(
        testing=testing,
        extra_kwargs=extra_kwargs,
    ).export()


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
