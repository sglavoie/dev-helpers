import os
import shlex
import subprocess
from pathlib import Path


class Backup:
    def __init__(self, delete_at_destination: bool, dry_run: bool) -> None:
        self.delete_at_destination = delete_at_destination
        self.dry_run = dry_run
        self.all_photos_path_str: str = ""
        self.all_photos_exclude_file: Path | None = Path()
        self.apple_photos_path_str: str = ""
        self.sd_card_path_str: str = ""
        self.sd_card_exclude_file: Path | None = Path()
        self.ssd_dst_path_str: str = ""
        self._set_up_paths()

    def backup(self) -> None:
        dry_run = "--dry-run" if self.dry_run else ""
        delete = "--delete" if self.delete_at_destination else ""
        exclude_all_photos = (
            ""
            if not self.all_photos_exclude_file
            else f"--exclude-from={self.all_photos_exclude_file}"
        )
        exclude_sd_card = (
            ""
            if not self.sd_card_exclude_file
            else f"--exclude-from={self.sd_card_exclude_file}"
        )

        if self.all_photos_path_str:
            cmd = f"""rsync -avh --progress {delete} \
                {dry_run} \
                {exclude_all_photos} \
                {self.all_photos_path_str} {self.ssd_dst_path_str}"""
            subprocess.run(shlex.split(cmd), check=True)

        if self.apple_photos_path_str:
            cmd = f"""rsync -avh --progress {delete} \
                {dry_run} \
                {self.apple_photos_path_str} {self.ssd_dst_path_str}"""
            subprocess.run(shlex.split(cmd), check=True)

        if self.sd_card_path_str:
            cmd = f"""rsync -avh --progress {delete} \
                {dry_run} \
                {exclude_sd_card} \
                {self.sd_card_path_str} {self.ssd_dst_path_str}"""
            subprocess.run(shlex.split(cmd), check=True)

    def _set_up_paths(self) -> None:
        all_photos_path_env = "ALL_PHOTOS_PATH"
        all_photos_path = os.getenv(all_photos_path_env, "")
        all_photos_exclude_file_env = "ALL_PHOTOS_EXCLUDE_FILE"
        all_photos_exclude_file_path = os.getenv(all_photos_exclude_file_env, "")
        apple_photos_path_env = "APPLE_PHOTOS_DST_PATH"
        apple_photos_path = os.getenv(apple_photos_path_env, "")
        sd_card_path_env = "SD_CARD_DST_PATH"
        sd_card_path = os.getenv(sd_card_path_env, "")
        sd_card_exclude_file_env = "SD_CARD_EXCLUDE_FILE"
        sd_card_exclude_file = os.getenv(sd_card_exclude_file_env, "")
        ssd_dst_path_env = "SSD_DST_PATH"
        ssd_dst_path = os.getenv(ssd_dst_path_env, "")

        for path_env, path_value in zip(
            [
                all_photos_path_env,
                apple_photos_path_env,
                sd_card_path_env,
                ssd_dst_path_env,
            ],
            [all_photos_path, apple_photos_path, sd_card_path, ssd_dst_path],
        ):
            if not path_value:
                raise ValueError(f"'{path_env}' is not set")

        if not os.path.exists(ssd_dst_path):
            try:
                os.makedirs(ssd_dst_path, exist_ok=True)
            except OSError as e:
                raise OSError(
                    f"Could not create {ssd_dst_path_env} directory: {ssd_dst_path}"
                ) from e

        if not os.path.exists(all_photos_path):
            self._print_skip_dir(all_photos_path)
        else:
            self.all_photos_path_str = all_photos_path
            self.all_photos_exclude_file = (
                Path(all_photos_exclude_file_path)
                if os.path.exists(all_photos_exclude_file_path)
                else None
            )
        if not os.path.exists(apple_photos_path):
            self._print_skip_dir(apple_photos_path)
        else:
            self.apple_photos_path_str = apple_photos_path

        if not os.path.exists(sd_card_path):
            self._print_skip_dir(sd_card_path)
        else:
            self.sd_card_path_str = sd_card_path
            self.sd_card_exclude_file = (
                Path(sd_card_exclude_file)
                if os.path.exists(sd_card_exclude_file)
                else None
            )

        self.ssd_dst_path_str = ssd_dst_path

    def _print_skip_dir(self, path: str) -> None:
        print(f"'{path}' does not exist: skipping")
