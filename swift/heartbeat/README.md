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
| `clean` | Remove build artifacts. |

The package has three targets plus tests:

- `HeartbeatCore`: Foundation-only logic shared by the app and the CLI. All the
  rules live here and are covered by the tests.
- `Heartbeat`: the AppKit menu-bar app.
- `heartbeatctl`: a command-line tool on top of HeartbeatCore that shows the
  same verdicts as the app (see [heartbeatctl](#heartbeatctl)).

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

## Health rules

Each agent gets a severity: hidden, ok, paused (gray), warning (amber) or
failing (red). Every rule that fires adds a reason, and the worst reason wins.

1. `hidden` in config: the agent is excluded.
2. Not loaded, and either `Disabled` in the plist or unloaded from Heartbeat:
   paused. Paused agents don't color the icon.
3. Not loaded for any other reason, or `launchctl print` output that can't be
   read: amber.
4. Last exit code not 0 (and not in `ignoreExitCodes`), or killed by a signal:
   red. A KeepAlive daemon that is running again after a failure is amber
   ("Restarted after exit 1") for an hour after the restart, then ok.
   `(never exited)` is neither a failure nor a sign that the job ran.
5. KeepAlive expected (`true`, or `SuccessfulExit: false`) but no PID: amber on
   the first poll, red from the second, so a ThrottleInterval respawn doesn't
   raise a false alarm.
6. Overdue, checked only while the agent isn't running:
   - `StartInterval` I: allowed age is `maxAgeSeconds`, or max(2I, I + 5 min)
     without it. Red when the newest evidence is older than that and the Mac
     has been awake, and the agent loaded, for longer than that too.
   - `StartCalendarInterval`: S is the latest slot. Skip it if it came before
     boot or before the agent was loaded, because launchd doesn't run slots
     missed while the Mac was off. If the Mac slept through S, launchd runs the
     job once on wake, so the deadline is max(S, last wake) + grace (15 min by
     default). Red when there is no evidence from S − 60 s on and the deadline
     has passed.
   - `maxAgeSeconds` also applies to agents without an interval, and it replaces
     the calendar check.
   - No evidence source at all (no log files, evidence paths or runs ledger):
     amber "Cannot verify last run".
7. Health command (`health` in config): an argv run without a shell, with a
   fixed GUI PATH, every 300 s by default with a 30 s timeout. Exit 0 is ok,
   any other exit or a signal is red, and an exit listed in `warningExitCodes`,
   a timeout or a command that can't start is amber. The first line of output
   (stderr first) goes into the message. A result older than three intervals
   is stale (amber), once the Mac has been awake that long, because it means
   the checks stopped running.
8. Receipt (`receipt` in config) for jobs that report to Uptime Kuma: a JSON
   file the job writes after each run. A `statusKey` value outside `okValues`
   is red. `reportedKey: false` is amber, "Ran locally but not reported to
   Kuma". A missing or unreadable receipt is amber too.

Rules 7 and 8 judge the job's own output rather than launchd, so they also
apply when an agent isn't loaded. They're skipped only for hidden and paused
agents.

Boot and wake times come from `sysctl kern.boottime` and `kern.waketime`
(`waketime` is 0 until the first sleep). Calendar slots are local wall time.
The missing keys are wildcards, and Day and Weekday match either one, as in
crontab. On the spring-forward day a time that doesn't exist, like 02:30, isn't
a slot, and on the fall-back day a repeated time counts once, at its first
occurrence. The evaluator is pure: the clock, time zone and power timeline are
passed in, so the tests pin them, with `America/Montreal` for the DST cases.

## Config

`~/.config/heartbeat/config.json` is optional. Every key has a default:

```json
{ "version": 1, "labelPrefix": "com.sglavoie.", "pollSeconds": 60, "notifications": true,
  "piHost": "pi.tailb5cfdf.ts.net", "piStatusSeconds": 300,
  "openLogCommand": ["open", "-a", "Ghostty", "--args", "-e", "nvim", "{path}"],
  "agents": {
    "com.sglavoie.forgejo-sync": { "displayName": "Forgejo sync", "maxAgeSeconds": 2700, "notify": false },
    "com.sglavoie.pi-backup-fetch": { "notify": false,
      "receipt": { "path": "~/Library/Application Support/pi-backup-fetch/last-run.json",
                   "reportedKey": "reported", "statusKey": "status", "okValues": ["up"] } },
    "com.sglavoie.brainnotes-vault-guard": { "health": {
      "command": ["~/.local/bin/check-brainnotes-vault-health.sh"], "warningExitCodes": [2] } } } }
```

Per-agent keys are `displayName`, `hidden`, `notify`, `ignoreExitCodes`,
`maxAgeSeconds`, `expectsRunning`, `graceSeconds`, `evidencePaths`, `health`
(`command`, `intervalSeconds`, `timeoutSeconds`, `warningExitCodes`) and
`receipt` (`path`, `reportedKey`, `statusKey`, `okValues`).

- Without a config file, the three agent entries above apply, so the two jobs
  Kuma already pages the phone for stay quiet. A file with an `agents` key
  replaces those entries completely.
- `~` is expanded in `evidencePaths`, `receipt.path` and the health command's
  executable.
- Unknown keys and out-of-range numbers are warnings, and the rest of the
  config still applies. `pollSeconds` has a minimum of 15.
- If the file isn't valid JSON or a value has the wrong type, Heartbeat keeps
  the last config that loaded (or the defaults) and turns amber.
- Banners need both `notifications` and the agent's `notify` to be on.

## State and notifications

`~/Library/Application Support/Heartbeat/state.json` holds what Heartbeat
remembers between polls: the boot time, and for each agent its `runs` count,
when `runs` last went up (the ledger's evidence of a run), when it was first
seen loaded, the last KeepAlive restart, the paused flag, the not-running
streak, the last health check result and the severity a banner was last shown
for. The app writes it atomically and the CLI only reads it. A file that can't
be decoded is renamed to `state.corrupt-<time>.json` and Heartbeat starts
fresh. A new boot time resets the per-session fields (`runs`, load time,
streak).

Banners appear only when an agent turns red, and again when it recovers to ok.
Amber is silent, and a red agent whose reason changes doesn't get a second
banner. Red to amber isn't a recovery, so amber back to red stays quiet too.
Pausing or hiding an agent clears its red marker without a banner. More than
three banners in one poll become one summary. The last notified severity is
saved in state.json, so relaunching the app doesn't repeat banners. Agents with
`notify: false` are still tracked, so turning banners on later doesn't replay
old failures.

## heartbeatctl

```
heartbeatctl status [--json] [--all] [--health]
heartbeatctl list [--json]
heartbeatctl explain <label>
heartbeatctl check-config
heartbeatctl --version
```

Every command builds a snapshot the same way the app does: discover the
agents, run `launchctl print` for them (four at a time), gather evidence and
evaluate the rules. Evidence is the newest of the stdout/stderr log mtimes, the
config `evidencePaths` mtimes, the ledger's last runs increment and the
receipt's `finished` time. A declared log path counts as a source even before
the file exists, because launchd creates the log when it starts the job.

The CLI reads `state.json` but never writes it, and it doesn't update the
ledger either: a runs increment noticed by a one-off command would date the run
at the moment someone looked. It still resets the saved history after a
reboot. Without the app running, there is no ledger history, so the evidence
comes from logs, evidence paths and receipts only.

- `status` shows a headline ("2 failing · 1 warning — checked 12:41") and the
  failing, warning and paused agents. `--all` adds ok and hidden agents.
  `--health` runs the configured health commands now; without it, the app's
  last results from `state.json` are used. `--json` prints `version`,
  `checkedAt`, `overall` (`ok`, `warning`, `failing` or `unknown`), `headline`,
  `counts`, `agents`, `problems`, `config` and `power`. Each agent has `label`,
  `name`, `severity`, `summary`, `detail`, `reasons` (`rule`, `code`,
  `severity`, `message`), `schedule`, `state`, `pid`, `runs`, `lastExitCode`,
  `lastSignal`, `lastEvidence`, `evidenceOrigin`, `nextExpected`, `plistPath`,
  `logPaths`, `disabled` and `notify`. Missing values are `null`, and dates are
  ISO 8601 in local time. The tests pin these keys.
- `list` prints every discovered agent with its launchd state and schedule.
  `--json` gives the same agent objects as `status --json`.
- `explain <label>` shows how one verdict was reached: the plist, launchd's
  view, config, saved history, each evidence source, the power timeline, the
  overdue working (T0, S, E + grace, next expected), the health check and one
  line per rule. The label can be written without the `com.sglavoie.` prefix.
- `check-config` shows where the config came from, its warnings and errors,
  and `agents` entries that match no discovered agent.

The overall status is failing if any agent is red. It is warning if any agent
is amber, the config is broken, a plist can't be read (broken symlink or
invalid file) or no agents are found. Otherwise it is ok. Paused and hidden
agents don't count. It is unknown when the LaunchAgents directory or the boot
time can't be read.

Exit status: 0 ok, 1 warning, 2 failing, 3 unknown, 64 usage error. `explain`
exits with its agent's severity. `check-config` exits 0 when clean, 1 on
warnings and 2 when the file can't be used.

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
