from __future__ import annotations

import csv
import tempfile
import unittest
from pathlib import Path

from photos_backup.apple_photos.late_additions import (
    generate_late_photo_additions_report,
)
from photos_backup.config import parse_csv_env
from photos_backup.summary import parse_apple_photos_csv


class ConfigTests(unittest.TestCase):
    def test_parse_csv_env_trims_empty_values(self) -> None:
        self.assertEqual(
            parse_csv_env(" iPhone SE (2nd generation),, iPhone 17 "),
            ("iPhone SE (2nd generation)", "iPhone 17"),
        )


class ApplePhotosCsvTests(unittest.TestCase):
    def test_parse_one_hot_osxphotos_report(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            report_path = Path(tmpdir) / "photos_export.csv"
            with report_path.open("w", newline="") as f:
                writer = csv.DictWriter(
                    f,
                    fieldnames=[
                        "datetime",
                        "filename",
                        "exported",
                        "new",
                        "updated",
                        "skipped",
                        "missing",
                    ],
                )
                writer.writeheader()
                writer.writerow(
                    {
                        "datetime": "2026-06-07T22:25:19",
                        "filename": "/tmp/new.heic",
                        "exported": "1",
                        "new": "1",
                        "updated": "0",
                        "skipped": "0",
                        "missing": "0",
                    }
                )
                writer.writerow(
                    {
                        "datetime": "2026-06-07T22:25:20",
                        "filename": "/tmp/updated.heic",
                        "exported": "1",
                        "new": "0",
                        "updated": "1",
                        "skipped": "0",
                        "missing": "0",
                    }
                )
                writer.writerow(
                    {
                        "datetime": "2026-06-07T22:25:21",
                        "filename": "/tmp/skipped.heic",
                        "exported": "0",
                        "new": "0",
                        "updated": "0",
                        "skipped": "1",
                        "missing": "0",
                    }
                )

            counts = parse_apple_photos_csv(str(report_path))

        self.assertEqual(counts["exported"], 2)
        self.assertEqual(counts["new"], 1)
        self.assertEqual(counts["updated"], 1)
        self.assertEqual(counts["skipped"], 1)
        self.assertEqual(counts["missing"], 0)

    def test_parse_legacy_export_status_report(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            report_path = Path(tmpdir) / "photos_export.csv"
            with report_path.open("w", newline="") as f:
                writer = csv.DictWriter(f, fieldnames=["filename", "export_status"])
                writer.writeheader()
                writer.writerow(
                    {"filename": "/tmp/new.heic", "export_status": "new"}
                )
                writer.writerow(
                    {"filename": "/tmp/updated.heic", "export_status": "updated"}
                )
                writer.writerow(
                    {"filename": "/tmp/skipped.heic", "export_status": "skipped"}
                )

            counts = parse_apple_photos_csv(str(report_path))

        self.assertEqual(counts, {"new": 1, "updated": 1, "skipped": 1})


class LateAdditionsReportTests(unittest.TestCase):
    def test_generates_report_with_device_and_late_month_flags(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp_path = Path(tmpdir)
            export_report_path = tmp_path / "photos_export.csv"
            output_path = tmp_path / "late_photo_additions.csv"
            spouse_old = tmp_path / "spouse-old.heic"
            other_same_month = tmp_path / "other-same-month.heic"
            missing_model_old = tmp_path / "missing-model-old.heic"
            skipped = tmp_path / "skipped.heic"

            self._write_export_report(
                export_report_path,
                [
                    {
                        "datetime": "2026-06-07T22:25:19",
                        "filename": str(spouse_old),
                        "exported": "1",
                        "new": "1",
                        "updated": "0",
                        "skipped": "0",
                    },
                    {
                        "datetime": "2026-06-07T22:25:20",
                        "filename": str(other_same_month),
                        "exported": "1",
                        "new": "0",
                        "updated": "1",
                        "skipped": "0",
                    },
                    {
                        "datetime": "2026-06-07T22:25:21",
                        "filename": str(missing_model_old),
                        "exported": "1",
                        "new": "0",
                        "updated": "1",
                        "skipped": "0",
                    },
                    {
                        "datetime": "2026-06-07T22:25:22",
                        "filename": str(skipped),
                        "exported": "0",
                        "new": "0",
                        "updated": "0",
                        "skipped": "1",
                    },
                ],
            )
            metadata = {
                spouse_old: {
                    "kMDItemContentCreationDate": "2026-05-21 16:40:50 +0000",
                    "kMDItemDateAdded": "2026-06-07 18:19:48 +0000",
                    "kMDItemAcquisitionMake": "Apple",
                    "kMDItemAcquisitionModel": "iPhone SE (2nd generation)",
                },
                other_same_month: {
                    "kMDItemContentCreationDate": "2026-06-01 15:00:00 +0000",
                    "kMDItemDateAdded": "2026-06-07 18:19:48 +0000",
                    "kMDItemAcquisitionMake": "Apple",
                    "kMDItemAcquisitionModel": "iPhone 13 Pro",
                },
                missing_model_old: {
                    "kMDItemContentCreationDate": "2026-05-31 23:08:48 +0000",
                    "kMDItemDateAdded": "2026-05-31 23:09:00 +0000",
                    "kMDItemAcquisitionMake": "",
                    "kMDItemAcquisitionModel": "",
                },
            }

            rows_written = generate_late_photo_additions_report(
                export_report_path=export_report_path,
                output_path=output_path,
                spouse_device_models=("iPhone SE",),
                metadata_reader=lambda path: metadata.get(path, {}),
            )

            with output_path.open(newline="") as f:
                rows = list(csv.DictReader(f))

        self.assertEqual(rows_written, 3)
        self.assertEqual(len(rows), 3)
        self.assertEqual(rows[0]["export_status"], "new")
        self.assertEqual(rows[0]["is_spouse_device"], "true")
        self.assertEqual(rows[0]["is_late_month_addition"], "true")
        self.assertEqual(rows[1]["export_status"], "updated")
        self.assertEqual(rows[1]["is_spouse_device"], "false")
        self.assertEqual(rows[1]["is_late_month_addition"], "false")
        self.assertEqual(rows[2]["is_spouse_device"], "false")
        self.assertEqual(rows[2]["is_late_month_addition"], "true")

    def _write_export_report(
        self,
        path: Path,
        rows: list[dict[str, str]],
    ) -> None:
        with path.open("w", newline="") as f:
            writer = csv.DictWriter(
                f,
                fieldnames=[
                    "datetime",
                    "filename",
                    "exported",
                    "new",
                    "updated",
                    "skipped",
                ],
            )
            writer.writeheader()
            writer.writerows(rows)


if __name__ == "__main__":
    unittest.main()
