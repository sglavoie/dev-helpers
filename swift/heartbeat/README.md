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
- `heartbeatctl`: a command-line tool on top of HeartbeatCore. For now it only
  answers `heartbeatctl --version`.

The `.app` bundle is put together by hand from `Resources/Info.plist.in` and
`Resources/icon.png`, since there is no Xcode project. The app target uses no
SwiftPM resources for the same reason.

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
