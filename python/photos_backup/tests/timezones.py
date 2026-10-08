import os
import time


def pin_timezone(test, name: str) -> None:
    """Run one test under a fixed local time zone, restoring the original after."""
    original = os.environ.get("TZ")

    def restore() -> None:
        if original is None:
            os.environ.pop("TZ", None)
        else:
            os.environ["TZ"] = original
        time.tzset()

    test.addCleanup(restore)
    os.environ["TZ"] = name
    time.tzset()
