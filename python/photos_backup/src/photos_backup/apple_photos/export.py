from __future__ import annotations

import datetime
import time
from typing import TYPE_CHECKING

from osxphotos.cli.export import export_cli

from photos_backup.summary import BackupSummary, parse_apple_photos_csv

if TYPE_CHECKING:
    from photos_backup.config import Config


class ApplePhotosExport:
    def __init__(self, config: Config, testing: bool, extra_kwargs: dict | None = None):
        self.testing = testing
        self.limit = config.apple_photos_limit_export if testing else 0
        self.dst_path = config.apple_photos_dst_path
        self.extra_kwargs = extra_kwargs or {}

    def export(self) -> BackupSummary:
        kwargs = {
            # Testing flags
            "dry_run": self.testing,
            "verbose_flag": self.testing,
            "limit": self.limit,
            # Regular flags
            "dest": str(self.dst_path),
            "exiftool": True,
            "directory": "{created.year}/{created.mm}/{album[ ,_],}",
            "filename_template": "{created.strftime,%Y-%m-%d-%H%M%S}_{original_name}",
            "report": "photos_export_{today.date}.csv",
            "update": True,
        }
        # Extra kwargs override defaults
        kwargs.update(self.extra_kwargs)

        csv_report = f"photos_export_{datetime.date.today()}.csv"

        start = time.monotonic()
        export_cli(**kwargs)
        elapsed = time.monotonic() - start

        counts = parse_apple_photos_csv(csv_report)
        files_exported = counts.get("exported", 0) + counts.get("new", 0)
        files_updated = counts.get("updated", 0)
        total_files = files_exported + files_updated

        return BackupSummary(
            step_name="Apple Photos",
            files_transferred=total_files,
            elapsed_seconds=elapsed,
        )
