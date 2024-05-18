import os
import shlex
import subprocess
from pathlib import Path


class Backup:
    def __init__(self, dry_run: bool) -> None:
        self.dry_run = dry_run
        self.src_path: Path = Path()
        self.dst_path: Path = Path()
        self._set_up_paths()

    def backup(self) -> None:
        dry_run = "--dry-run" if self.dry_run else ""
        cmd = f"""rsync -a --progress \
            {dry_run} \
            {self.src_path} {self.dst_path}"""
        subprocess.run(shlex.split(cmd), check=True)

    def _set_up_paths(self) -> None:
        src_path = "SDD_SRC_PATH"
        dst_path = "SDD_DST_PATH"
        sdd_src_path = os.getenv(src_path, "")
        sdd_dst_path = os.getenv(dst_path, "")

        for env_var_name, env_var_value in {
            src_path: {"type": "source", "path": sdd_src_path},
            dst_path: {"type": "destination", "path": sdd_dst_path},
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

        self.src_path = Path(sdd_src_path)
        self.dst_path = Path(sdd_dst_path)
