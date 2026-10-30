# goback: unattended runs and macOS scheduling

Part of the [goback README](../README.md).

## Unattended execution

`--non-interactive` is available on `run daily`, `weekly`, `monthly`, and `all`:

```bash
goback --profile default run daily --non-interactive --dry-run
goback --profile default run daily --non-interactive
```

The flag explicitly approves the configured snapshot operations, including any
configured deletion, even when `confirmExec` is true. Configuration must already
exist and pass validation; goback never offers to create or repair it in this
mode and never rewrites it. Mounted-drive, overlap, source, and destination checks
still apply. Competing goback operations fail immediately through the existing
destination locks. Missing destinations are not created. A missing drive or busy
destination is a failed job; the next scheduled invocation tries again.

Rsync progress output is suppressed. Start and finish lines include RFC 3339
timestamps, and the result table still reports every step. Add `--quiet` to omit
transfer statistics too. Rsync exit 24 remains a success. Main transfer failures,
missing or failed companions, and interruptions make an unattended job exit
nonzero. A successful main backup keeps its own successful history even when a
companion fails. Without `--non-interactive`, companions retain their existing
behavior of not changing the main command's exit status.

Dry runs do not create backup directories, locks, or history. A companion with
no `dryRunArgs` is intentionally skipped and does not fail a dry run. A companion
that runs and fails still fails the unattended command. `run all` follows the
existing sequence: companion failure does not prevent weekly/monthly copies of
a successful daily transfer, but the overall unattended job fails.

Companion programs run without a shell and with stdin closed. They must support
unattended execution themselves; this flag cannot suppress prompts or grant
permissions inside another program. `ejectOnExit` retains its configured
behavior, requiring every requested transfer and companion to succeed.

## Generate and review a daily LaunchAgent

Build and install the updated binary first (see the README quick start), then
check the configuration and preview the backup:

```bash
goback config check
goback --profile default preview daily
goback --profile default run daily --non-interactive --dry-run
```

Generate a plist into a temporary directory for review:

```bash
schedule_dir="$(mktemp -d)"
goback --profile default schedule daily \
  --at 09:00 \
  --binary "$HOME/.local/bin/goback" \
  --exec-path "$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin" \
  > "$schedule_dir/local.goback.daily.plist"
plutil -lint "$schedule_dir/local.goback.daily.plist"
cat "$schedule_dir/local.goback.daily.plist"
```

Generation only prints XML; it creates no files or directories itself and never
calls `launchctl`. Drives may be offline. The selected configuration is validated
read-only. `--config PATH` is supported, and its absolute path is recorded.

The plist runs exactly one fixed profile with `run daily --non-interactive
--quiet`. A sole or unambiguous automatically selected profile is accepted, then
written explicitly into the job. Multiple selections require `--profile NAME`;
`--all` is refused. Source and destination must be absolute paths. A profile
without daily configured, or with `rsync.daily.dryRun` enabled, is refused during
generation. The job reads the configuration afresh on each run, so later edits
to backup settings apply without regenerating the plist.

`--at` uses local 24-hour `HH:MM` time and defaults to `09:00`. The default
`--label` is `local.goback.daily`; use a distinct label for each additional
profile or configuration. Label characters are letters, digits, dots,
underscores, and dashes, starting with a letter or digit. The label also names
the stdout and stderr logs.

`--binary` defaults to the executable generating the plist. Pass the installed
binary's absolute path when generating from `go run` or a temporary build, since
those executable paths may disappear. The binary must exist and be executable.

`--exec-path` defaults to the generating process's PATH and records it literally.
Every directory must be absolute, with no empty entries. Ensure it includes
the intended rsync and companion executables. The plist explicitly sets HOME
and PATH and uses HOME as its working directory; it does not load shell startup
files. Other environment variables needed by companions must be added to the
reviewed plist. Companion executables may be absolute paths or bare names
resolved through PATH; relative executable paths are refused. Relative companion
arguments resolve from HOME.

Logs default to `$HOME/Library/Logs/goback`; `--log-dir` selects another absolute
directory. The generated job uses umask `077`. Create the log directory before
loading the agent, so launchd can open stdout/stderr even when configuration or
drive checks fail.

## Install and inspect

After reviewing the plist, install it as a user LaunchAgent:

```bash
mkdir -p "$HOME/Library/LaunchAgents" "$HOME/Library/Logs/goback"
chmod 700 "$HOME/Library/Logs/goback"
cp "$schedule_dir/local.goback.daily.plist" "$HOME/Library/LaunchAgents/local.goback.daily.plist"
chmod 600 "$HOME/Library/LaunchAgents/local.goback.daily.plist"
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/local.goback.daily.plist"
launchctl print "gui/$(id -u)/local.goback.daily"
```

Use your chosen label and log directory consistently if overriding the defaults.
For an existing loaded job, unload it with the `bootout` command below before
replacing its plist and bootstrapping again. Remove the temporary directory once
the reviewed plist is installed.

The agent runs in your logged-in user session. It uses `StartCalendarInterval`:
macOS runs a missed calendar job after waking from sleep, while a missed job
during power-off waits until the next scheduled time. See Apple's
[timed-job documentation](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/ScheduledJobs.html).
The generated job has no immediate run on load and no automatic restart loop.
Reconnect an unavailable backup drive before the next run, or explicitly run
the installed job now:

```bash
launchctl kickstart "gui/$(id -u)/local.goback.daily"
tail -n 80 "$HOME/Library/Logs/goback/local.goback.daily.stdout.log"
tail -n 80 "$HOME/Library/Logs/goback/local.goback.daily.stderr.log"
```

These are real backup invocations. Check the job's last exit status with
`launchctl print` and inspect stderr for configuration, permissions, missing
programs, or unavailable drives. If macOS denies access to protected folders or
Photos, grant the scheduled process the required privacy permissions and retry.

The stdout/stderr files append across runs and are not rotated by goback;
manage their size separately. Failed transfers and companions also retain the
existing bounded diagnostic logs under `~/.goback/logs`. `clean logs` only trims
those diagnostic files, not the LaunchAgent output.

## Health checks and removal

```bash
goback --profile default status --daily --older-than 48h --check
goback --profile default status --companions --older-than 48h --check
```

Only run the second check when the profile has companions; an empty selection
fails a health check. Heartbeat can keep using the existing status JSON.
Failures before a program starts (including unavailable drives, lock contention,
and a missing companion executable) do not create attempt history: job exit
status and stderr report them immediately, while age checks detect the missing
new success over time. Scheduling does not change backup freshness by itself.

Unload and remove the schedule:

```bash
launchctl bootout "gui/$(id -u)/local.goback.daily"
rm "$HOME/Library/LaunchAgents/local.goback.daily.plist"
```

Backups, configuration, history, and logs remain available.
