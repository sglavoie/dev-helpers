"""Compare serial and concurrent report enrichment without running an export."""

from __future__ import annotations

import argparse
import cProfile
import csv
import statistics
import tempfile
import time
from pathlib import Path

from photos_backup.apple_photos.late_additions import (
    METADATA_WORKERS,
    generate_late_photo_additions_report,
)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--files", type=int, default=80)
    parser.add_argument("--repeats", type=int, default=3)
    parser.add_argument("--delay-ms", type=float, default=10)
    parser.add_argument(
        "--spotlight",
        action="store_true",
        help="run actual mdls on temporary fixtures instead of simulated I/O",
    )
    parser.add_argument(
        "--export-report",
        type=Path,
        help="read an existing export CSV and query its files with mdls; writes only temporary reports",
    )
    parser.add_argument(
        "--profile",
        type=Path,
        help="write a cProfile file for the concurrent report coordinator (worker time appears as waiting)",
    )
    args = parser.parse_args()
    if args.files < 1 or args.repeats < 1 or args.delay_ms < 0:
        parser.error("files and repeats must be positive; delay-ms must be nonnegative")
    if args.export_report is not None and not args.export_report.is_file():
        parser.error("export-report must be an existing CSV file")

    with tempfile.TemporaryDirectory(prefix="photos-backup-profile-") as directory:
        root = Path(directory)
        source = args.export_report or root / "export.csv"
        if args.export_report is None:
            with source.open("w", newline="") as stream:
                writer = csv.DictWriter(stream, fieldnames=["filename", "new"])
                writer.writeheader()
                for index in range(args.files):
                    path = root / f"fixture-{index}.txt"
                    path.write_text("Temporary metadata profiling fixture.\n")
                    writer.writerow({"filename": str(path), "new": "1"})

        def delayed_reader(path: Path) -> dict[str, str]:
            time.sleep(args.delay_ms / 1000)
            return {"kMDItemAcquisitionModel": path.name}

        real_spotlight = args.spotlight or args.export_report is not None
        reader = None if real_spotlight else delayed_reader
        label = (
            "actual mdls" if real_spotlight else f"simulated {args.delay_ms:g}ms I/O"
        )
        print(f"Report enrichment only: {label}", flush=True)
        samples: dict[int, list[float]] = {1: [], METADATA_WORKERS: []}
        outputs: dict[int, bytes] = {}
        for repeat in range(args.repeats):
            # Alternate ordering to reduce warm-cache bias in the comparison.
            order = (1, METADATA_WORKERS) if repeat % 2 == 0 else (METADATA_WORKERS, 1)
            for workers in order:
                target = root / f"late-{workers}.csv"
                started = time.perf_counter()
                count = generate_late_photo_additions_report(
                    source,
                    target,
                    (),
                    reader,
                    lambda _: None,
                    metadata_workers=workers,
                )
                samples[workers].append(time.perf_counter() - started)
                outputs[workers] = target.read_bytes()
        for workers, values in samples.items():
            print(
                f"{workers} worker(s): {statistics.median(values):.3f}s median; {count} rows"
            )
        print(
            f"Speedup: {statistics.median(samples[1]) / statistics.median(samples[METADATA_WORKERS]):.2f}x"
        )
        print(f"Identical CSV output: {outputs[1] == outputs[METADATA_WORKERS]}")
        if args.profile:
            profiler = cProfile.Profile()
            profiler.runcall(
                generate_late_photo_additions_report,
                source,
                root / "profile.csv",
                (),
                reader,
                lambda _: None,
            )
            profiler.dump_stats(str(args.profile))
            print(f"Coordinator profile: {args.profile}")


if __name__ == "__main__":
    main()
