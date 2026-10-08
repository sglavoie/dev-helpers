"""Turn copy step summaries into the documented exit codes."""

from __future__ import annotations

from collections.abc import Sequence

import click

from photos_backup.errors import ActionRequired
from photos_backup.summary import BackupSummary


def raise_for_summaries(
    summaries: Sequence[BackupSummary], *, name_failed_steps: bool = False
) -> None:
    """Exit 1 for any failed step, else 3 for any step that needs a person.

    A pipeline has already shown each error in its table, so it names the
    failed steps instead of repeating their messages.
    """
    failures = [s for s in summaries if s.error and not s.action_required]
    if failures:
        if name_failed_steps:
            names = ", ".join(s.step_name for s in failures)
            raise click.ClickException(f"Step(s) failed: {names}")
        raise click.ClickException("; ".join(s.error or "" for s in failures))
    actions = [s.error for s in summaries if s.error and s.action_required]
    if actions:
        raise ActionRequired("; ".join(actions))
