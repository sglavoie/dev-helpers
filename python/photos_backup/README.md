# osxphotos

Opinionated backup solution for Apple Photos on macOS.

## Goals

1. Back up all photos and videos from Apple Photos to an external drive.
2. Preserve metadata (EXIF, IPTC, etc.) as much as possible.
3. Back up RAW files from SD card to external drive.
4. Back up all of the above to a local directory.
5. Back up all of the above to a cloud storage provider.
6. Do so with incremental backups.

```mermaid
flowchart TD
    A[Apple Photos] --export--> B[Primary on-site backup]
    E[SD Card] --copy--> B
    B --copy--> C[Secondary on-site backup]
    C --upload--> D[Remote backup]
```

## Commands

| Command | Description |
|---------|-------------|
| `cli apple-photos` | Export from Apple Photos to primary backup |
| `cli sd-card` | Copy RAW files from SD card to primary backup |
| `cli ssd` | Sync primary backup to secondary on-site backup (SSD) |
| `cli remote` | Sync backup to cloud via rclone |
| `cli backup-all` | Run the full pipeline (all of the above) |

## Remote backup (rclone)

The `remote` command syncs your backup to a cloud storage provider using
[rclone](https://rclone.org/).

1. Install rclone: `brew install rclone` or see https://rclone.org/install/
2. Configure a remote: `rclone config`
3. Set `RCLONE_REMOTE` in `~/.osxphotos.env` (e.g. `b2:my-photos-bucket`)
4. Optionally set `RCLONE_SRC_PATH` (defaults to `SSD_DST_PATH`)

## Development

### Using uv

```bash
uv venv
uv pip install -e .
uv run cli apple-photos --use-photokit --download-missing
```

### Using built-in Python

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -e .

# Use from elsewhere:
# `which python`
/path/to/.venv/bin/python -m osxphotos.cli.cli
```
