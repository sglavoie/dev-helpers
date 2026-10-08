from __future__ import annotations

import unittest

import click

from photos_backup.cli.outcome import raise_for_summaries
from photos_backup.errors import ActionRequired
from photos_backup.summary import BackupSummary
from tests.transfer_case import TransferTestCase

FAILED = BackupSummary(step_name="SSD", error="rsync exited with code 23")
NEEDS_PERSON = BackupSummary(
    step_name="SD Card", error="rename the archived copies", action_required=True
)


class RaiseForSummariesTests(unittest.TestCase):
    def test_clean_and_skipped_steps_exit_zero(self):
        raise_for_summaries(
            [BackupSummary(step_name="SSD"), BackupSummary("Remote", skipped=True)]
        )

    def test_a_failure_outranks_a_step_that_needs_a_person(self):
        with self.assertRaises(click.ClickException) as raised:
            raise_for_summaries([NEEDS_PERSON, FAILED])

        self.assertNotIsInstance(raised.exception, ActionRequired)
        self.assertEqual(raised.exception.message, "rsync exited with code 23")

    def test_a_pipeline_names_its_failed_steps(self):
        with self.assertRaises(click.ClickException) as raised:
            raise_for_summaries([FAILED], name_failed_steps=True)

        self.assertEqual(raised.exception.message, "Step(s) failed: SSD")

    def test_steps_that_need_a_person_exit_three_together(self):
        with self.assertRaises(ActionRequired) as raised:
            raise_for_summaries(
                [
                    NEEDS_PERSON,
                    BackupSummary("Remote", error="mount", action_required=True),
                ]
            )

        self.assertEqual(raised.exception.exit_code, 3)
        self.assertEqual(raised.exception.message, "rename the archived copies; mount")


class BackupAllTimingTests(TransferTestCase):
    def test_backup_all_reports_total_command_time(self):
        self.config.write_text("")

        result = self.invoke("backup-all", "--skip-apple-photos")

        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("NOTHING TO DO", result.output)
        self.assertIn("Total command time:", result.output)


if __name__ == "__main__":
    unittest.main()
