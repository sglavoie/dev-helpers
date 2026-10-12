import datetime
import json
from unittest import mock

from click.testing import CliRunner

from photos_backup.cli.cli import cli
from photos_backup.summary import BackupSummary
from photos_backup.status_report import print_transfer_history
from tests.transfer_history_case import TransferHistoryTestCase


class TransferStatusTests(TransferHistoryTestCase):
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
        with mock.patch("photos_backup.status_report.click.echo") as echo:
            print_transfer_history(receipts, errors, now=now)
        output = "\n".join(call.args[0] for call in echo.call_args_list)
        self.assertIn("3 days ago", output)
        self.assertIn("offline", output)
        self.assertIn("0 files, 0 B, 2.5s", output)
        receipts[0]["last_success"].pop("elapsed_seconds")
        with mock.patch("photos_backup.status_report.click.echo"):
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
                    with mock.patch("photos_backup.status_report.click.echo") as echo:
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
