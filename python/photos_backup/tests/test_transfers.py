from unittest import mock


from photos_backup.summary import BackupSummary
from photos_backup.transfers import TransferHistory
from tests.transfer_history_case import TransferHistoryTestCase


class TransferHistoryTests(TransferHistoryTestCase):
    def test_reads_and_previews_create_nothing(self):
        self.assertEqual(self.history.read(), ([], []))
        self.run_transfer(dry_run=True)
        self.assertFalse(self.history_root.exists())

    def test_failed_or_interrupted_attempt_retains_previous_success(self):
        self.run_transfer()
        success = self.receipt()["last_success"]
        self.assertEqual(success["files_transferred"], 4)
        for error in (OSError("offline"), KeyboardInterrupt()):
            with self.subTest(error=error):
                with self.assertRaises(type(error)):
                    self.run_transfer(mock.Mock(side_effect=error))
                receipt = self.receipt()
                self.assertEqual(receipt["last_success"], success)
                self.assertEqual(
                    receipt["last_attempt"]["status"],
                    "failed" if isinstance(error, Exception) else "interrupted",
                )
        self.run_transfer(
            lambda: BackupSummary("SSD: All Photos", error="failed result")
        )
        self.assertEqual(self.receipt()["last_attempt"]["error"], "failed result")
        self.assertEqual(self.receipt()["last_success"], success)

    def test_started_attempt_is_visible_and_preview_never_updates_existing_receipt(
        self,
    ):
        def copy():
            self.assertEqual(self.receipt()["last_attempt"]["status"], "started")
            self.assertIsNone(self.receipt()["last_success"])
            return BackupSummary("SSD: All Photos")

        self.run_transfer(copy)
        before = {p: p.read_bytes() for p in self.history.directory.iterdir()}
        self.run_transfer(dry_run=True)
        self.assertEqual(
            {p: p.read_bytes() for p in self.history.directory.iterdir()}, before
        )

    def test_destination_and_configuration_histories_are_separate(self):
        self.run_transfer()
        self.run_transfer(destination=self.root / "another/photos")
        receipts, errors = self.history.read()
        self.assertEqual(len(receipts), 2)
        self.assertEqual(errors, [])
        self.assertEqual(TransferHistory(self.root / "another.toml").read(), ([], []))

    def test_overlapping_attempt_completions_keep_newest_attempt_and_success(self):
        identity = {
            "step": "Remote",
            "source": str(self.source),
            "destination": "b2:photos",
        }
        old = {
            "started_at": "2026-10-04T10:00:00+00:00",
            "completed_at": None,
            "status": "started",
        }
        new = {**old, "started_at": "2026-10-04T11:00:00+00:00"}
        self.history._record(identity, old)
        self.history._record(identity, new)
        self.history._record(
            identity,
            {**old, "completed_at": "2026-10-04T12:00:00+00:00", "status": "succeeded"},
        )
        self.history._record(
            identity,
            {**new, "completed_at": "2026-10-04T13:00:00+00:00", "status": "failed"},
        )
        receipt = self.receipt()
        self.assertEqual(receipt["last_attempt"]["started_at"], new["started_at"])
        self.assertEqual(receipt["last_attempt"]["status"], "failed")
        self.assertEqual(receipt["last_success"]["started_at"], old["started_at"])

    def test_unwritable_or_corrupt_history_warns_without_masking_transfer_result(self):
        self.history_root.write_text("not a directory")
        self.assertEqual(len(self.history.read()[1]), 1)
        with mock.patch("photos_backup.transfers.click.echo") as echo:
            self.assertEqual(self.run_transfer().files_transferred, 4)
        self.assertIn("could not save transfer receipt", echo.call_args.args[0])
        self.history_root.unlink()
        self.run_transfer()
        path = next(self.history.directory.glob("*.json"))
        path.write_text("invalid")
        receipts, errors = self.history.read()
        self.assertEqual(receipts, [])
        self.assertEqual(len(errors), 1)
        with mock.patch("photos_backup.transfers.click.echo"):
            self.assertEqual(self.run_transfer().files_transferred, 4)
        self.assertEqual(self.receipt()["last_attempt"]["status"], "succeeded")
        quarantined = list(self.history.directory.glob("*.corrupt-*"))
        self.assertEqual(len(quarantined), 1)
        self.assertEqual(quarantined[0].read_text(), "invalid")

    def test_recovery_preserves_each_corrupt_receipt_without_inventing_success(self):
        self.run_transfer()
        path = next(self.history.directory.glob("*.json"))
        for content in (b"invalid", b"\xff", b'{"version": 1}'):
            with self.subTest(content=content):
                path.write_bytes(content)
                before = {p: p.read_bytes() for p in self.history.directory.iterdir()}
                self.history.read()
                self.run_transfer(dry_run=True)
                self.assertEqual(
                    {p: p.read_bytes() for p in self.history.directory.iterdir()},
                    before,
                )
                with mock.patch("photos_backup.transfers.click.echo") as echo:
                    self.run_transfer(
                        lambda: BackupSummary("SSD: All Photos", error="offline")
                    )
                self.assertIn(
                    "preserved invalid transfer receipt", echo.call_args.args[0]
                )
                receipt = self.receipt()
                self.assertIsNone(receipt["last_success"])
                self.assertEqual(receipt["last_attempt"]["status"], "failed")
        self.assertCountEqual(
            [p.read_bytes() for p in self.history.directory.glob("*.corrupt-*")],
            [b"invalid", b"\xff", b'{"version": 1}'],
        )
        self.run_transfer()
        self.assertEqual(self.receipt()["last_success"]["files_transferred"], 4)

    def test_failed_quarantine_preserves_corrupt_receipt_and_transfer_result(self):
        self.run_transfer()
        path = next(self.history.directory.glob("*.json"))
        path.write_text("invalid")
        with (
            mock.patch(
                "photos_backup.transfers.os.replace", side_effect=OSError("denied")
            ),
            mock.patch("photos_backup.transfers.click.echo") as echo,
        ):
            self.assertEqual(self.run_transfer().files_transferred, 4)
        self.assertIn("could not save transfer receipt", echo.call_args.args[0])
        self.assertEqual(path.read_text(), "invalid")
        self.assertEqual(list(self.history.directory.glob("*.corrupt-*")), [])
