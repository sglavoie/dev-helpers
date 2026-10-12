# Heartbeat

A native macOS menu-bar app that watches the health of the `com.sglavoie.*`
LaunchAgents on this Mac. It reads `launchctl print` and each agent's logs and
schedule, then shows one heart icon: green when everything ran as expected,
amber when something cannot be verified, red when an agent failed or is overdue.
It also shows a read-only summary row for the Raspberry Pi. Phone alerts stay
with Uptime Kuma and ntfy on the Pi.

The app lives in the menu bar and has no Dock icon. The menu shows each agent's
verdict and why, and has per-agent actions: View Log, Run Now, Unload, Load,
Run Health Check Now, Edit Schedule… and Reveal Plist. macOS banners appear
when an agent turns red and when it recovers.

Copy Diagnostics in an agent's submenu copies the same explanation as
`heartbeatctl explain`, using the app's current snapshot without running another
check. Refresh Now shows “Refreshing…” while checks run. Agent rows show progress
while a start, restart, load, unload or schedule save is in flight, and conflicting
menu actions are disabled until it finishes. “Requesting start…” means the start
request is being sent; the next polls determine the job's health.

The headline counts paused agents separately from healthy ones (for example,
“12 ok · 2 paused”). When every discovered agent is hidden, it says how many
are hidden instead of “No agents found”. Changing `piHost` clears the old
host's status immediately and shows “Checking [new host]…” until its check
finishes.

View Log labels separate streams as Standard output and Standard error. Its
case-insensitive filter searches only the displayed tail (up to 200 lines),
and Copy copies the filtered lines. Turning Follow back on immediately scrolls
to the newest line in both the log and Pi journal windows. If a refresh fails,
these windows retain the last successful output with a warning and its fetch
time; switching files or hosts clears the saved output.

Pause updates freezes the local log text while you read or copy it; Follow only
controls scrolling. Resuming fetches the latest tail immediately. Switching log
streams resumes updates. The Pi journal also has a case-insensitive filter over
its fetched lines, a matching-line count, and Copy copies only the filtered lines.

The schedule editor rejects impossible dates such as February 30 when Weekday is
Any, and intervals too large to calculate safely. February 29 remains valid, as do
entries with a weekday alternative: launchd matches either Day or Weekday when
both are specified.

While a schedule is being saved, its fields, Cancel button and window close action
are disabled. Custom log editors can remain open indefinitely; their stderr is
drained continuously with at most 64 KB retained for an error report.

Heartbeat is the Mac-local half of the monitoring. The Pi runs Uptime Kuma,
which pushes phone alerts through ntfy and already watches two Mac jobs
(forgejo-sync and pi-backup-fetch) and the Pi itself. Heartbeat doesn't send
phone alerts. It watches every agent Kuma can't see, keeps banners off for the
two jobs Kuma already covers, and shows the Pi as one read-only row.

## Documentation

Detailed reference lives in [`docs/`](docs/). This README covers setup,
discovery, runtime state, signing and data files.

- [Health rules](docs/health-rules.md): how each agent's verdict is computed, including schedules, DST and power gaps.
- [Menu](docs/menu.md): the menu-bar UI, per-agent actions, windows and the Pi row.
- [Config and state](docs/config-and-state.md): `config.json`, the ledger, state file and notifications.
- [heartbeatctl](docs/heartbeatctl.md): the command-line companion (`status`, `explain`, `check-config`).
- [Troubleshooting and live verification](docs/troubleshooting.md): common problems and the recorded live test results.

