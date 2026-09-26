extension HeartbeatConfig {
    /// What "Open Config" writes when there is no config file yet. It parses to exactly `HeartbeatConfig.defaults`,
    /// so creating it changes nothing until it is edited (the tests pin that).
    public static let exampleText = #"""
    // Heartbeat config. Every key is optional; the values below are the defaults.
    // JSON5 is accepted: comments and trailing commas are fine.
    // `heartbeatctl check-config` validates this file.
    {
      "version": 1,
      // Agents whose plist file name starts with this are watched.
      "labelPrefix": "com.sglavoie.",
      // Seconds between polls (at least 15).
      "pollSeconds": 60,
      // Global switch for macOS banners.
      "notifications": true,
      // Read-only Pi summary: `ssh <piHost> ~/.local/bin/pi-status --json` every piStatusSeconds.
      "piHost": "pi.tailb5cfdf.ts.net",
      "piStatusSeconds": 300,
      // argv used by "Open Log"; {path} is replaced by the log path. Without it the log opens in the default app.
      // "openLogCommand": ["open", "-a", "Ghostty", "--args", "-e", "nvim", "{path}"],

      // Per-agent settings. This key replaces the built-in agent settings wholesale, so keep the ones you want.
      // Keys: displayName, hidden, notify, ignoreExitCodes, maxAgeSeconds, expectsRunning, graceSeconds,
      // evidencePaths, health {command, intervalSeconds, timeoutSeconds, warningExitCodes},
      // receipt {path, reportedKey, statusKey, okValues}.
      "agents": {
        // Uptime Kuma already pages the phone for this one.
        "com.sglavoie.forgejo-sync": {
          "displayName": "Forgejo sync",
          "maxAgeSeconds": 2700,
          "notify": false,
        },
        // Kuma watches it too; the receipt says whether the run reached Kuma.
        "com.sglavoie.pi-backup-fetch": {
          "notify": false,
          "receipt": {
            "path": "~/Library/Application Support/pi-backup-fetch/last-run.json",
            "reportedKey": "reported",
            "statusKey": "status",
            "okValues": ["up"],
          },
        },
        // Exit 0 healthy, 1 unhealthy (red), 2 could not check (amber).
        "com.sglavoie.brainnotes-vault-guard": {
          "health": {
            "command": ["~/.local/bin/check-brainnotes-vault-health.sh"],
            "intervalSeconds": 300,
            "timeoutSeconds": 30,
            "warningExitCodes": [2],
          },
        },
      },
    }

    """#
}
