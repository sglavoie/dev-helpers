import datetime
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.config import SdCardConfig, SsdConfig
from photos_backup.errors import ActionRequired
from photos_backup.ssd.backup import Backup as SsdBackup
from photos_backup.summary import BackupSummary, print_transfer_history
from photos_backup.transfers import TransferHistory
from tests import isolate_transfer_history


class TransferHistoryTests(unittest.TestCase):
    def setUp(self):
        self.root = Path(self.enterContext(tempfile.TemporaryDirectory())).resolve()
        self.history_root = isolate_transfer_history(self)
        self.config = self.root / "config.toml"
        self.history = TransferHistory(self.config)
        self.source = self.root / "photos"
        self.destination = self.root / "backup/photos"

    def run_transfer(self, operation=None, *, dry_run=False, destination=None):
        return self.history.run(
            "SSD: All Photos",
            self.source,
            destination or self.destination,
            operation
            or (lambda: BackupSummary("SSD: All Photos", files_transferred=4)),
            dry_run=dry_run,
        )

    def receipt(self):
        receipts, errors = self.history.read()
        self.assertEqual(errors, [])
        self.assertEqual(len(receipts), 1)
        return receipts[0]

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
        self.assertEqual(path.read_text(), "invalid")

    def test_cli_records_ssd_and_remote_results_in_standalone_and_pipeline_runs(self):
        self.source.mkdir()
        for command in ("ssd", "remote", "backup-all"):
            with self.subTest(command=command):
                config = self.root / f"{command}.toml"
                config.write_text(
                    f'[ssd]\nsource = "{self.source}"\ndestination = "{self.root / "backup"}"\n'
                    f'[rclone]\nsource = "{self.source}"\nremote = "b2:photos"\n'
                )
                with (
                    mock.patch(
                        "photos_backup.cli.backup_all.which", return_value="tool"
                    ),
                    mock.patch(
                        "photos_backup.remote.backup.shutil.which",
                        return_value="rclone",
                    ),
                    mock.patch(
                        "photos_backup.ssd.backup.stream_command",
                        return_value=subprocess.CompletedProcess(
                            [], 0, stdout="Number of regular files transferred: 4\n"
                        ),
                    ),
                    mock.patch(
                        "photos_backup.remote.backup.stream_command",
                        return_value=subprocess.CompletedProcess(
                            [], 0, stdout="Transferred: 4 / 4\n"
                        ),
                    ),
                ):
                    args = ["--config", str(config), command]
                    if command == "backup-all":
                        args += ["--skip-apple-photos", "--skip-sd-card"]
                    result = CliRunner().invoke(cli, args)
                self.assertEqual(result.exit_code, 0, result.output)
                receipts, errors = TransferHistory(config).read()
                self.assertEqual(errors, [])
                self.assertEqual(len(receipts), 2 if command == "backup-all" else 1)
                for receipt in receipts:
                    self.assertEqual(receipt["last_success"]["files_transferred"], 4)
                    self.assertEqual(receipt["last_attempt"]["status"], "succeeded")

    def test_ssd_preflight_failures_preserve_success_and_dry_run_history(self):
        self.source.mkdir()
        card_backup = self.root / "card-backup"
        card_backup.mkdir()
        config = SsdConfig(
            source=self.source, destination=self.root / "backup", exclude_file=None
        )
        sd = SdCardConfig(
            source=self.root / "card", destination=card_backup, exclude_file=None
        )
        for step, source in (
            ("SSD: All Photos", self.source),
            ("SSD: SD Card", card_backup),
        ):
            self.history.run(
                step,
                source,
                config.destination / source.name,
                lambda: BackupSummary(step, files_transferred=4),
                dry_run=False,
            )
        previous = {r["step"]: r["last_success"] for r in self.history.read()[0]}
        for seam in ("check_copy_paths", "copy_source_lock", "exclude_from_arg"):
            for dry_run in (True, False):
                with self.subTest(seam=seam, dry_run=dry_run):
                    before = {
                        p: p.read_bytes() for p in self.history.directory.iterdir()
                    }
                    with (
                        mock.patch(
                            f"photos_backup.ssd.backup.{seam}",
                            side_effect=ActionRequired(seam),
                        ),
                        mock.patch("photos_backup.ssd.backup.stream_command") as run,
                        self.assertRaises(ActionRequired),
                    ):
                        SsdBackup(
                            config, True, dry_run, sd_card=sd, history=self.history
                        ).backup()
                    run.assert_not_called()
                    self.assertFalse(config.destination.exists())
                    if dry_run:
                        self.assertEqual(
                            {
                                p: p.read_bytes()
                                for p in self.history.directory.iterdir()
                            },
                            before,
                        )
                    else:
                        receipts, errors = self.history.read()
                        self.assertEqual(errors, [])
                        for receipt in receipts:
                            self.assertEqual(
                                receipt["last_success"], previous[receipt["step"]]
                            )
                            self.assertEqual(
                                receipt["last_attempt"]["status"], "failed"
                            )
                            self.assertIn(
                                f"Preflight: {seam}", receipt["last_attempt"]["error"]
                            )

    def test_sd_card_receipts_cover_both_commands_failures_and_previews(self):
        self.source.mkdir()
        for command in ("sd-card", "backup-all"):
            config = self.root / f"{command}.toml"
            config.write_text(
                f'[sd_card]\nsource = "{self.source}"\ndestination = "{self.root / "backup"}"\n'
            )
            history = TransferHistory(config)
            args = ["--config", str(config), command]
            if command == "backup-all":
                args += ["--skip-apple-photos", "--skip-ssd", "--skip-remote"]
            with (
                mock.patch("photos_backup.cli.backup_all.which", return_value="rsync"),
                mock.patch(
                    "photos_backup.sd_card.backup.stream_command",
                    return_value=subprocess.CompletedProcess(
                        [], 0, stdout="Number of regular files transferred: 4\n"
                    ),
                ) as run,
            ):
                preview = CliRunner().invoke(cli, [*args, "--dry-run"])
                self.assertEqual(preview.exit_code, 0, preview.output)
                self.assertFalse(history.directory.exists())
                result = CliRunner().invoke(cli, args)
                self.assertEqual(result.exit_code, 0, result.output)
                receipts, errors = history.read()
                self.assertEqual(errors, [])
                self.assertEqual(len(receipts), 1)
                receipt = receipts[0]
                self.assertEqual(receipt["step"], "SD Card")
                self.assertEqual(receipt["destination"], str(self.destination))
                self.assertEqual(receipt["last_success"]["files_transferred"], 4)
                before = {p: p.read_bytes() for p in history.directory.iterdir()}
                CliRunner().invoke(cli, [*args, "--dry-run"])
                self.assertEqual(
                    {p: p.read_bytes() for p in history.directory.iterdir()}, before
                )
                run.side_effect = subprocess.CalledProcessError(23, ["rsync"])
                failure = CliRunner().invoke(cli, args)
                self.assertEqual(failure.exit_code, 1, failure.output)
                latest = history.read()[0][0]
                self.assertEqual(latest["last_success"], receipt["last_success"])
                self.assertEqual(latest["last_attempt"]["status"], "failed")

    def test_history_display_uses_success_details_and_relative_ages(self):
        self.run_transfer(
            lambda: BackupSummary(
                "SSD: All Photos",
                files_transferred=0,
                total_size="0 B",
                elapsed_seconds=2.5,
            )
        )
        self.run_transfer(lambda: BackupSummary("SSD: All Photos", error="offline"))
        receipts, errors = self.history.read()
        now = datetime.datetime.fromisoformat(
            receipts[0]["last_success"]["completed_at"]
        ) + datetime.timedelta(days=3)
        with mock.patch("photos_backup.summary.click.echo") as echo:
            print_transfer_history(receipts, errors, now=now)
        output = "\n".join(call.args[0] for call in echo.call_args_list)
        self.assertIn("3 days ago", output)
        self.assertIn("offline", output)
        self.assertIn("0 files, 0 B, 2.5s", output)
        receipts[0]["last_success"].pop("elapsed_seconds")
        with mock.patch("photos_backup.summary.click.echo"):
            print_transfer_history(receipts, errors, now=now)

    def test_invalid_receipt_schema_is_reported(self):
        self.run_transfer()
        path = next(self.history.directory.glob("*.json"))
        original = json.loads(path.read_text())
        for value in (
            [],
            {"version": 99},
            {**original, "last_success": {}},
            {**original, "last_attempt": None},
            {key: value for key, value in original.items() if key != "last_success"},
        ):
            path.write_text(json.dumps(value))
            self.assertEqual(len(self.history.read()[1]), 1)
