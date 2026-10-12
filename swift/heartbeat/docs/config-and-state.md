# Heartbeat: config, state and notifications

Part of the [Heartbeat README](../README.md).

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

