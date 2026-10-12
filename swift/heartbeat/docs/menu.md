# Heartbeat: menu

Part of the [Heartbeat README](../README.md).

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

Below that, the **LAN devices row** shows the Pi's lan-devices list, read with
every Pi check by `ssh <piHost> python3 pi-monitoring/lan_devices.py list
--json`: "LAN devices — 1 pending · 4 approved", "LAN devices — all 5
approved", or "LAN devices — unknown" when the Pi or the list couldn't be read.
Its submenu groups the devices under Rejected, Pending and Approved, each as
"192.168.1.66  My iPhone · 3 min ago" (the MAC when there is no name). Each
device's submenu has its status, MAC (marked private when randomized), IP and
first and last sighting, then **Approve…** (**Rename…** once approved), which
asks for a name, **Reject…** (**Edit Note…** once rejected), which asks for an
optional note, and **Forget…**, which confirms. The decision runs
`lan_devices.py approve|reject|forget MAC [TEXT]` over the same ssh, every word
single-quoted for the remote shell, shows "Approving…" on the device while it
runs, and checks the Pi again when it's done; a failure shows the script's
message and the command. The dot is red while a rejected device has been seen
in the last hour, amber with any pending, and green when all are approved. Like
the journal row, it never colors the icon and never counts toward `overall`:
the Pi already sends the phone alert.

Below that, the **Mac backups row** shows how fresh goback's backups are, read
locally (no ssh) with `goback status --json` at launch, every 5 minutes, on menu
open when the last check is older than that, and on Refresh Now (⌘R) or the
submenu's Check Backups Now. goback opens `~/.goback.db` read-only, so a check
changes nothing. The binary is the first executable of `~/.local/bin/goback`
(where `just install` puts it), `~/go/bin/goback`, `/opt/homebrew/bin/goback`
and `/usr/local/bin/goback`; the row's tooltip shows the command. The dot comes
from the age of each kind's last *successful* run: daily green up to 2 days,
amber up to 4; weekly green up to 9 days, amber up to 14; monthly green up to
35 days, amber up to 45; red beyond that or when no success was ever recorded
(`BackupFreshness` in `MacBackups.swift`). The row names the worst kind, or the
one furthest through its green window: "Mac backups — weekly never recorded",
"Mac backups — daily 3 days ago", with ", last attempt failed" when that kind's
latest run failed. Its submenu lists each kind's last success and, when it was
a different run or failed, its last attempt with the exit code; then the mirror
and the daily companions (Apple Photos) as information only, since they never
affect the dot; then the thresholds, the history database, the binary and the
check time. The dot is amber with "goback not found" or "cannot read goback
status" (the submenu shows goback's error, such as a configuration it rejects).
Unlike the Pi rows, nothing else watches these backups, so a red row turns an
ok icon amber and counts as a warning in `overall`. Only red counts, because
goback runs by hand and a day or two late is a reminder; and only as amber,
because the red icon and its count mean an agent failed.

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

