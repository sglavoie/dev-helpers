from __future__ import annotations

import csv
import subprocess
import tempfile
import threading
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.apple_photos.late_additions import (
    METADATA_BATCH_SIZE,
    is_earlier_month,
    parse_mdls_datetime,
    generate_late_photo_additions_report,
    read_spotlight_metadata,
)
from photos_backup.summary import read_export_report
from tests.timezones import pin_timezone


class LateMonthTests(unittest.TestCase):
    def test_months_are_compared_in_local_time_not_utc(self):
        pin_timezone(self, "America/New_York")
        # 22:00 on May 31 in New York is already June 1 in UTC.
        taken = parse_mdls_datetime("2026-06-01 02:00:00 +0000")
        added = parse_mdls_datetime("2026-06-02 12:00:00 +0000")

        self.assertTrue(is_earlier_month(taken, added))

        pin_timezone(self, "Europe/Paris")
        # 01:00 on June 1 in Paris is still May 31 in UTC.
        taken = parse_mdls_datetime("2026-05-31 23:00:00 +0000")
        self.assertFalse(is_earlier_month(taken, added))


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

            counts = read_export_report(report_path).counts

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
                writer.writerow({"filename": "/tmp/new.heic", "export_status": "new"})
                writer.writerow(
                    {"filename": "/tmp/updated.heic", "export_status": "updated"}
                )
                writer.writerow(
                    {"filename": "/tmp/skipped.heic", "export_status": "skipped"}
                )

            counts = read_export_report(report_path).counts

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

    def test_duplicate_paths_reuse_metadata_and_keep_all_report_rows(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "export.csv"
            target = Path(directory) / "late.csv"
            self._write_export_report(
                source,
                [
                    {"filename": "/image.jpg", "new": "1"},
                    {"filename": "/image.jpg", "updated": "1"},
                    {"filename": "/skipped.jpg", "skipped": "1"},
                ],
            )
            reader = mock.Mock(return_value={})
            warning = mock.Mock()
            count = generate_late_photo_additions_report(
                source, target, (), reader, warning
            )
            reader.assert_called_once_with(Path("/image.jpg"))
            self.assertEqual(count, 2)
            self.assertIn("1 file(s)", warning.call_args.args[0])
            with target.open() as stream:
                self.assertEqual(len(list(csv.DictReader(stream))), 2)

    def test_spotlight_timeout_preserves_empty_metadata(self):
        with mock.patch(
            "photos_backup.apple_photos.late_additions.subprocess.run",
            side_effect=subprocess.TimeoutExpired("mdls", 10),
        ) as run:
            self.assertEqual(read_spotlight_metadata(Path("/image.jpg")), {})
        self.assertEqual(run.call_args.kwargs["timeout"], 10)

    def test_concurrent_readers_are_bounded_and_csv_keeps_source_order(self):
        workers = 4
        barrier = threading.Barrier(workers)
        lock = threading.Lock()
        active = peak = 0
        first_finished = threading.Event()

        def reader(path):
            nonlocal active, peak
            with lock:
                active += 1
                peak = max(peak, active)
            try:
                barrier.wait(timeout=5)
                # Make a later row finish before the first row of its group.
                if int(path.stem) % workers == 0:
                    self.assertTrue(first_finished.wait(timeout=5))
                else:
                    first_finished.set()
                return {"kMDItemAcquisitionModel": path.name}
            finally:
                with lock:
                    active -= 1

        with tempfile.TemporaryDirectory() as directory:
            source, target = (
                Path(directory) / "export.csv",
                Path(directory) / "late.csv",
            )
            names = [f"/{i}.jpg" for i in range(workers * 3)]
            self._write_export_report(
                source, [{"filename": name, "new": "1"} for name in names]
            )
            count = generate_late_photo_additions_report(
                source, target, (), reader, metadata_workers=workers
            )
            with target.open() as stream:
                rows = list(csv.DictReader(stream))
        self.assertEqual(count, len(names))
        self.assertEqual(peak, workers)
        self.assertEqual(active, 0)
        self.assertEqual([row["filename"] for row in rows], names)
        self.assertEqual(
            [row["acquisition_model"] for row in rows],
            [Path(name).name for name in names],
        )

    def test_batches_preserve_cache_missing_metadata_and_serial_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "export.csv"
            rows = [
                {"filename": f"/{i}.jpg", "new": "1"}
                for i in range(METADATA_BATCH_SIZE + 1)
            ]
            rows.extend(
                [
                    {"filename": "/0.jpg", "updated": "1"},
                    {"filename": "", "new": "1"},
                    {"filename": "/skipped.jpg", "skipped": "1"},
                ]
            )
            self._write_export_report(source, rows)
            for workers in (1, 4):
                reader = mock.Mock(
                    side_effect=lambda path: (
                        {}
                        if path.name == "0.jpg"
                        else {"kMDItemAcquisitionModel": path.name}
                    )
                )
                warning = mock.Mock()
                target = root / f"late-{workers}.csv"
                count = generate_late_photo_additions_report(
                    source, target, (), reader, warning, metadata_workers=workers
                )
                self.assertEqual(count, len(rows) - 1)
                self.assertCountEqual(
                    [call.args[0] for call in reader.call_args_list],
                    [Path(f"/{i}.jpg") for i in range(METADATA_BATCH_SIZE + 1)],
                )
                warning.assert_called_once()
                self.assertIn("1 file(s)", warning.call_args.args[0])
            self.assertEqual(
                (root / "late-1.csv").read_bytes(), (root / "late-4.csv").read_bytes()
            )

    def test_reader_error_shuts_down_pool_and_propagates(self):
        with tempfile.TemporaryDirectory() as directory:
            source, target = (
                Path(directory) / "export.csv",
                Path(directory) / "late.csv",
            )
            self._write_export_report(source, [{"filename": "/0.jpg", "new": "1"}])
            threads = []

            def reader(path):
                threads.append(threading.current_thread())
                raise OSError("metadata reader failed")

            with self.assertRaisesRegex(OSError, "metadata reader failed"):
                generate_late_photo_additions_report(source, target, (), reader)
            self.assertTrue(threads)
            self.assertTrue(all(not thread.is_alive() for thread in threads))

    def test_invalid_worker_count_writes_nothing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for workers in (0, -1):
                with self.assertRaisesRegex(ValueError, "must be positive"):
                    generate_late_photo_additions_report(
                        root / "missing.csv",
                        root / "late.csv",
                        (),
                        metadata_workers=workers,
                    )
            self.assertEqual(list(root.iterdir()), [])

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
