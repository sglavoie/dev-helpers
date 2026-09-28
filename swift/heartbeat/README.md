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

Heartbeat is the Mac-local half of the monitoring. The Pi runs Uptime Kuma,
which pushes phone alerts through ntfy and already watches two Mac jobs
(forgejo-sync and pi-backup-fetch) and the Pi itself. Heartbeat doesn't send
phone alerts. It watches every agent Kuma can't see, keeps banners off for the
two jobs Kuma already covers, and shows the Pi as one read-only row.

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

## Menu

The icon changes shape as well as color: an outlined `heart` when everything is
ok, an orange filled heart for a warning (or when Heartbeat can't check at all),
and a red `heart.slash.fill` followed by the number of failing agents.

The menu starts with a header such as "3 failing · 1 warning — checked 17:08",
then sections Failing, Warning, OK and Paused. Each row is a colored dot, the
agent's name (`displayName`, or the label without the prefix) and a short
detail like "Exited with status 1 · log 2 h ago". Its submenu lists the reasons,
the label, schedule, last evidence and where it came from, the next expected run,
the launchd state (pid, runs, last exit), the last health check (outcome, age
and first output line) and the receipt ("reported to Kuma", "not reported to
Kuma", "status down", missing or unreadable, with its finish time) for agents
that have them, and the plist, stow target and log paths. Below that come the
agent's actions:

- **View Log…** opens a window with the last 200 lines of the log (a
  stdout/stderr picker when they differ), re-read every 2 s while it is open,
  with Copy, Reveal in Finder and Open buttons.
- **Open Log** (and **Open Error Log** when stderr is a separate file) runs
  `openLogCommand`, or opens the file with its default app when that isn't set.
- **Run Now** (`launchctl kickstart`) for a loaded job that isn't running, or
  **Restart…** (`kickstart -k`, confirmed) for a running one.
- **Unload…** (`bootout`, confirmed) marks the agent paused, so it shows gray
  instead of amber. **Load** (`bootstrap gui/$UID <plist>`) loads it again.
  Seeing an agent loaded, from the menu or the shell, ends the pause.
- **Run Health Check Now** runs the agent's health command at once (shown as
  "Health Check Running…" until it finishes).
- **Edit Schedule…** opens a window to change what starts the agent: an
  interval, one or more calendar times (each field can be Any), watch paths
  with a throttle, or none. It previews the result ("daily 10:00 · next: …") and
  checks it before Save is enabled. Saving does four things:
  - It rewrites only the `StartInterval`, `StartCalendarInterval`, `WatchPaths`
    and (for watch paths) `ThrottleInterval` entries of the plist. For stowed
    agents it writes the stow target, keeping the file's indentation and key
    order, so `git diff` shows only the schedule lines. The result is parsed back
    and must match before the file is replaced.
  - When config.json sets a `maxAgeSeconds` for the agent that the new schedule
    makes too tight or far too loose, a checkbox offers a new value and replaces
    just that number, comments intact.
  - A loaded agent is reloaded (`bootout`, then `bootstrap`). An unloaded one
    picks up the change at its next Load.
  - With "Run now after saving" it is kicked off right away (not offered with
    RunAtLoad, which already runs it on reload).

  Plists an installer wrote (not stow symlinks) can be edited too, with a
  warning that reinstalling may revert the change.
- **Reveal Plist** shows the plist (the stow target for stowed agents) in
  Finder.

The actions run these commands, with no shell in between. `$UID` is your user
id and `<plist>` is the path in `~/Library/LaunchAgents`:

| Action | Command |
| --- | --- |
| Run Now | `launchctl kickstart gui/$UID/<label>` |
| Restart… | `launchctl kickstart -k gui/$UID/<label>` |
| Unload… | `launchctl bootout gui/$UID/<label>` |
| Load | `launchctl bootstrap gui/$UID <plist>` |
| Edit Schedule… (save) | `launchctl bootout gui/$UID/<label>`, then `launchctl bootstrap gui/$UID <plist>` (retried briefly) |
| (every poll) | `launchctl print gui/$UID/<label>` |

The same commands work from a terminal, and Heartbeat notices within one poll.
An agent unloaded from the shell shows amber "Not loaded", because only an
Unload from the menu marks it paused.

After the agent sections comes the **Pi row**, which is read-only: "Pi — ok ·
48/48 up", "Pi — warn: Kuma: Forgejo pending — timeout…" (the first problem),
or "Pi — unreachable over Tailscale", which is the one thing Kuma can't tell
the Mac. Its submenu lists every problem pi-status reports (Kuma monitors that
aren't up, containers that aren't ok, failed systemd units and timers, the
backup and host limits) or ssh's error. Then come the Kuma counts, the host and
the check age, followed by **Check Pi Now** and **Open Uptime Kuma**
(`https://uptime.sglavoie.com`). Any Pi state other than ok makes the icon
amber at most, and the Pi row never shows banners.

Below it, the **Pi journal row** shows the last hour of journal errors from the
same report: "Pi journal — no errors in the last hour", "Pi journal — 15+
errors, last 17:41" ("+" when pi-status hit its 15-line limit), or "Pi journal
— unknown" when the journal or the Pi couldn't be read. Its submenu lists each
distinct message, newest first, as `ident: message ×count · time`, with the
hostname and PID dropped and logfmt lines cut down to their `msg` and `error`.
The dot is amber while there are errors, but the journal is a look-back rather
than the Pi's health now. It never colors the icon, never changes the Pi row
and never counts toward `overall`, so a fixed problem doesn't leave the icon
amber for the rest of the hour. After **Check Pi Now**, **View Journal…** opens a
window with the full lines from `ssh <piHost> journalctl -p err --since -1h -n
300 -r --no-pager -o short-iso -q`, which is pi-status's query without its
15-line limit, shown oldest first. The window asks the Pi when it opens, every
30 s while it stays open, and on Refresh (⌘R). It also has Follow and Copy.
If the output reaches the 64 KB limit, the oldest lines are the ones dropped.

When launchctl fails, an alert shows its stderr and the command. After every
action the app polls again after 1 s and 5 s. Config errors and warnings, plist problems and state.json problems appear
under Problems. The footer has Refresh Now (⌘R), Open Config… (writes a
commented example first if there is no config file), Notifications, Launch at
Login and Quit. Notifications turns banners off without touching the config; it
reads "(off in config)" when `notifications` is false and "(allow in System
Settings)…" when macOS denied the permission.

