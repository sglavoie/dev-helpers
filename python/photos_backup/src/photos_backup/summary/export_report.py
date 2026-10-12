"""Reading osxphotos CSV export reports."""

from __future__ import annotations

import csv
from dataclasses import dataclass
from dataclasses import field as dataclass_field
from pathlib import Path

from photos_backup.apple_photos.late_additions import csv_flag


ONE_HOT_REPORT_FIELDS = frozenset(
    {
        "exported",
        "new",
        "updated",
        "skipped",
        "exif_updated",
        "touched",
        "converted_to_jpeg",
        "sidecar_xmp",
        "sidecar_json",
        "sidecar_exiftool",
        "missing",
        "error",
        "exiftool_warning",
        "exiftool_error",
        "extended_attributes_written",
        "extended_attributes_skipped",
        "cleanup_deleted_file",
        "cleanup_deleted_directory",
        "sidecar_user",
        "sidecar_user_error",
        "user_written",
        "user_skipped",
        "user_error",
        "aae_written",
        "aae_skipped",
    }
)


@dataclass(frozen=True)
class ExportReport:
    """One osxphotos CSV report, or the reason it could not be read."""

    counts: dict[str, int] = dataclass_field(default_factory=dict)
    problem: str | None = None


def read_export_report(report_path: Path) -> ExportReport:
    """Read an osxphotos export CSV report and count rows by status.

    A report that was never written, cannot be read, or carries none of the
    columns osxphotos writes is a problem rather than a run with nothing to
    report; a report holding only its header is a clean run that exported
    nothing.
    """
    if not report_path.is_file():
        return ExportReport(problem=f"osxphotos wrote no report at '{report_path}'")

    counts: dict[str, int] = {}
    try:
        with report_path.open(newline="") as f:
            reader = csv.DictReader(f)
            fieldnames = set(reader.fieldnames or [])
            one_hot_fields = ONE_HOT_REPORT_FIELDS.intersection(fieldnames)
            if not one_hot_fields and "export_status" not in fieldnames:
                return ExportReport(
                    problem=(
                        f"the report at '{report_path}' has no column this tool "
                        "recognizes"
                    )
                )

            for row in reader:
                if one_hot_fields:
                    for name in one_hot_fields:
                        counts[name] = counts.get(name, 0) + _csv_count(row.get(name))
                    continue

                status = row.get("export_status", "").lower().strip()
                if status:
                    counts[status] = counts.get(status, 0) + 1
    except (OSError, UnicodeDecodeError, csv.Error) as error:
        return ExportReport(
            problem=f"the report at '{report_path}' could not be read ({error})"
        )
    return ExportReport(counts=counts)


def _csv_count(value: str | None) -> int:
    if not csv_flag(value):
        return 0

    try:
        return int((value or "").strip())
    except ValueError:
        return 1
