import datetime
import json
import subprocess
import tempfile
import unittest
from itertools import product
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

    def test_cli_records_ssd_and_remote_results_in_standalone_and_pipeline_runs(self):
        self.source.mkdir()
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
        with mock.patch("photos_backup.summary.click.echo") as echo:
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
        with mock.patch("photos_backup.summary.click.echo") as echo:
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

    def test_copy_only_status_reads_receipts_without_opening_archive_or_writing(self):
        self.config.write_text(
            f'[ssd]\nsource = "{self.source}"\ndestination = "{self.destination.parent}"\n'
        )
        for recorded in (False, True):
            if recorded:
                self.run_transfer()
            before = {p: p.read_bytes() for p in self.root.rglob("*") if p.is_file()}
            for as_json in (False, True):
                with (
                    self.subTest(recorded=recorded, as_json=as_json),
                    mock.patch("photos_backup.cli.status.open_archive") as open_archive,
                ):
                    result = CliRunner().invoke(
                        cli,
                        ["--config", str(self.config), "status"]
                        + (["--json"] if as_json else []),
                    )
                    self.assertEqual(result.exit_code, 0, result.output)
                    open_archive.assert_not_called()
                    if as_json:
                        document = json.loads(result.stdout)
                        self.assertFalse(document["archive_configured"])
                        for field in (
                            "archive",
                            "state",
                            "next_export",
                            "archive_error",
                        ):
                            self.assertIsNone(document[field])
                        self.assertEqual(len(document["transfers"]), int(recorded))
                        self.assertEqual(document["historical_transfers"], [])
                        self.assertEqual(len(document["configured_transfers"]), 1)
                        self.assertEqual(
                            document["configured_transfers"][0]["last_success"]
                            is not None,
                            recorded,
                        )
                    else:
                        self.assertIn("Archive status: not configured", result.stdout)
                        self.assertIn("Transfer history", result.stdout)
                        self.assertIn(
                            "SSD: All Photos" if recorded else "No transfer receipts",
                            result.stdout,
                        )
            self.assertEqual(
                {p: p.read_bytes() for p in self.root.rglob("*") if p.is_file()}, before
            )
            if not recorded:
                self.assertFalse(self.history.directory.exists())

    def test_status_does_not_hide_missing_invalid_or_malformed_configuration(self):
        for content in (
            None,
            "[broken",
            "[apple_photos]\n",
            "apple_photos = 3\n",
            "[ssd]\n",
            "[sd_card]\n",
            '[rclone]\nremote = "b2:photos"\n',
        ):
            with self.subTest(content=content):
                if content is not None:
                    self.config.write_text(content)
                result = CliRunner().invoke(
                    cli, ["--config", str(self.config), "status", "--json"]
                )
                self.assertEqual(result.exit_code, 2, result.output)
                self.assertNotIn("archive_configured", result.stdout)
                self.assertFalse(self.history.directory.exists())

    def test_status_matches_all_configured_routes_without_probing_copy_paths(self):
        card = self.root / "card"
        raw = self.root / "raw"
        backup = self.root / "backup"
        self.config.write_text(
            f'[sd_card]\nsource = "{card}"\ndestination = "{raw}"\n'
            f'[ssd]\nsource = "{self.source}"\ndestination = "{backup}"\n'
            '[rclone]\nremote = "b2:photos"\n'
        )
        routes = [
            ("SD Card", card, raw / card.name),
            ("SSD: All Photos", self.source, backup / self.source.name),
            ("SSD: SD Card", raw, backup / raw.name),
            ("Remote", backup, "b2:photos"),
        ]
        for step, source, destination in routes:
            self.history.run(
                step, source, destination, lambda: BackupSummary(step), dry_run=False
            )
        before = {p: p.read_bytes() for p in self.history.directory.iterdir()}
        result = CliRunner().invoke(
            cli, ["--config", str(self.config), "status", "--json"]
        )
        self.assertEqual(result.exit_code, 0, result.output)
        document = json.loads(result.stdout)
        self.assertEqual(document["historical_transfers"], [])
        self.assertEqual(len(document["configured_transfers"]), 4)
        for row in document["configured_transfers"]:
            self.assertIsNotNone(row["last_success"])
        self.assertEqual(
            {p: p.read_bytes() for p in self.history.directory.iterdir()}, before
        )
        self.assertFalse(backup.exists())

    def test_status_separates_changed_sources_destinations_and_removed_routes(self):
        self.run_transfer(lambda: BackupSummary("SSD: All Photos", error="old failure"))
        for content in (
            f'[ssd]\nsource = "{self.source}"\ndestination = "{self.root / "new"}"\n',
            f'[ssd]\nsource = "{self.root / "new/photos"}"\ndestination = "{self.root / "backup"}"\n',
            '[rclone]\nsource = "/offline/photos"\nremote = "b2:new"\n',
            "",
        ):
            with self.subTest(content=content):
                self.config.write_text(content)
                args = ["--config", str(self.config), "status"]
                result = CliRunner().invoke(cli, [*args, "--json"])
                self.assertEqual(result.exit_code, 0, result.output)
                document = json.loads(result.stdout)
                self.assertEqual(
                    document["historical_transfers"], document["transfers"]
                )
                self.assertEqual(
                    len(document["configured_transfers"]), int(bool(content))
                )
                for row in document["configured_transfers"]:
                    self.assertIsNone(row["last_attempt"])
                    self.assertIsNone(row["last_success"])
                rendered = CliRunner().invoke(cli, args).stdout
                self.assertIn(
                    "Historical destinations (not currently configured)", rendered
                )
                self.assertNotIn("latest attempt failed", rendered)
                if content:
                    self.assertIn("no successful copy recorded", rendered)
                else:
                    self.assertIn("No configured transfer destinations", rendered)

    def test_attention_summary_distinguishes_failure_interruption_and_unknown_completion(
        self,
    ):
        self.run_transfer()
        receipt = self.receipt()
        for status in ("succeeded", "failed", "interrupted", "started"):
            for never_succeeded in (False, True):
                with self.subTest(status=status, never_succeeded=never_succeeded):
                    row = {
                        **receipt,
                        "last_attempt": {**receipt["last_attempt"], "status": status},
                        "last_success": None
                        if never_succeeded
                        else receipt["last_success"],
                    }
                    with mock.patch("photos_backup.summary.click.echo") as echo:
                        print_transfer_history([row], [])
                    lines = [call.args[0] for call in echo.call_args_list]
                    attention = [line for line in lines if "Attention:" in line]
                    if status == "succeeded" and not never_succeeded:
                        self.assertEqual(attention, [])
                        continue
                    self.assertEqual(len(attention), 1)
                    if status == "started":
                        self.assertIn(
                            "completion not recorded (running or interrupted)",
                            attention[0],
                        )
                    elif status != "succeeded":
                        self.assertIn(f"latest attempt {status}", attention[0])
                    self.assertEqual(
                        "no successful copy recorded" in attention[0], never_succeeded
                    )
