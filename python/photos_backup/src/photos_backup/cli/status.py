import datetime
import json
from dataclasses import asdict

import click

from photos_backup.apple_photos.plan import plan_export
from photos_backup.archive import ArchiveError, open_archive
from photos_backup.cli.context import apple_photos_config_from, config_path_from
from photos_backup.summary import print_archive_status, print_transfer_history
from photos_backup.transfers import TransferHistory


@click.command(
    name="status",
    help="Show recorded backup state and the next export, without scanning files.",
)
@click.option("--json", "as_json", is_flag=True, help="Print recorded status as JSON.")
@click.pass_context
def status(ctx: click.Context, as_json: bool) -> None:
    config = apple_photos_config_from(ctx)
    receipts, errors = TransferHistory(config_path_from(ctx)).read()
    now = datetime.datetime.now(datetime.UTC)
    document = {
        "version": 1,
        "archive": str(config.archive),
        "observed_at": now.isoformat(),
        "files_verified": False,
        "state": None,
        "next_export": None,
        "archive_error": None,
        "transfers": receipts,
        "transfer_history_errors": errors,
    }
    archive_error = None
    try:
        with open_archive(config, dry_run=True) as archive:
            state = archive.state_store.load()
            now = archive.now()
            plan = plan_export(config, state, now)
            state_document = asdict(state)
            for name, value in state_document.items():
                if hasattr(value, "isoformat"):
                    state_document[name] = value.isoformat()
            document.update(
                observed_at=now.isoformat(),
                state=state_document,
                next_export={"mode": plan.mode.value, "reason": plan.reason},
            )
            if not as_json:
                print_archive_status(archive, state, plan, now=now)
    except (ArchiveError, OSError) as error:
        archive_error = (
            error
            if isinstance(error, ArchiveError)
            else click.ClickException(str(error))
        )
        document["archive_error"] = str(error)
        if not as_json:
            click.echo(f"Archive unavailable: {config.archive} — {error}")
    if as_json:
        click.echo(json.dumps(document, indent=2, default=str))
    else:
        print_transfer_history(receipts, errors, now=now)
    if archive_error is not None:
        raise archive_error
