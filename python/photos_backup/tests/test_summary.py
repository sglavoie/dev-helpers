import io
import unittest
from pathlib import Path
from unittest import mock

from photos_backup.cli.cli import cli
from photos_backup.cli.context import CliContext
from photos_backup.summary import parse_rsync_stats, print_mirror_outcome

OPENRSYNC_STATS = """\
Number of files: 6
Number of files transferred: 4
Total file size: 1500 kB
Total transferred file size: 1500 kB
Unmatched data: 1500 kB
Matched data: 0 B
File list size: 190 B
Total sent: 1501 kB
Total received: 120 B

sent 1501k bytes  received 120 bytes  185M bytes/sec
total size is 1500k  speedup is 1.00
"""


GNU_RSYNC_STATS = """\
Number of files: 1,206 (reg: 1,204, dir: 2)
Number of created files: 6 (reg: 4, dir: 2)
Number of deleted files: 0
Number of regular files transferred: 1,204
Total file size: 1.50G bytes
Total transferred file size: 1.50G bytes
"""


class RsyncStatsTests(unittest.TestCase):
    def test_openrsync_statistics_are_parsed(self):
        self.assertEqual(
            parse_rsync_stats(OPENRSYNC_STATS),
            {"files_transferred": 4, "total_size": "1500 kB"},
        )

    def test_openrsync_without_human_readable_sizes(self):
        stats = parse_rsync_stats(
            "Number of files transferred: 0\nTotal transferred file size: 0 B\n"
        )
        self.assertEqual(stats, {"files_transferred": 0, "total_size": "0 B"})

    def test_gnu_rsync_plain_sizes(self):
        stats = parse_rsync_stats(
            "Number of regular files transferred: 2\n"
            "Total transferred file size: 1,500,006 bytes\n"
        )
        self.assertEqual(
            stats, {"files_transferred": 2, "total_size": "1,500,006 bytes"}
        )

    def test_gnu_rsync_human_readable_statistics(self):
        self.assertEqual(
            parse_rsync_stats(GNU_RSYNC_STATS),
            {"files_transferred": 1204, "total_size": "1.50G bytes"},
        )


class MirrorOutcomeTests(unittest.TestCase):
    def test_pending_cleanup_offers_preview_approve_and_discard(self):
        outcome = mock.Mock(
            reason="awaiting approval",
            reconciliation=None,
            manifest_path=Path("/archive/cleanup/run-7.json"),
            run_id="run-7",
        )
        outcome.status.value = "pending"
        with cli.make_context("photos-backup", [], resilient_parsing=True) as ctx:
            ctx.obj = CliContext(config_path=None)
            with mock.patch("sys.stdout", new_callable=io.StringIO) as output:
                print_mirror_outcome(outcome)
        lines = output.getvalue().splitlines()
        self.assertEqual(
            lines[-3:],
            [
                "  Preview with: photos-backup approve-cleanup run-7 --dry-run",
                "  Approve with: photos-backup approve-cleanup run-7",
                "  Discard with: photos-backup approve-cleanup run-7 --discard",
            ],
        )


if __name__ == "__main__":
    unittest.main()
