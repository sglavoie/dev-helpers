from __future__ import annotations

import contextlib
import sqlite3
import tempfile
import unittest
from pathlib import Path

from photos_backup.apple_photos.adapter import (
    export_db_version_supported,
    read_export_db_assets,
    read_export_db_version,
)
from photos_backup.apple_photos.identity import AssetIdentity
from photos_backup.archive import ArchiveUnsafe
from tests.test_takeover import asset, write_export_db


class ExportDbReaderTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmpdir.cleanup)
        self.root = Path(self._tmpdir.name).resolve()
        self.export_db = self.root / "export.db"

    def test_a_missing_database_reads_as_no_records(self) -> None:
        self.assertEqual(read_export_db_assets(self.export_db), ())
        self.assertIsNone(read_export_db_version(self.export_db))

    def test_current_records_are_read_with_their_cloud_guids(self) -> None:
        write_export_db(self.export_db, (asset("old-1", "guid-1"),))

        self.assertEqual(
            read_export_db_assets(self.export_db),
            (AssetIdentity("old-1", "guid-1", "guid-1.HEIC"),),
        )
        self.assertEqual(read_export_db_version(self.export_db), "11.0")

    def test_a_legacy_info_table_is_read_the_same_way(self) -> None:
        write_export_db(
            self.export_db,
            (asset("old-1", "guid-1"),),
            version="4.3",
            table="info",
            column="json_info",
        )

        self.assertEqual(
            read_export_db_assets(self.export_db),
            (AssetIdentity("old-1", "guid-1", "guid-1.HEIC"),),
        )

    def test_an_unreadable_record_is_unidentifiable_rather_than_fatal(self) -> None:
        write_export_db(self.export_db, ())
        connection = sqlite3.connect(str(self.export_db))
        with contextlib.closing(connection):
            connection.execute(
                "INSERT INTO photoinfo(uuid, photoinfo) VALUES (?, ?)",
                ("old-1", "{not json"),
            )
            connection.commit()

        self.assertEqual(
            read_export_db_assets(self.export_db), (AssetIdentity("old-1", None, ""),)
        )

    def test_a_file_without_asset_records_is_refused_not_read_as_empty(self) -> None:
        self.export_db.write_bytes(b"this is not a database")

        with self.assertRaises(ArchiveUnsafe) as raised:
            read_export_db_assets(self.export_db)

        self.assertIn("no readable asset records", str(raised.exception))

    def test_only_versions_this_osxphotos_understands_are_supported(self) -> None:
        self.assertTrue(export_db_version_supported("11.0"))
        self.assertTrue(export_db_version_supported("4.3"))
        self.assertFalse(export_db_version_supported("12.0"))
        self.assertFalse(export_db_version_supported(None))
        self.assertFalse(export_db_version_supported("not-a-version"))


if __name__ == "__main__":
    unittest.main()
