import os
import shlex
import subprocess
from pathlib import Path


class Backup:
    def __init__(self, dry_run: bool) -> None:
        self.dry_run = dry_run
        self.apple_photos_path: Path = Path()
        self.sd_card_path: Path = Path()
        self.sdd_dst_path: Path = Path()
        self._set_up_paths()

    def backup(self) -> None:
        dry_run = "--dry-run" if self.dry_run else ""
        cmd = f"""rsync -a --progress \
            {dry_run} \
            {self.apple_photos_path} {self.sdd_dst_path}"""
        subprocess.run(shlex.split(cmd), check=True)

        cmd = f"""rsync -a --progress \
            {dry_run} \
            {self.sd_card_path} {self.sdd_dst_path}"""
        subprocess.run(shlex.split(cmd), check=True)

    def _set_up_paths(self) -> None:
        apple_photos_path_env = "APPLE_PHOTOS_DST_PATH"
        apple_photos_path = os.getenv(apple_photos_path_env, "")
        sd_card_path_env = "SD_CARD_DST_PATH"
        sd_card_path = os.getenv(sd_card_path_env, "")
        sdd_dst_path_env = "SDD_DST_PATH"
        sdd_dst_path = os.getenv(sdd_dst_path_env, "")

        for path_env, path_value in zip(
            [apple_photos_path_env, sd_card_path_env, sdd_dst_path_env],
            [apple_photos_path, sd_card_path, sdd_dst_path],
        ):
            if not path_value:
                raise ValueError(f"'{path_env}' is not set")

        for path in [apple_photos_path, sd_card_path]:
            if not os.path.exists(path):
                raise FileNotFoundError(f"'{path}' does not exist")

        if not os.path.exists(sdd_dst_path):
            try:
                os.makedirs(sdd_dst_path, exist_ok=True)
            except OSError as e:
                raise OSError(
                    f"Could not create {sdd_dst_path_env} directory: {sdd_dst_path}"
                ) from e

        self.apple_photos_path = Path(apple_photos_path)
        self.sd_card_path = Path(sd_card_path)
        self.sdd_dst_path = Path(sdd_dst_path)
