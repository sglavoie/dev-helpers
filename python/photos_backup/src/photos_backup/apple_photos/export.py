from __future__ import annotations

import datetime
import time
from pathlib import Path
from typing import TYPE_CHECKING

from osxphotos.cli.export import export_cli

from photos_backup.apple_photos.late_additions import (
    generate_late_photo_additions_report,
)
from photos_backup.summary import BackupSummary, parse_apple_photos_csv

if TYPE_CHECKING:
    from photos_backup.config import Config


class ApplePhotosExport:
    def __init__(self, config: Config, testing: bool, extra_kwargs: dict | None = None):
        self.testing = testing
        self.limit = config.apple_photos_limit_export if testing else 0
        self.dst_path = config.apple_photos_dst_path
        self.spouse_device_models = config.apple_photos_spouse_device_models
        self.extra_kwargs = extra_kwargs or {}

    def export(self) -> BackupSummary:
        self.dst_path.mkdir(parents=True, exist_ok=True)
        today = datetime.date.today()
        csv_report = Path(f"photos_export_{today}.csv")
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
            "report": str(csv_report),
            "update": True,
        }
        # Extra kwargs override defaults
        kwargs.update(self.extra_kwargs)
        csv_report = _resolve_report_path(str(kwargs["report"]), today)
        late_additions_report = csv_report.with_name(f"late_photo_additions_{today}.csv")

        start = time.monotonic()
        export_cli(**kwargs)
        elapsed = time.monotonic() - start

        counts = parse_apple_photos_csv(str(csv_report))
        files_exported = counts.get("new", 0)
        files_updated = counts.get("updated", 0)
        total_files = files_exported + files_updated or counts.get("exported", 0)

        generate_late_photo_additions_report(
            export_report_path=csv_report,
            output_path=late_additions_report,
            spouse_device_models=self.spouse_device_models,
        )

        return BackupSummary(
            step_name="Apple Photos",
            files_transferred=total_files,
            elapsed_seconds=elapsed,
        )


def _resolve_report_path(report: str, today: datetime.date) -> Path:
    """Resolve the report filename shape this project uses by default."""
    return Path(report.replace("{today.date}", str(today)))
