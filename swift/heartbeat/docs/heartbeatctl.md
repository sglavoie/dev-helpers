# Heartbeat: heartbeatctl

Part of the [Heartbeat README](../README.md).

## heartbeatctl

```
heartbeatctl status [--json] [--all] [--health] [--pi] [--backups]
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
  `last`). `overall` includes the Pi but not its journal. `--backups` runs
  `goback status --json` alongside and prints the Mac backups row with each
  kind's lines; the JSON gets a `backups` object with `binary`, `checkedAt`,
  `status` (`ok`, `unavailable` or `unreadable`), `detail`, `severity`, `row`,
  `dbPath`, `dbExists`, `kinds` (daily, weekly and monthly: `profile`,
  `backupType`, `lastSuccess`, `latestAttempt`, `exitCode`, `severity`,
  `okDays`, `warningDays`) and `info` (the mirror and companions, same keys
  without the last three). Without the flag the output is unchanged.
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
but never makes it worse than that. Journal errors alone don't. With
`--backups`, a red Mac backups row turns ok into warning the same way. It is unknown when the LaunchAgents directory or the boot
time can't be read.

Exit status: 0 ok, 1 warning, 2 failing, 3 unknown, 64 usage error. `explain`
exits with its agent's severity. `check-config` exits 0 when clean, 1 on
warnings and 2 when the file can't be used.