The app polls every `pollSeconds`, 90 s after the Mac wakes (launchd runs
calendar jobs missed during sleep on wake), when the menu opens on a snapshot
older than 15 s, and 0.5 s after a change in `~/Library/LaunchAgents`, in a
stowed plist's target file or its directory, or in the config. One poll runs at
a time. After each poll the app saves state.json, so `heartbeatctl status` sees
the same ledger and shows the same verdicts.

Health commands run on their own schedule, not per poll: each one when its
`intervalSeconds` has passed since its last result, at most two at a time, with
the fixed GUI PATH. A finished check is saved in state.json and triggers a
poll, so its verdict (and a banner) follows within a second. Results survive a
relaunch, so restarting the app doesn't rerun every command.

The Pi check has its own timer and never shares one with the agent poll: every
`piStatusSeconds` (default 300), when the menu opens on a result at least
that old, on Refresh Now and right after `piHost` changes, the app runs
`ssh -o BatchMode=yes -o ConnectTimeout=5 -- <piHost> ~/.local/bin/pi-status
--json` on a background task, capped at 30 s. The full path is needed because
`~/.local/bin` isn't on the Pi's non-interactive PATH. pi-status exits 1 when
a section is failing, but its report is still read. ssh's exit 255, a timeout
or a failed launch counts as unreachable. Any other output that isn't a
report shows "cannot read pi-status". A slow or unreachable Pi never delays
an agent poll.

The report pi-status prints has a top-level `status` (`ok`, `unknown`, `warn`
or `fail`), `generated` (Unix time) and `sections`: `kuma` (`counts` and
`problems`), `containers` (an object whose own `containers` list holds each
container's `level`), `systemd` (`failed` units and `timers`), `backup`,
`host` (load, memory, disk, temperature, throttling) and `errors` (journal
`lines`, `distinct` messages with `count` and `last`, and from newer pi-status
`limit` and `truncated`). A section pi-status couldn't collect is `{status,
reason}`, and Heartbeat shows the reason. The Pi row's status is the worst
section level other than `errors`, and newer pi-status reports its top-level
`status` the same way. Heartbeat computes it from the sections so that an older
pi-status, which folds the journal in, reads the same.

Why the Pi row never sends banners: Kuma already pushes Pi problems to the
phone, so a Mac banner would only repeat it. The row is there for the one
thing Kuma can't report, which is the Mac losing its route to the Pi.

## Config

`~/.config/heartbeat/config.json` is optional. Every key has a default. The
file is read as JSON5, so `//` comments and trailing commas are allowed; the
example that Open Config… writes parses to exactly the defaults.

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
- `openLogCommand` is an argv: each `{path}` is replaced with the log path, or
  the path is appended when there is no `{path}`. Bare command names are looked
  up on the GUI PATH (`/opt/homebrew/bin` first). A non-zero exit shows the
  command's stderr.
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

