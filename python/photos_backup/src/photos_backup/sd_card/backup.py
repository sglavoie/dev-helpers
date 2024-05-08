import os
import shlex
import subprocess
from pathlib import Path


class Backup:
    def __init__(self, dry_run: bool) -> None:
        self.dry_run = dry_run
        self.src_path: Path = Path()
        self.dst_path: Path = Path()
        self.exclude_file: Path | None = Path()
        self._set_up_paths()

    def backup(self) -> None:
        dry_run = "--dry-run" if self.dry_run else ""
        exclude = "" if not self.exclude_file else f"--exclude-from={self.exclude_file}"
        cmd = f"""rsync -a --progress \
            {dry_run} \
            {exclude} \
            {self.src_path} {self.dst_path}"""
        subprocess.run(shlex.split(cmd), check=True)

    def _set_up_paths(self) -> None:
        src_path = "SD_CARD_SRC_PATH"
        dst_path = "SD_CARD_DST_PATH"
        exclude_file = "SD_CARD_EXCLUDE_FILE"
        sd_card_src_path = os.getenv(src_path, "")
        sd_card_dst_path = os.getenv(dst_path, "")
        sd_card_exclude_file = os.getenv(exclude_file, "")

        for env_var_name, env_var_value in {
            src_path: {"type": "source", "path": sd_card_src_path},
            dst_path: {"type": "destination", "path": sd_card_dst_path},
            exclude_file: {"type": "exclude_file", "path": sd_card_exclude_file},
        }.items():
            if not env_var_value:
                raise ValueError(f"'{env_var_name}' is not set")

            if not os.path.exists(env_var_value.get("path", "")):
                if env_var_value.get("type") == "source":
                    raise FileNotFoundError(
                        f"{env_var_name}={env_var_value} does not exist"
                    )

                if env_var_value.get("type") == "destination":
                    try:
                        os.makedirs(env_var_value.get("path", ""), exist_ok=True)
                    except OSError as e:
                        raise OSError(
                            f"Could not create {env_var_value.get('type')} directory: {env_var_value.get('path')}"
                        ) from e

        self.exclude_file = (
            Path(sd_card_exclude_file) if os.path.exists(sd_card_exclude_file) else None
        )
        self.src_path = Path(sd_card_src_path)
        self.dst_path = Path(sd_card_dst_path)
