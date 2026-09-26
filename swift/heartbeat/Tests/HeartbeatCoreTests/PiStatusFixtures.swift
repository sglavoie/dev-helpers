import Foundation

/// `pi-status --json` output.
enum PiStatusFixtures {
    /// Captured on 2026-09-26 with `ssh -o BatchMode=yes -o ConnectTimeout=5 pi.tailb5cfdf.ts.net ~/.local/bin/pi-status --json`.
    static let ok = #"""
    {
      "status": "ok",
      "generated": 1790466071.7666924,
      "sections": {
        "kuma": {
          "status": "ok",
          "counts": {
            "down": 0,
            "up": 48,
            "pending": 0,
            "maintenance": 0
          },
          "problems": []
        },
        "containers": {
          "status": "ok",
          "containers": [
            {
              "name": "beszel",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "beszel-agent",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "beszel-socket-proxy",
              "state": "running",
              "health": "",
              "status": "Up 4 hours",
              "level": "ok"
            },
            {
              "name": "changedetection",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "changedetection-browser",
              "state": "running",
              "health": "",
              "status": "Up 4 hours",
              "level": "ok"
            },
            {
              "name": "cocotte-alerts",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "dozzle",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "dozzle-socket-proxy",
              "state": "running",
              "health": "",
              "status": "Up 4 hours",
              "level": "ok"
            },
            {
              "name": "forgejo",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "forgejo-runner",
              "state": "running",
              "health": "",
              "status": "Up 4 hours",
              "level": "ok"
            },
            {
              "name": "forgejo-runner-docker",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "homepage",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "homepage-socket-proxy",
              "state": "running",
              "health": "",
              "status": "Up 4 hours",
              "level": "ok"
            },
            {
              "name": "miniflux",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "miniflux-db",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "ntfy",
              "state": "running",
              "health": "",
              "status": "Up 4 hours",
              "level": "ok"
            },
            {
              "name": "uptime-kuma",
              "state": "running",
              "health": "healthy",
              "status": "Up 4 hours (healthy)",
              "level": "ok"
            },
            {
              "name": "uptime-kuma-bridge",
              "state": "running",
              "health": "",
              "status": "Up 4 hours",
              "level": "ok"
            }
          ]
        },
        "systemd": {
          "status": "ok",
          "failed": [],
          "timers": [
            {
              "scope": "system",
              "timer": "brainnotes-sync-health.timer",
              "service": "brainnotes-sync-health.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 26.42876410484314,
              "level": "ok"
            },
            {
              "scope": "system",
              "timer": "pi-autopilot-health.timer",
              "service": "pi-autopilot-health.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 71.42877745628357,
              "level": "ok"
            },
            {
              "scope": "system",
              "timer": "pi-autopilot-notify.timer",
              "service": "pi-autopilot-notify.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 671.4287827014923,
              "level": "ok"
            },
            {
              "scope": "system",
              "timer": "pi-autopilot.timer",
              "service": "pi-autopilot.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": null,
              "level": "ok"
            },
            {
              "scope": "system",
              "timer": "pi-music-mac-storage.timer",
              "service": "pi-music-probe@mac-storage.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 71.42879104614258,
              "level": "ok"
            },
            {
              "scope": "system",
              "timer": "pi-music-music.timer",
              "service": "pi-music-probe@music.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 11.428794145584106,
              "level": "ok"
            },
            {
              "scope": "system",
              "timer": "pi-music-recovery.timer",
              "service": "pi-music-recovery.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 17.428797006607056,
              "level": "ok"
            },
            {
              "scope": "system",
              "timer": "pi-services-backup.timer",
              "service": "pi-services-backup.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": null,
              "level": "ok"
            },
            {
              "scope": "system",
              "timer": "pi-updates.timer",
              "service": "pi-updates.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 15187.428802967072,
              "level": "ok"
            },
            {
              "scope": "user",
              "timer": "pi-monitoring-heartbeat.timer",
              "service": "pi-monitoring-heartbeat.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 69.50678157806396,
              "level": "ok"
            },
            {
              "scope": "user",
              "timer": "pi-probe-backup.timer",
              "service": "pi-probe@backup.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 71.50678849220276,
              "level": "ok"
            },
            {
              "scope": "user",
              "timer": "pi-probe-browser.timer",
              "service": "pi-probe@browser.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 71.50679230690002,
              "level": "ok"
            },
            {
              "scope": "user",
              "timer": "pi-probe-forgejo.timer",
              "service": "pi-probe@forgejo.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 70.50679445266724,
              "level": "ok"
            },
            {
              "scope": "user",
              "timer": "pi-probe-memory.timer",
              "service": "pi-probe@memory.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 11.506796836853027,
              "level": "ok"
            },
            {
              "scope": "user",
              "timer": "pi-probe-pi-storage.timer",
              "service": "pi-probe@pi-storage.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 71.50680017471313,
              "level": "ok"
            },
            {
              "scope": "user",
              "timer": "pi-probe-power.timer",
              "service": "pi-probe@power.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 11.506803035736084,
              "level": "ok"
            },
            {
              "scope": "user",
              "timer": "pi-probe-temperature.timer",
              "service": "pi-probe@temperature.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 11.506805419921875,
              "level": "ok"
            },
            {
              "scope": "user",
              "timer": "pi-probe-watches.timer",
              "service": "pi-probe@watches.service",
              "result": "success",
              "exit_status": "0",
              "last_run_ago": 71.50680828094482,
              "level": "ok"
            }
          ]
        },
        "backup": {
          "status": "ok",
          "id": "20260926T184625Z",
          "verified": true,
          "age": 17195.716568231583
        },
        "host": {
          "status": "ok",
          "load": [
            0.09,
            0.09,
            0.09
          ],
          "cores": 4,
          "memory": {
            "total": 4245749760,
            "available": 2371158016,
            "swap_used": 109854720
          },
          "disk": {
            "total": 491737796608,
            "free": 344118304768,
            "used_fraction": 0.30019960405378043
          },
          "temperature_c": 48.5,
          "throttled": "0x0",
          "uptime": 16104.38
        },
        "errors": {
          "status": "ok",
          "lines": [],
          "distinct": []
        }
      }
    }
    """#

    /// The same report's shape with a pending Kuma monitor, an unhealthy container, a failed timer and journal errors
    /// (the levels pi-status gives them).
    static let warn = #"""
    {
      "status": "fail",
      "generated": 1790466071.7666924,
      "sections": {
        "kuma": {
          "status": "warn",
          "counts": {"down": 0, "up": 46, "pending": 2, "maintenance": 0},
          "problems": [
            {"name": "Forgejo", "group": "Services", "state": "pending", "message": "timeout of 48000ms exceeded"},
            {"name": "Miniflux", "group": "Services", "state": "pending", "message": ""}
          ]
        },
        "containers": {
          "status": "fail",
          "containers": [
            {"name": "beszel", "state": "running", "health": "healthy", "status": "Up 4 hours (healthy)", "level": "ok"},
            {"name": "miniflux", "state": "running", "health": "unhealthy", "status": "Up 2 hours (unhealthy)", "level": "fail"},
            {"name": "forgejo", "state": "running", "health": "starting", "status": "Up 10 seconds (health: starting)", "level": "warn"}
          ]
        },
        "systemd": {
          "status": "fail",
          "failed": [{"scope": "system", "unit": "pi-backup.service"}],
          "timers": [
            {"scope": "system", "timer": "pi-backup.timer", "service": "pi-backup.service", "result": "exit-code",
             "exit_status": "1", "last_run_ago": 3600.5, "level": "fail"},
            {"scope": "user", "timer": "vault-sync.timer", "service": "vault-sync.service", "result": "exit-code",
             "exit_status": "2", "last_run_ago": 60.0, "level": "fail"},
            {"scope": "system", "timer": "pi-autopilot-health.timer", "service": "pi-autopilot-health.service",
             "result": "success", "exit_status": "0", "last_run_ago": 71.4, "level": "ok"}
          ]
        },
        "backup": {"status": "warn", "id": "20260925T033000Z", "verified": true, "age": 97200.0},
        "host": {
          "status": "warn", "load": [5.1, 4.6, 3.9], "cores": 4,
          "memory": {"total": 4245749760, "available": 2371158016, "swap_used": 109854720},
          "disk": {"total": 491737796608, "free": 58000000000, "used_fraction": 0.882},
          "temperature_c": 48.5, "throttled": "0x0", "uptime": 16104.38
        },
        "errors": {
          "status": "warn",
          "lines": ["a", "b", "c"],
          "distinct": [{"last": "2026-09-26T17:40:01", "message": "kernel: usb 1-1.3: device descriptor read/64, error -71", "count": 3}]
        }
      }
    }
    """#

    /// Sections pi-status couldn't collect carry only a reason.
    static let reasons = #"""
    {
      "status": "fail",
      "generated": 1790466071.0,
      "sections": {
        "kuma": {"status": "fail", "reason": "Kuma status page unreachable (URLError)"},
        "containers": {"status": "unknown", "reason": "Cannot connect to the Docker daemon"},
        "systemd": {"status": "ok", "failed": [], "timers": []},
        "backup": {"status": "fail", "id": "20260920T033000Z", "verified": false, "age": 500000.0, "attempt": "failed"},
        "host": {"status": "warn", "load": [0.1, 0.1, 0.1], "cores": 4},
        "errors": {"status": "ok", "lines": [], "distinct": []},
        "future": {"status": "warn"}
      }
    }
    """#
}
