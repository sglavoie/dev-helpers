import datetime
import json
import subprocess
from itertools import product
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.config import SdCardConfig, SsdConfig
from photos_backup.errors import ActionRequired
from photos_backup.ssd.backup import Backup as SsdBackup
from photos_backup.summary import BackupSummary
from photos_backup.status_report import print_transfer_history
from photos_backup.transfers import TransferHistory
from tests.transfer_history_case import TransferHistoryTestCase


class TransferReceiptsCliTests(TransferHistoryTestCase):
    def test_cli_records_ssd_and_remote_results_in_standalone_and_pipeline_runs(self):
        self.source.mkdir()
        (self.source / "IMG_0001.JPG").write_text("photo")
        for command, delete in product(("ssd", "remote", "backup-all"), (False, True)):
            with self.subTest(command=command, delete=delete):
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
                    if delete:
                        args.append("--delete")
                        if command == "backup-all":
                            args.append("--delete-remote")
                    result = CliRunner().invoke(cli, args)
                self.assertEqual(result.exit_code, 0, result.output)
                receipts, errors = TransferHistory(config).read()
                self.assertEqual(errors, [])
                self.assertEqual(len(receipts), 2 if command == "backup-all" else 1)
                for receipt in receipts:
                    self.assertEqual(receipt["last_success"]["files_transferred"], 4)
                    self.assertEqual(receipt["last_attempt"]["status"], "succeeded")
                    self.assertEqual(
                        receipt["last_success"]["mode"], "mirror" if delete else "copy"
                    )

    def test_failed_mirror_retains_successful_copy_mode_and_legacy_is_unknown(self):
        self.history.run(
            "Remote",
            self.source,
            "b2:photos",
            lambda: BackupSummary("Remote"),
            dry_run=False,
            delete_at_destination=False,
        )
        self.history.run(
            "Remote",
            self.source,
            "b2:photos",
            lambda: BackupSummary("Remote", error="offline"),
            dry_run=False,
            delete_at_destination=True,
        )
        receipt = self.receipt()
        self.assertEqual(receipt["last_success"]["mode"], "copy")
        self.assertEqual(receipt["last_attempt"]["mode"], "mirror")
        with mock.patch("photos_backup.status_report.click.echo") as echo:
            print_transfer_history([receipt], [])
        output = "\n".join(str(call.args[0]) for call in echo.call_args_list)
        self.assertIn("Attempt mode: mirror (deletions enabled)", output)
        self.assertIn(
            "Successful copy mode: copy (preserves destination-only files)", output
        )
        path = next(self.history.directory.glob("*.json"))
        for field in ("last_attempt", "last_success"):
            receipt[field].pop("mode")
        path.write_text(json.dumps(receipt))
        with mock.patch("photos_backup.status_report.click.echo") as echo:
            print_transfer_history([self.receipt()], [])
        self.assertEqual(
            sum(
                "mode not recorded" in str(call.args[0]) for call in echo.call_args_list
            ),
            2,
        )

    def test_preflight_failure_records_requested_mode(self):
        self.history.record_preflight_failure(
            "SSD: All Photos",
            self.source,
            self.destination,
            ActionRequired("offline"),
            started_at=datetime.datetime.now(datetime.UTC).isoformat(),
            dry_run=False,
            delete_at_destination=True,
        )
        self.assertEqual(self.receipt()["last_attempt"]["mode"], "mirror")

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
                self.assertEqual(receipt["last_success"]["mode"], "copy")
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
