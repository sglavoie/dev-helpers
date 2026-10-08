import datetime
import unittest

import click
from click.testing import CliRunner

from photos_backup.cli.apple_photos import _parse_extra_args
from photos_backup.cli.cli import cli


class ForwardedOsxphotosOptionTests(unittest.TestCase):
    def test_options_are_typed_by_osxphotos(self):
        self.assertEqual(
            _parse_extra_args(
                [
                    "--album",
                    "2024",
                    "--album=Holiday",
                    "--from-date",
                    "2024-01-02",
                    "--limit",
                    "5",
                    "-V",
                    "-V",
                    "--use-photokit",
                    "--sidecar",
                    "xmp",
                ]
            ),
            {
                "album": ("2024", "Holiday"),
                "from_date": datetime.datetime(2024, 1, 2),
                "limit": 5,
                "verbose_flag": 2,
                "use_photokit": True,
                "sidecar": ("xmp",),
            },
        )

    def test_only_given_options_are_returned(self):
        self.assertEqual(_parse_extra_args([]), {})
        self.assertEqual(_parse_extra_args(["--only-new"]), {"only_new": True})

    def test_invalid_options_are_usage_errors(self):
        for arguments, message in (
            (["--bogus"], "No such option"),
            (["--limit", "many"], "not a valid integer"),
            (["--limit"], "requires an argument"),
        ):
            with self.subTest(arguments=arguments):
                with self.assertRaises(click.UsageError) as raised:
                    _parse_extra_args(arguments)
                self.assertIn("osxphotos export:", raised.exception.format_message())
                self.assertIn(message, raised.exception.format_message())
        with self.assertRaisesRegex(click.UsageError, "Unexpected argument: photos"):
            _parse_extra_args(["photos"])

    def test_aliases_of_archive_managed_options_are_refused(self):
        with self.assertRaises(click.UsageError) as raised:
            _parse_extra_args(["--library", "/elsewhere.photoslibrary"])
        self.assertIn("Archive-managed option(s)", raised.exception.format_message())
        self.assertIn("--db", raised.exception.format_message())


class ApplePhotosHelpTests(unittest.TestCase):
    def test_usage_shows_forwarded_options_and_an_example(self):
        result = CliRunner().invoke(cli, ["apple-photos", "--help"])
        self.assertEqual(result.exit_code, 0, result.output)
        self.assertIn("[OSXPHOTOS EXPORT OPTIONS]...", result.output)
        self.assertIn("apple-photos --album Holiday", result.output)


if __name__ == "__main__":
    unittest.main()
