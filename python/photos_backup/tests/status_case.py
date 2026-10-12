from __future__ import annotations

import datetime

EARLY = datetime.datetime(2026, 8, 10, tzinfo=datetime.UTC)
LATE = EARLY + datetime.timedelta(days=1)
TIMEOUT_COUNT = 3


def transfer(step, source, destination, when=EARLY):
    success = {
        "started_at": when.isoformat(),
        "completed_at": when.isoformat(),
        "status": "succeeded",
    }
    return {
        "step": step,
        "source": source,
        "destination": destination,
        "last_attempt": success,
        "last_success": success,
    }
