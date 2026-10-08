# Deployment and migration

[Back to the user guide](../README.md)

## Stow-managed configuration

The live configuration is not written by hand. It is a stow package in the
dotfiles repository:

```
~/dotfiles/osxphotos-backup/.config/osxphotos-backup/photos-backup.toml
```

`osxphotos-backup` is listed in the dotfiles `justfile`, so `just all` and
`just delete` cover it like every other package. To deploy it alone:

```bash
cd ~/dotfiles
stow --no --verbose --target=$HOME osxphotos-backup   # simulate first
stow --verbose --target=$HOME osxphotos-backup        # then link
```

`~/.config/osxphotos-backup` becomes a symlink into the repository, so editing
the file in `~/dotfiles` is the same as editing the live configuration. Update
it there, commit, and `git pull` on the other Mac; nothing needs to be restowed
for a content change, only for a new package.

Both Macs share `apple_photos.volume` and `apple_photos.archive` because the
archive travels with the drive. `library`, `[sd_card]`, and `[ssd]` are the
per-Mac lines: a Mac without an SD reader or a second drive simply omits those
sections, and the commands that need them report themselves as skipped.

## Running it from goback

`goback run daily` runs this tool as a warning-only companion after its own
rsync. Add one entry to the profile in `~/.goback.json`:

```json
"dailyCompanions": [
  {
    "id": "apple-photos",
    "name": "Apple Photos",
    "command": ["photos-backup", "daily"],
    "dryRunArgs": ["--dry-run"]
  }
]
```

`command` is the program followed by each argument as a separate item; a single
shell string is rejected, because companions are executed without a shell.
`photos-backup` is resolved through `PATH`, so the editable install above is
what makes the entry work — reinstall with `uv tool install --force --editable .`
on a machine that only has the older `cli` entry point.

The companion never changes the exit status of `goback run daily`: the rsync
result alone decides it, while both outcomes are printed in one table and
recorded in the goback history as `daily` and `companion/apple-photos`. A
`goback run daily --dry-run` appends `--dry-run` here too, so a dry run can
never export for real.

## First run on a new Mac

1. Install the tool, as above.
2. Clone the dotfiles repository and stow `osxphotos-backup`, then check
   `apple_photos.library` and the `[sd_card]`/`[ssd]` sections against what this
   Mac actually has.
3. Mount the archive drive.
4. Run `photos-backup bootstrap --dry-run` to see the export it would run, then
   `photos-backup bootstrap` until it prints `Archive initialized`. On a Mac that
   joins an archive another Mac already initialized, run
   `photos-backup daily --dry-run` instead and resolve the takeover it reports.
5. Run `photos-backup verify` and fix anything that fails.
6. Add the `dailyCompanions` entry above to that Mac's goback profile, or
   schedule `photos-backup daily` directly, treating exit 3 as a notification
   rather than an alarm.
7. Only once all of that holds, run `photos-backup cleanup-local-export` to
   retire `~/Pictures/export`.

The two Macs write the same archive one at a time. Whoever mounts the drive next
runs `daily`, which proves the library matches before writing and repoints the
export database at it if needed; the other Mac does nothing until it has the
drive. Deletions are the exception: only the Mac that currently owns the archive
may run `approve-cleanup`, though either may `--discard` a pending run.

## Manual acceptance

These are the checks a person runs once, in this order, and they are deliberately
not automated:

| Step | What it proves |
|------|----------------|
| `photos-backup verify` with the drive mounted | The archive is readable, claimed, and has no pending cleanup |
| `photos-backup daily --dry-run` | The cadence, the takeover, and the export are what you expect, with nothing written |
| `goback preview daily` | The companion is configured and shows the exact real and dry-run argument vectors |
| `goback run daily --dry-run` | Both steps run end to end, nothing is recorded, and the companion gets `--dry-run` |
| `goback run daily` | The table reports both statuses and the shell exit status still follows rsync alone |
| `goback usage last` | `daily` and `companion/apple-photos` appear as separate rows |

`photos-backup bootstrap`, `approve-cleanup`, and `cleanup-local-export` are
kept out of scheduled runs: bootstrap is a one-time decision, `approve-cleanup`
applies the explicitly named manifest without a confirmation prompt, and
`cleanup-local-export` requires confirmation at a terminal. `sd-card` and
`remote` stay manual as well — the SD card is only ever plugged in by hand, and
the remote sync is a separate cost and bandwidth decision.

## Migrating from `~/.osxphotos.env`

`~/.osxphotos.env` and `python-dotenv` are no longer used. Move the values into
the TOML file as follows:

| Environment variable | TOML key |
|----------------------|----------|
| `APPLE_PHOTOS_DST_PATH` | `apple_photos.legacy_export` |
| `APPLE_PHOTOS_LIMIT_EXPORT` | `apple_photos.limit_export` |
| `APPLE_PHOTOS_SPOUSE_DEVICE_MODELS` | `apple_photos.spouse_device_models` (array) |
| `SD_CARD_SRC_PATH` | `sd_card.source` |
| `SD_CARD_DST_PATH` | `sd_card.destination` |
| `SD_CARD_EXCLUDE_FILE` | `sd_card.exclude_file` |
| `ALL_PHOTOS_PATH` | `ssd.source` |
| `ALL_PHOTOS_EXCLUDE_FILE` | `ssd.exclude_file` |
| `SSD_DST_PATH` | `ssd.destination` |
| `RCLONE_REMOTE` | `rclone.remote` |
| `RCLONE_SRC_PATH` | `rclone.source` (defaults to `ssd.destination`) |

`APPLE_PHOTOS_DST_PATH` and `ALL_PHOTOS_PATH` held the same export directory, so
`ssd` now copies that directory once instead of twice. Any `ONE_DRIVE_*` values
in the old file are unused credentials and must not be carried over. An exclude
file that does not exist on disk is warned about and ignored for ordinary copies;
SSD deletion-enabled runs refuse it instead.
