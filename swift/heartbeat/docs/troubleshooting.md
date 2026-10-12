# Heartbeat: troubleshooting and live verification

Part of the [Heartbeat README](../README.md).

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
