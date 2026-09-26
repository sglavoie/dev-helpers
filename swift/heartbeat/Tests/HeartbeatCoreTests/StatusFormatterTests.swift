import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct StatusFormatterTests {
    typealias F = SnapshotFixtures

    let formatter = StatusFormatter(calendar: TimeFixtures.montreal, home: "/Users/me")

    /// sync-legacy failing (exit 1), forgejo-sync ok, pi-backup-fetch amber (not reported), hidden-job hidden.
    func mixed() -> Snapshot {
        let sync = F.agent("sync-legacy", schedule: .watchPaths(["/a"], throttleSeconds: 30), log: "/Users/me/Library/Logs/sync.log")
        let forgejo = F.agent("forgejo-sync", log: "/logs/forgejo.log")
        let pi = F.agent("pi-backup-fetch", schedule: .calendar([CalendarEntry(minute: 10, hour: 6)]), log: "/logs/pi.log")
        let hidden = F.agent("hidden-job")
        let config = F.config([
            forgejo.label: AgentConfig(displayName: "Forgejo sync", notify: false),
            pi.label: AgentConfig(receipt: ReceiptConfig(path: "/r.json")),
            hidden.label: AgentConfig(hidden: true),
        ])
        let prints = Dictionary(uniqueKeysWithValues: [
            F.loaded(sync.label, runs: 4, lastExit: "1"), F.loaded(forgejo.label, runs: 18), F.loaded(pi.label, runs: 2),
            F.loaded(hidden.label),
        ])
        let files = [
            "/Users/me/Library/Logs/sync.log": F.now.addingTimeInterval(-3 * 3600),
            "/logs/forgejo.log": F.now.addingTimeInterval(-240), "/logs/pi.log": F.now.addingTimeInterval(-6 * 3600),
        ]
        return F.builder([forgejo, hidden, pi, sync], prints: prints, files: files,
                         receipts: { _ in .notReported(finished: F.now.addingTimeInterval(-6 * 3600)) })
            .buildSync(config: config, state: .empty, ledger: .readOnly)
    }

    func parse(_ json: String) throws -> [String: Any] {
        try #require(try JSONSerialization.jsonObject(with: Data(json.utf8)) as? [String: Any])
    }

    // MARK: - JSON keys (the CLI's interface)

    @Test func statusJSONTopLevelKeys() throws {
        let object = try parse(formatter.statusJSON(mixed()))
        #expect(Set(object.keys) == ["version", "checkedAt", "overall", "headline", "counts", "agents", "problems", "config", "power"])
        #expect(object["overall"] as? String == "failing")
        #expect(object["version"] as? String == HeartbeatCore.version)
        #expect(object["checkedAt"] as? String == "2026-09-26T12:41:00-04:00")
        #expect(object["counts"] as? [String: Int] == ["failing": 1, "warning": 1, "ok": 1, "paused": 0, "hidden": 1])
        let config = try #require(object["config"] as? [String: Any])
        #expect(Set(config.keys) == ["source", "error", "warnings"])
        #expect(config["source"] as? String == "file")
        #expect(config["error"] is NSNull)
        let power = try #require(object["power"] as? [String: Any])
        #expect(Set(power.keys) == ["bootTime", "lastWake"])
        #expect(power["lastWake"] is NSNull)
    }

    @Test func statusJSONAgentKeys() throws {
        let object = try parse(formatter.statusJSON(mixed()))
        let agents = try #require(object["agents"] as? [[String: Any]])
        #expect(agents.map { $0["label"] as? String } == ["com.sglavoie.forgejo-sync", "com.sglavoie.pi-backup-fetch", "com.sglavoie.sync-legacy"])
        for agent in agents {
            #expect(Set(agent.keys) == [
                "label", "name", "severity", "summary", "detail", "reasons", "schedule", "state", "pid", "runs",
                "lastExitCode", "lastSignal", "lastEvidence", "evidenceOrigin", "nextExpected", "plistPath", "logPaths",
                "disabled", "notify",
            ])
        }
        let sync = agents[2]
        #expect(sync["severity"] as? String == "failing")
        #expect(sync["summary"] as? String == "Exited with status 1")
        #expect(sync["lastExitCode"] as? Int == 1)
        #expect(sync["lastSignal"] is NSNull)
        #expect(sync["runs"] as? Int == 4)
        #expect(sync["pid"] is NSNull)
        #expect(sync["state"] as? String == "not running")
        #expect(sync["name"] as? String == "sync-legacy")
        let reasons = try #require(sync["reasons"] as? [[String: Any]])
        #expect(reasons.count == 1)
        #expect(Set(reasons[0].keys) == ["rule", "code", "severity", "message"])
        #expect(reasons[0]["rule"] as? Int == 4)
        #expect(reasons[0]["code"] as? String == "exitCode")

        let forgejo = agents[0]
        #expect(forgejo["name"] as? String == "Forgejo sync")
        #expect(forgejo["notify"] as? Bool == false)
        #expect(forgejo["summary"] is NSNull)
        #expect((forgejo["reasons"] as? [Any])?.isEmpty == true)

        let pi = agents[1]
        #expect(pi["severity"] as? String == "warning")
        #expect(pi["nextExpected"] as? String == "2026-09-27T06:10:00-04:00")
        #expect(pi["evidenceOrigin"] as? String == "/logs/pi.log")
    }

    @Test func statusJSONAllIncludesHidden() throws {
        let agents = try #require(try parse(formatter.statusJSON(mixed(), all: true))["agents"] as? [[String: Any]])
        #expect(agents.contains { $0["severity"] as? String == "hidden" })
    }

    @Test func listJSONKeys() throws {
        let object = try parse(formatter.listJSON(mixed()))
        #expect(Set(object.keys) == ["agents", "problems"])
        #expect((object["agents"] as? [Any])?.count == 4)
    }

    @Test func overallValuesAreStable() {
        #expect(OverallStatus.allCases.map(\.rawValue) == ["ok", "warning", "failing", "unknown"])
        #expect(OverallStatus.allCases.map(\.exitCode) == [0, 1, 2, 3])
        #expect(Severity.allCases.map(\.rawValue) == ["hidden", "ok", "paused", "warning", "failing"])
    }

    // MARK: - Text

    @Test func headline() {
        #expect(formatter.headline(mixed()) == "1 failing · 1 warning — checked 12:41")
        let ok = F.builder([F.agent("job", log: "/l")], prints: Dictionary(uniqueKeysWithValues: [F.loaded("com.sglavoie.job")]),
                           files: ["/l": F.now]).buildSync(config: F.config(), state: .empty, ledger: .readOnly)
        #expect(formatter.headline(ok) == "All 1 agent ok — checked 12:41")
        let empty = F.builder([], prints: [:]).buildSync(config: F.config(error: "invalid JSON"), state: .empty, ledger: .readOnly)
        #expect(formatter.headline(empty) == "config error · No agents found — checked 12:41")
    }

    @Test func statusTextHidesOkUnlessAll() {
        let text = formatter.statusText(mixed())
        #expect(text.contains("Failing (1)"))
        #expect(text.contains("✗ com.sglavoie.sync-legacy      Exited with status 1 · log 3 h ago"))
        #expect(text.contains("! com.sglavoie.pi-backup-fetch  Ran locally but not reported to Kuma · log 6 h ago"))
        #expect(!text.contains("forgejo-sync"))
        #expect(text.contains("(1 ok, 1 hidden not shown; use --all)"))

        let all = formatter.statusText(mixed(), all: true)
        #expect(all.contains("OK (1)"))
        #expect(all.contains("✓ com.sglavoie.forgejo-sync"))
        #expect(all.contains("Hidden (1)"))
        #expect(!all.contains("not shown"))
    }

    @Test func detailFallsBackToSchedule() {
        let snapshot = mixed()
        #expect(formatter.detail(snapshot.agent("com.sglavoie.forgejo-sync")!, now: F.now) == "every 15 min · log 4 min ago")
    }

    @Test func listTextColumns() {
        let lines = formatter.listText(mixed()).split(separator: "\n").map(String.init)
        #expect(lines[0].hasPrefix("LABEL"))
        #expect(lines[0].contains("STATE        PID  LAST EXIT  RUNS  SCHEDULE"))
        #expect(lines.contains { $0.hasPrefix("com.sglavoie.sync-legacy") && $0.contains("not running") && $0.contains("  1  ") })
    }

    @Test func explainShowsTheRuleTrace() {
        let snapshot = mixed()
        let text = formatter.explain(snapshot.agent("com.sglavoie.sync-legacy")!, snapshot: snapshot)
        #expect(text.hasPrefix("com.sglavoie.sync-legacy — FAILING\n  Exited with status 1 (rule 4)"))
        #expect(text.contains("log  ~/Library/Logs/sync.log: 2026-09-26 09:41:00 (3 h ago)  <- newest"))
        #expect(text.contains("  4  failing: Exited with status 1"))
        #expect(text.contains("  6  pass: no schedule to check"))
        #expect(text.contains("  8  pass: no receipt"))
        #expect(text.hasSuffix("Verdict: failing"))
    }

    @Test func explainShowsTheCalendarTrace() {
        let snapshot = mixed()
        let text = formatter.explain(snapshot.agent("com.sglavoie.pi-backup-fetch")!, snapshot: snapshot)
        #expect(text.contains("Overdue check (rule 6)"))
        #expect(text.contains("S (latest slot)       2026-09-26 06:10:00 (6 h ago)"))
        #expect(text.contains("E + grace (deadline)  2026-09-26 06:25:00 (6 h ago)"))
        #expect(text.contains("next expected         2026-09-27 06:10:00 (in 17 h)"))
        #expect(text.contains("  8  warning: Ran locally but not reported to Kuma"))
        #expect(text.contains("receipt  /r.json"))
    }

    @Test func configReportFlagsUnknownLabels() {
        let result = ConfigLoadResult(config: HeartbeatConfig(agents: ["com.sglavoie.typo": AgentConfig()]), source: .file,
                                      warnings: ["unknown key \"polSeconds\""])
        let text = formatter.configReport(result, path: "/Users/me/.config/heartbeat/config.json", discovered: ["com.sglavoie.job"])
        #expect(text.hasPrefix("Config: ~/.config/heartbeat/config.json (file)"))
        #expect(text.contains("warning: unknown key \"polSeconds\""))
        #expect(text.contains("warning: agents.com.sglavoie.typo matches no discovered agent"))
        #expect(!text.hasSuffix("OK"))
        let clean = ConfigLoadResult(config: HeartbeatConfig(agents: [:]), source: .defaults)
        #expect(formatter.configReport(clean, path: "/x", discovered: []).hasSuffix("OK"))
    }
}
