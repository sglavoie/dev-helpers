import os
from pathlib import Path

from osxphotos.cli.export import export_cli


class ApplePhotosExport:
    def __init__(self, dry_run: bool):
        self.dry_run = dry_run
        self.dst_path: Path = Path()
        self._set_up_paths()

    def export(self):
        export_cli(
            dest=str(self.dst_path),
            dry_run=self.dry_run,
            exiftool=True,
            export_by_date=True,
            report="photos_export_{today.date}.csv",
            update=True,
        )

    def _set_up_paths(self) -> None:
        dst_path = "APPLE_PHOTOS_DST_PATH"
        apple_photos_dst_path = os.getenv(dst_path, "")

        if not apple_photos_dst_path:
            raise ValueError(f"'{dst_path}' is not set")

        if not os.path.exists(apple_photos_dst_path):
            try:
                os.makedirs(apple_photos_dst_path)
            except OSError as e:
                raise OSError(
                    f"Could not create directory {dst_path}={apple_photos_dst_path}"
                ) from e

        self.dst_path = Path(apple_photos_dst_path)
