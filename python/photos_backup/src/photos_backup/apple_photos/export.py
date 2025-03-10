import os
from pathlib import Path

from osxphotos.cli.export import export_cli


class ApplePhotosExport:
    def __init__(self, dl_missing: bool, testing: bool):
        self.dl_missing = dl_missing

        # Set a couple of flags when testing
        self.testing = testing
        self.limit = int(os.getenv("APPLE_PHOTOS_LIMIT_EXPORT", "0")) if testing else 0

        self.dst_path: Path = Path()
        self._set_up_paths()

    def export(self):
        export_cli(
            # Testing flags
            dry_run=self.testing,
            verbose_flag=self.testing,
            limit=self.limit,
            # Regular flags
            dest=str(self.dst_path),
            download_missing=self.dl_missing,
            exiftool=True,
            directory="{created.year}/{created.mm}/{album[ ,_],}",
            filename_template="{created.strftime,%Y-%m-%d-%H%M%S}_{original_name}",
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
