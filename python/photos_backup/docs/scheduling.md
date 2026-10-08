# Scheduling

[Back to the user guide](../README.md)

`daily` usually runs as a goback companion (see [deployment](deployment.md)).
What goback cannot tell you is that it has stopped running, or that a copy has
fallen behind. A launchd agent running `status --check --notify` covers that: it
reads recorded state only, stays silent while nothing needs doing, and posts a
macOS notification when it suggests a command.

## A status watchdog

Save as `~/Library/LaunchAgents/com.sglavoie.photos-backup-check.plist` (or in
the stowed `launchagents` package that other agents already use):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.sglavoie.photos-backup-check</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/sglavoie/.local/bin/photos-backup</string>
        <string>status</string>
        <string>--check</string>
        <string>--notify</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
    <key>StartCalendarInterval</key>
    <dict>
        <key>Hour</key>
        <integer>19</integer>
        <key>Minute</key>
        <integer>0</integer>
    </dict>
    <key>StandardOutPath</key>
    <string>/Users/sglavoie/Library/Logs/photos-backup-check.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/sglavoie/Library/Logs/photos-backup-check.log</string>
</dict>
</plist>
```

Load it with `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.sglavoie.photos-backup-check.plist`
and try it once with `launchctl kickstart gui/$(id -u)/com.sglavoie.photos-backup-check`.

`status --check` exits 3, and notifies, whenever it suggests a command: an
overdue `daily`, a pending cleanup, a stale or never-recorded copy, a missing
verification, or an archive drive that is not connected. Copy suggestions for a
drive that is not mounted end with `# connect /Volumes/... first`. Run
`photos-backup status --short` by hand to see the full list.

## Scheduling `daily` without goback

Use the same plist with `daily --verify --notify` as the arguments. `daily` exits 3 when a
cleanup needs approval; its export has still completed, so the notification is
a reminder, not a failure. Keep `bootstrap`, `approve-cleanup`, and
`cleanup-local-export` out of schedules, as [deployment](deployment.md#manual-acceptance)
explains.

## launchd gotchas

- **`PATH`.** launchd starts jobs with only `/usr/bin:/bin:/usr/sbin:/sbin`.
  rsync 3.x, exiftool, and rclone are found through `PATH`, so without
  `/opt/homebrew/bin` a scheduled run would pick up macOS's old rsync or fail
  to find exiftool. `photos-backup doctor` run from the job (temporarily
  swapping the arguments) shows which binaries it resolved.
- **Permissions.** A launchd job cannot answer macOS privacy prompts. If a
  scheduled `daily` fails to read the Photos library or the drive while the same
  command works in Terminal, grant Full Disk Access to the interpreter that runs
  the tool, found with `realpath "$(uv tool dir)/photos-backup/bin/python3"`. That
  path changes when uv upgrades its managed Python, so recheck it after an
  upgrade. `status --check` does not open the Photos library.
- **Mistyped options** exit 2 before the command starts and `--notify` cannot
  report them, so run any new schedule once with `launchctl kickstart` and read
  the log.
- **Stopping.** `launchctl bootout` and logout send `SIGTERM`, which stops a run
  like Ctrl-C; see [transfers](transfers.md).