Banners use `UNUserNotificationCenter`, so they only work from the installed
app bundle. The first launch asks for permission; banners from polls that
finish while that prompt is open wait for the answer. Each agent's banner has
the id `heartbeat.<label>`, so "recovered" replaces the "failing" banner it
answers. Agent banners have **View Log** and **Run Now** actions, and clicking
the banner opens the log. Every banner requested is logged:
`log show --last 1h --predicate 'subsystem == "dev.sglavoie.Heartbeat"'`.

## heartbeatctl

```
heartbeatctl status [--json] [--all] [--health] [--pi]
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
  ISO 8601 in local time. The tests pin these keys. `--pi` also asks the Pi
  (the same ssh command as the app, run while the snapshot is being built) and
  prints the Pi row with its problems, then the Pi journal row with its
  entries. The JSON gets a `pi` object with `host`, `checkedAt`, `status`
  (the Pi's health: `ok`, `warn`, `fail` or `unknown`, or else `unreachable` or
  `unreadable`), `severity`, `row`, `problems`, `kumaUp`, `kumaTotal`,
  `generated` and `journal`. `journal` is `null` when there's no report, or
  else an object with `status`, `severity` (`ok`, `warning` or `unknown`),
  `row`, `count`, `truncated`, `reason` and `entries` (`message`, `count`,
  `last`). `overall` includes the Pi but not its journal.
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
agents don't count. With `--pi`, a Pi that isn't ok turns ok into warning,
but never makes it worse than that. Journal errors alone don't. It is unknown when the LaunchAgents directory or the boot
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

## Data files

| Path | What it is |
| --- | --- |
| `~/.config/heartbeat/config.json` | Optional config (see [Config](#config)). Heartbeat never writes it, except Open Config… creating the commented example when it is missing, and Edit Schedule… replacing an agent's existing `maxAgeSeconds` number when you tick that box. |
| `~/Library/Application Support/Heartbeat/state.json` | Ledger and notification memory (see [State and notifications](#state-and-notifications)). Written by the app only. |
| `~/Library/Application Support/Heartbeat/state.corrupt-<time>.json` | A state file that couldn't be decoded, moved aside. Safe to delete. |
| `~/Library/Application Support/Heartbeat/canary/` | Scripts and log of the test canaries. `just canary-uninstall` removes it. |
| `~/Library/LaunchAgents/com.sglavoie.*.plist` | The agents being watched. Heartbeat only reads them, except Edit Schedule… rewriting the schedule entries (in the stow target for stowed agents) when you save. |
| `~/Library/Keychains/heartbeat-signing.keychain-db` | The local signing identity (see [Signing](#signing)). |

Deleting `state.json` while the app is quit is harmless: the next poll starts a
new ledger, so agents that only have ledger evidence show "Cannot verify last
run" until they run again, and banners already shown for red agents may be
shown once more.

## Troubleshooting

- **An agent is red and you want to know why.** Run
  `heartbeatctl explain <label>` (or `just explain <label>`). It prints every
  evidence source, the power timeline, the overdue working and one line per
  rule.
- **The menu and `heartbeatctl status` disagree.** The CLI builds its own
  snapshot right now, and the menu shows the app's last poll. Use Refresh Now
  (⌘R), then compare again. Without the app running, the CLI has no ledger, so
  agents whose only evidence is the ledger can differ.
- **An agent shows "Not loaded" (amber) after you stopped it on purpose.**
  `launchctl bootout` from a shell doesn't mark it paused. Load it again, then
  use Unload… in the menu, or set `hidden: true` for it in the config.
- **"Cannot verify last run".** The agent has a schedule but no log paths,
  `evidencePaths` or ledger history. Add an `evidencePaths` entry for a file
  the job writes on every run.
- **A false "Overdue" after sleep or reboot.** Check the power timeline in
  `heartbeatctl explain <label>`: `sysctl kern.boottime` and `kern.waketime`
  should match reality. Raise `graceSeconds` for a calendar job that takes a
  while to start after wake.
- **No banners.** Check the Notifications item in the footer: "(off in
  config)" means `notifications` is false, and "(allow in System Settings)…"
  means macOS denied the permission (System Settings → Notifications →
  Heartbeat). Banners only come from the installed, signed app, and only for
  agents with `notify` on. See whether Heartbeat asked for them with
  `log show --last 1h --predicate 'subsystem == "dev.sglavoie.Heartbeat"'`.
- **Permissions or the login item forgotten after a rebuild.** Compare
  `just show-dr` with an earlier build. A different certificate leaf means the
  signing keychain was recreated.
- **The Pi row says "unreachable over Tailscale".** Try the same command in a
  terminal: `ssh -o BatchMode=yes -o ConnectTimeout=5 pi.tailb5cfdf.ts.net
  ~/.local/bin/pi-status --json`. BatchMode means the ssh key must work without
  a prompt. "Cannot read pi-status" means ssh worked but the output wasn't a
  report; the submenu shows what came back.
- **A config edit seems ignored.** `heartbeatctl check-config` shows the
  file's warnings and errors. Invalid JSON keeps the last good config and shows
  "config error" in the menu header.
- **`swift test` can't find `TestingMacros`.** `Package.swift` already passes
  the Command Line Tools plugin path. Make sure `xcode-select -p` points at
  `/Library/Developer/CommandLineTools`.

## Live verification

Run on 2026-09-26 against the installed, signed app (Heartbeat 0.1.0, macOS
27.0) on this Mac. The menu was read and clicked through System Events, and
banners were confirmed in the `dev.sglavoie.Heartbeat` log and usernoted's
"Presenting" log. Every temporary file (canaries, config, symlink) was removed
afterwards.

| # | Check | Result |
| --- | --- | --- |
| 1 | `just test`, `just install`, no Dock icon, `just show-dr` stable | Pass: 214 tests in 25 suites. `show-dr` was identical before and after the install. The process is background-only (UIElement). |
| 2 | Live failures: backup-legacy-repo and sync-legacy red | Pass: both, plus check-legacy-health, show "Exited with status 1" in the menu and in `heartbeatctl status`, which exits 2. `status --json \| jq .overall` prints `"failing"`. |
| 3 | Canary red with one banner; exit 0 and Run Now recovers it; Unload, Load, shell bootout | Pass: red and exactly one banner across two runs. `canary-exit 0` then Run Now turned it ok and posted "recovered" with the same id. Unload… (confirmed) gave "Paused (unloaded by Heartbeat)", Load reloaded it, and `launchctl bootout` from the shell gave amber "Not loaded" on the next poll. |
| 4 | KeepAlive canary, `SuccessfulExit: false`, exits 0 | Pass: "Not running (KeepAlive)" right after loading, then red under Failing 16 s later, after the second poll, with one banner. (Session 8 also saw amber on the first poll in the menu.) |
| 5 | `maxAgeSeconds: 60` | Pass: red "Overdue: last run 1 min ago (allowed 1 min)" in the menu and CLI. Removing the config reverted it and posted "recovered". |
| 6 | Health command `exit 1`, then `sleep 60` with a 5 s timeout | Pass: red "Health check failed (exit 1): canary unhealthy", then amber "Health check timed out after 5 s" with no banner and no leftover `sleep` (`pgrep`). |
| 7 | Receipt with `"reported": false` | Pass: amber "Ran locally but not reported to Kuma", no banner. |
| 8 | Pi row ok; bogus `piHost` | Pass: "Pi — ok · 48/48 up". With `piHost: bogus.invalid` it showed "Pi — unreachable over Tailscale", and back to ok after. No Pi banner. |
| 9 | Sleep across a calendar slot, no false red on wake | **Pending**: putting the Mac to sleep would interrupt whoever is using it, so it needs someone at the Mac. The rule is covered by the HealthEvaluator sleep and boot tests, and the app polls 90 s after wake. To check: note a calendar agent's next slot, sleep the Mac across it, wake it, and confirm the agent stays ok (launchd runs the missed slot on wake, within the 15 min grace). |
| 10 | Stowed plist edit refreshes in under 1 s; broken config | Pass: editing the target of a symlinked plist (in `/tmp`) refreshed state 0.65 s later, with the new schedule in the submenu. Invalid JSON showed "config error" in the header and the error under Problems, and kept the last good config. |
| 11 | View Log, Open Log, Reveal Plist, Launch at Login, `just canary-uninstall` | Pass: the log window showed a line appended while it was open. Open Log ran a temporary `openLogCommand` with the log path, and Reveal Plist selected the plist in Finder. Launch at Login toggled on and back off with no error. `canary-uninstall` left both labels at exit 113, and the canaries were pruned from `state.json`. |
| 12 | Forced forgejo-sync transport failure: Heartbeat red, Kuma/ntfy phone alert, no osascript banner | Pass: 2026-09-26. A temporary canary repo under `~/1_dev_projects` with a Forgejo remote that doesn't exist was pushed by a `launchctl kickstart` at 18:10:49. The run logged "push failed", sent a DOWN report to Kuma and exited 1. Heartbeat red: the menu and `heartbeatctl status` listed "Forgejo sync — Exited with status 1" under Failing. Kuma/ntfy phone alert: the "Mac Forgejo sync" (#49) page reached the phone shortly after the failure. No osascript banner: no osascript process ran and usernoted presented no banner. Heartbeat sent none either, because forgejo-sync is `notify: false`. Deleting the canary and kicking the job again at 18:13:32 logged "Recovered", exited 0 and turned the agent ok. |
