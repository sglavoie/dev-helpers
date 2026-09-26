# Heartbeat

A native macOS menu-bar app that watches the health of the `com.sglavoie.*`
LaunchAgents on this Mac. It reads `launchctl print` and each agent's logs and
schedule, then shows one heart icon: green when everything ran as expected,
amber when something cannot be verified, red when an agent failed or is overdue.
It also shows a read-only summary row for the Raspberry Pi. Phone alerts stay
with Uptime Kuma and ntfy on the Pi.

The app lives in the menu bar and has no Dock icon. This is an early scaffold:
the menu only has the version and Quit so far.

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
| `test` | Run the HeartbeatCore test suite (`swift test`). |
| `icon` | Build `AppIcon.icns` from `Resources/icon.png`. |
| `bundle` | Assemble an unsigned `Heartbeat.app` in `.build/bundle`. |
| `sign` | Bundle, then sign it with the local signing identity. |
| `install` | Sign, replace `~/Applications/Heartbeat.app` and launch it. |
| `run` | Sign and launch the app straight from `.build/bundle`. |
| `uninstall` | Quit the app and remove it from `~/Applications`. |
| `show-dr` | Print the designated requirement of the built app. |
| `setup-signing` | One-off: create the local code-signing identity. |
| `clean` | Remove build artifacts. |

The package has three targets plus tests:

- `HeartbeatCore`: Foundation-only logic shared by the app and the CLI. All the
  rules live here and are covered by the tests.
- `Heartbeat`: the AppKit menu-bar app.
- `heartbeatctl`: a command-line tool on top of HeartbeatCore. For now it
  answers `heartbeatctl --version` and `heartbeatctl list`, which prints every
  discovered agent with its schedule.

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