On this page: [Requirements](#requirements) · [Build](#build) · [Agent discovery](#agent-discovery) · [Runtime state](#runtime-state) · [Signing](#signing) · [Data files](#data-files)

## Requirements

- macOS 15 or later.
- The Xcode Command Line Tools with Swift 6.2 or later. Xcode itself is not
  needed. Tests use Swift Testing, because XCTest ships only with Xcode.
- [`just`](https://github.com/casey/just).

## Build

See the available recipes:

```bash
just
```

| Recipe | What it does |
| --- | --- |
| `build` | Debug build of every target. |
| `test` | Run the core and app-model test suites (`swift test`). |
| `status` | `heartbeatctl status` with any flags, e.g. `just status --all`. |
| `explain` | `heartbeatctl explain <label>`, e.g. `just explain sync-legacy`. |
| `icon` | Build `AppIcon.icns` from `Resources/icon.png`. |
| `bundle` | Assemble an unsigned `Heartbeat.app` in `.build/bundle`. |
| `sign` | Bundle, then sign it with the local signing identity. |
| `install` | Sign, replace `~/Applications/Heartbeat.app` and launch it. |
| `run` | Sign and launch the app straight from `.build/bundle`. |
| `uninstall` | Quit the app and remove it from `~/Applications`. |
| `show-dr` | Print the designated requirement of the built app. |
| `setup-signing` | One-off: create the local code-signing identity. |
| `canary-install` | Load a throwaway agent that runs every 120 s and exits with the code in its `exit-code` file (default 1). |
| `canary-exit` | Set the code the canary's next run exits with, e.g. `just canary-exit 0`. |
| `canary-daemon-install` | Load a KeepAlive (`SuccessfulExit: false`) canary that exits 0 and stays down. |
| `canary-daemon-uninstall` | Unload and delete the KeepAlive canary. |
| `canary-uninstall` | Unload and delete both canaries and their files. |
| `clean` | Remove build artifacts. |

The package has three targets plus tests:

- `HeartbeatCore`: Foundation-only logic shared by the app and the CLI. All the
  rules live here and are covered by the tests.
- `Heartbeat`: the AppKit menu-bar app.
- `heartbeatctl`: a command-line tool on top of HeartbeatCore that shows the
  same verdicts as the app (see [heartbeatctl](docs/heartbeatctl.md)).

The `.app` bundle is put together by hand from `Resources/Info.plist.in` and
`Resources/icon.png`, since there is no Xcode project. The app target uses no
SwiftPM resources for the same reason.

## Agent discovery

Heartbeat looks for `com.sglavoie.*.plist` files in `~/Library/LaunchAgents`.
Stow symlinks are followed to their targets, and broken links are reported
instead of skipped. Installer-managed agents that were copied in as regular
files are picked up the same way. Plists are read with
`PropertyListSerialization`, so XML and binary plists both work.

In `StartCalendarInterval`, a missing key means "every", not zero: an entry with
only `Minute = 10` runs hourly at :10. Weekday 0 and 7 both mean Sunday.

## Runtime state

For each agent, Heartbeat runs `launchctl print gui/$UID/<label>` (at most four at
a time, 5 s timeout each). It doesn't use `launchctl list`, which reports a raw
wait status (256 for exit 1) and doesn't give runs or state. Only top-level
fields are read, meaning lines indented by exactly one tab, because nested
blocks repeat keys such as `state = active`. From those fields it takes `state`,
`pid`, `runs` and `last exit code`, or `last terminating signal` if the job was
killed. A running daemon that hasn't exited yet prints `last exit code = (never
exited)`. Exit status 113 means the label isn't loaded.

`heartbeatctl list` shows these columns next to the schedule:

```
LABEL                          STATE        PID   LAST EXIT  RUNS  SCHEDULE
com.sglavoie.ddc-brightnessd   running      1076  -          1     always running (KeepAlive)
com.sglavoie.forgejo-sync      not running  -     0          18    every 15 min
com.sglavoie.sync-legacy       not running  -     1          4     on change: PARA_MARIANA.md +3 (throttle 30 s)
```

External commands never go through a shell; they run as argv. Each one runs in
its own process group, stdin is `/dev/null`, and output is capped at 64 KB per
stream. On timeout, the whole group gets SIGKILL, so a script's `sleep` can't
outlive it.

## Signing

Notification permission and the login item are tied to the app's designated
requirement. With ad-hoc signing that requirement changes on every build, so
macOS would forget them after each rebuild. To avoid that, the app is signed
with a stable self-signed identity:

```bash
just setup-signing   # once per machine
just install         # every time you want the latest build
```

`setup-signing` creates the "Heartbeat Local Signing" certificate in its own
keychain, `~/Library/Keychains/heartbeat-signing.keychain-db`, and trusts it for
code signing. It never replaces an existing keychain, because a new certificate
would change the designated requirement. `just sign` unlocks that keychain on
its own, including after a reboot.

To check that a rebuild kept the same identity, compare `just show-dr` before and
after. The output should be identical, naming `dev.sglavoie.Heartbeat` and the
same certificate leaf hash.

## Data files

| Path | What it is |
| --- | --- |
| `~/.config/heartbeat/config.json` | Optional config (see [Config](docs/config-and-state.md#config)). Heartbeat never writes it, except Open Config… creating the commented example when it is missing, and Edit Schedule… replacing an agent's existing `maxAgeSeconds` number when you tick that box. |
| `~/Library/Application Support/Heartbeat/state.json` | Ledger and notification memory (see [State and notifications](docs/config-and-state.md#state-and-notifications)). Written by the app only. |
| `~/Library/Application Support/Heartbeat/state.corrupt-<time>.json` | A state file that couldn't be decoded, moved aside. Safe to delete. |
| `~/Library/Application Support/Heartbeat/canary/` | Scripts and log of the test canaries. `just canary-uninstall` removes it. |
| `~/Library/LaunchAgents/com.sglavoie.*.plist` | The agents being watched. Heartbeat only reads them, except Edit Schedule… rewriting the schedule entries (in the stow target for stowed agents) when you save. |
| `~/Library/Keychains/heartbeat-signing.keychain-db` | The local signing identity (see [Signing](#signing)). |

Deleting `state.json` while the app is quit is harmless: the next poll starts a
new ledger, so agents that only have ledger evidence show "Cannot verify last
run" until they run again, and banners already shown for red agents may be
shown once more.

