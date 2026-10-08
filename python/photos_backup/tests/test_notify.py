from __future__ import annotations

import subprocess
import unittest
from unittest import mock

import click
from click.testing import CliRunner

from photos_backup.cli.notify import notify_on_problems, post_notification
from photos_backup.errors import ActionRequired
from tests.transfer_case import TransferTestCase


def command_raising(error: BaseException | None) -> click.Command:
    @click.command(name="daily")
    @notify_on_problems
    def daily() -> None:
        if error is not None:
            raise error

    return daily


class NotifyOnProblemsTests(unittest.TestCase):
    def setUp(self) -> None:
        self.post = self.enterContext(
            mock.patch("photos_backup.cli.notify.post_notification")
        )

    def invoke(self, error: BaseException | None, *arguments: str):
        return CliRunner().invoke(command_raising(error), list(arguments))

    def test_a_run_that_needs_a_person_notifies_and_keeps_exit_three(self):
        result = self.invoke(ActionRequired("cleanup awaits approval"), "--notify")

        self.assertEqual(result.exit_code, 3)
        self.post.assert_called_once_with(
            "photos-backup daily needs you", "cleanup awaits approval"
        )

    def test_a_failed_run_notifies_and_keeps_exit_one(self):
        result = self.invoke(click.ClickException("osxphotos failed"), "--notify")

        self.assertEqual(result.exit_code, 1)
        self.post.assert_called_once_with(
            "photos-backup daily failed", "osxphotos failed"
        )

    def test_an_unexpected_error_still_notifies(self):
        result = self.invoke(RuntimeError("disk vanished"), "--notify")

        self.assertEqual(result.exit_code, 1)
        self.post.assert_called_once_with("photos-backup daily failed", "disk vanished")

    def test_an_interrupted_run_notifies(self):
        result = self.invoke(KeyboardInterrupt(), "--notify")

        self.assertEqual(result.exit_code, 1)
        self.post.assert_called_once_with(
            "photos-backup daily interrupted", "Stopped before finishing."
        )

    def test_success_and_runs_without_the_flag_stay_silent(self):
        self.assertEqual(self.invoke(None, "--notify").exit_code, 0)
        self.assertEqual(self.invoke(ActionRequired("x")).exit_code, 3)
        self.assertEqual(self.invoke(click.exceptions.Exit(0), "--notify").exit_code, 0)

        self.post.assert_not_called()


class PostNotificationTests(unittest.TestCase):
    def test_text_is_passed_as_arguments_and_long_messages_are_shortened(self):
        with mock.patch("photos_backup.cli.notify.subprocess.run") as run:
            post_notification('Title "quoted"', "x" * 500)

        command = run.call_args.args[0]
        self.assertEqual(command[0], "osascript")
        self.assertEqual(command[-2], 'Title "quoted"')
        self.assertEqual(len(command[-1]), 240)
        self.assertTrue(command[-1].endswith("…"))

    def test_a_missing_or_hung_osascript_is_ignored(self):
        for error in (
            FileNotFoundError("osascript"),
            subprocess.TimeoutExpired("osascript", 10),
        ):
            with (
                self.subTest(error=type(error).__name__),
                mock.patch(
                    "photos_backup.cli.notify.subprocess.run", side_effect=error
                ),
            ):
                post_notification("title", "message")


class PipelineNotifyTests(TransferTestCase):
    def test_backup_all_accepts_notify_and_stays_silent_when_nothing_fails(self):
        self.config.write_text("")
        with mock.patch("photos_backup.cli.notify.post_notification") as post:
            result = self.invoke("backup-all", "--skip-apple-photos", "--notify")

        self.assertEqual(result.exit_code, 0, result.output)
        post.assert_not_called()


if __name__ == "__main__":
    unittest.main()
