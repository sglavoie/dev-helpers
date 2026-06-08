from __future__ import annotations

import csv
import subprocess
from collections.abc import Callable
from datetime import datetime
from pathlib import Path

SPOTLIGHT_KEYS = (
    "kMDItemContentCreationDate",
    "kMDItemDateAdded",
    "kMDItemAcquisitionMake",
    "kMDItemAcquisitionModel",
)

REPORT_COLUMNS = (
    "filename",
    "export_status",
    "export_run_timestamp",
    "content_creation_date",
    "date_added",
    "acquisition_make",
    "acquisition_model",
    "is_spouse_device",
    "is_late_month_addition",
)

MetadataReader = Callable[[Path], dict[str, str]]


def generate_late_photo_additions_report(
    export_report_path: Path,
    output_path: Path,
    spouse_device_models: tuple[str, ...],
    metadata_reader: MetadataReader | None = None,
) -> int:
    """Write a CSV report for files newly exported or updated in this run."""
    if not export_report_path.exists():
        return 0

    metadata_reader = metadata_reader or read_spotlight_metadata
    rows = []
    with export_report_path.open(newline="") as f:
        reader = csv.DictReader(f)
        for row in reader:
            statuses = export_statuses(row)
            if not {"new", "updated"}.intersection(statuses):
                continue

            filename = row.get("filename", "")
            metadata = metadata_reader(Path(filename)) if filename else {}
            report_row = build_report_row(row, metadata, spouse_device_models)
            rows.append(report_row)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=REPORT_COLUMNS)
        writer.writeheader()
        writer.writerows(rows)

    return len(rows)


def build_report_row(
    export_row: dict[str, str],
    metadata: dict[str, str],
    spouse_device_models: tuple[str, ...],
) -> dict[str, str]:
    """Build one late-additions report row from osxphotos and Spotlight data."""
    content_creation_date = metadata.get("kMDItemContentCreationDate", "")
    date_added = metadata.get("kMDItemDateAdded", "")
    acquisition_model = metadata.get("kMDItemAcquisitionModel", "")

    date_added_dt = parse_mdls_datetime(date_added)
    export_dt = parse_export_datetime(export_row.get("datetime", ""))
    content_date = parse_mdls_datetime(content_creation_date)
    is_late_month_addition = is_earlier_month(
        content_date, date_added_dt
    ) or is_earlier_month(content_date, export_dt)

    return {
        "filename": export_row.get("filename", ""),
        "export_status": "+".join(export_statuses(export_row)),
        "export_run_timestamp": export_row.get("datetime", ""),
        "content_creation_date": content_creation_date,
        "date_added": date_added,
        "acquisition_make": metadata.get("kMDItemAcquisitionMake", ""),
        "acquisition_model": acquisition_model,
        "is_spouse_device": str(
            matches_spouse_device(acquisition_model, spouse_device_models)
        ).lower(),
        "is_late_month_addition": str(is_late_month_addition).lower(),
    }


def read_spotlight_metadata(path: Path) -> dict[str, str]:
    """Read selected Spotlight metadata for an exported photo or video."""
    cmd = ["mdls"]
    for key in SPOTLIGHT_KEYS:
        cmd.extend(["-name", key])
    cmd.append(str(path))

    try:
        result = subprocess.run(
            cmd,
            check=True,
            capture_output=True,
            text=True,
        )
    except (FileNotFoundError, subprocess.CalledProcessError):
        return {}

    metadata: dict[str, str] = {}
    for line in result.stdout.splitlines():
        key, separator, value = line.partition("=")
        if not separator:
            continue
        normalized = value.strip()
        if normalized == "(null)":
            normalized = ""
        elif normalized.startswith('"') and normalized.endswith('"'):
            normalized = normalized[1:-1]
        metadata[key.strip()] = normalized
    return metadata


def export_statuses(row: dict[str, str]) -> tuple[str, ...]:
    """Return osxphotos status names from legacy or one-hot CSV rows."""
    export_status = row.get("export_status", "").strip().lower()
    if export_status:
        return (export_status,)

    transfer_statuses = [
        status for status in ("new", "updated") if csv_flag(row.get(status, ""))
    ]
    if transfer_statuses:
        return tuple(transfer_statuses)

    statuses = []
    for status in ("exported", "skipped", "missing", "error"):
        if csv_flag(row.get(status, "")):
            statuses.append(status)
    return tuple(statuses)


def csv_flag(value: str | None) -> bool:
    if value is None:
        return False

    stripped = value.strip().lower()
    if not stripped or stripped in {"0", "false", "no"}:
        return False

    try:
        return int(stripped) != 0
    except ValueError:
        return True


def matches_spouse_device(
    acquisition_model: str,
    spouse_device_models: tuple[str, ...],
) -> bool:
    if not acquisition_model:
        return False

    normalized_model = acquisition_model.casefold()
    return any(model.casefold() in normalized_model for model in spouse_device_models)


def is_earlier_month(
    content_date: datetime | None,
    comparison_date: datetime | None,
) -> bool:
    if content_date is None or comparison_date is None:
        return False
    return (content_date.year, content_date.month) < (
        comparison_date.year,
        comparison_date.month,
    )


def parse_mdls_datetime(value: str) -> datetime | None:
    if not value:
        return None

    try:
        return datetime.strptime(value, "%Y-%m-%d %H:%M:%S %z")
    except ValueError:
        return None


def parse_export_datetime(value: str) -> datetime | None:
    if not value:
        return None

    try:
        return datetime.fromisoformat(value)
    except ValueError:
        return None
