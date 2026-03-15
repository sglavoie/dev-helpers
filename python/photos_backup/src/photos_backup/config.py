import os
from dataclasses import dataclass
from pathlib import Path

import click
from dotenv import load_dotenv


@dataclass
class Config:
    apple_photos_dst_path: Path
    apple_photos_limit_export: int
    sd_card_src_path: Path
    sd_card_dst_path: Path
    sd_card_exclude_file: Path | None
    all_photos_path: Path
    all_photos_exclude_file: Path | None
    ssd_dst_path: Path
    rclone_remote: str
    rclone_src_path: Path | None

    @classmethod
    def from_env(cls) -> "Config":
        env_path = Path.home() / ".osxphotos.env"
        if not env_path.exists():
            raise click.UsageError(f"Could not find {env_path}")

        load_dotenv(env_path)

        def get_required(name: str) -> str:
            value = os.getenv(name, "")
            if not value:
                raise click.UsageError(f"'{name}' is not set in {env_path}")
            return value

        apple_photos_dst_path = Path(get_required("APPLE_PHOTOS_DST_PATH"))
        apple_photos_limit_export = int(os.getenv("APPLE_PHOTOS_LIMIT_EXPORT", "0"))
        sd_card_src_path = Path(get_required("SD_CARD_SRC_PATH"))
        sd_card_dst_path = Path(get_required("SD_CARD_DST_PATH"))
        all_photos_path = Path(get_required("ALL_PHOTOS_PATH"))
        ssd_dst_path = Path(get_required("SSD_DST_PATH"))

        # Resolve exclude files — None if file doesn't exist on disk
        sd_card_exclude_raw = os.getenv("SD_CARD_EXCLUDE_FILE", "")
        sd_card_exclude_file = (
            Path(sd_card_exclude_raw)
            if sd_card_exclude_raw and Path(sd_card_exclude_raw).exists()
            else None
        )

        all_photos_exclude_raw = os.getenv("ALL_PHOTOS_EXCLUDE_FILE", "")
        all_photos_exclude_file = (
            Path(all_photos_exclude_raw)
            if all_photos_exclude_raw and Path(all_photos_exclude_raw).exists()
            else None
        )

        rclone_remote = os.getenv("RCLONE_REMOTE", "")
        rclone_src_raw = os.getenv("RCLONE_SRC_PATH", "")
        rclone_src_path = Path(rclone_src_raw) if rclone_src_raw else None

        return cls(
            apple_photos_dst_path=apple_photos_dst_path,
            apple_photos_limit_export=apple_photos_limit_export,
            sd_card_src_path=sd_card_src_path,
            sd_card_dst_path=sd_card_dst_path,
            sd_card_exclude_file=sd_card_exclude_file,
            all_photos_path=all_photos_path,
            all_photos_exclude_file=all_photos_exclude_file,
            ssd_dst_path=ssd_dst_path,
            rclone_remote=rclone_remote,
            rclone_src_path=rclone_src_path,
        )
