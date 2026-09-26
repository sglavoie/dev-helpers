import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct PiStatusParserTests {
    static func parse(_ json: String) throws -> PiSummary {
        try PiStatusParser.parse(Data(json.utf8))
    }

    @Test func realOkReport() throws {
        let summary = try Self.parse(PiStatusFixtures.ok)
        #expect(summary.status == .ok)
        #expect(summary.kumaUp == 48)
        #expect(summary.kumaTotal == 48)
        #expect(summary.problems.isEmpty)
        #expect(summary.generated == Date(timeIntervalSince1970: 1790466071.7666924))
    }

    @Test func warnWithProblemsInSectionOrder() throws {
        let summary = try Self.parse(PiStatusFixtures.warn)
        #expect(summary.status == .fail)
        #expect(summary.kumaUp == 46)
        #expect(summary.kumaTotal == 48)
        #expect(summary.problems == [
            "Kuma: Forgejo pending — timeout of 48000ms exceeded",
            "Kuma: Miniflux pending",
            "Container miniflux: Up 2 hours (unhealthy)",
            "Container forgejo: Up 10 seconds (health: starting)",
            "systemd: pi-backup.service failed (system)",
            "systemd: vault-sync.service exit-code (exit 2)",
            "Backup: last verified 1 d ago",
            "Host: load 4.60 on 4 cores, disk 88% used",
            "Journal: 3 errors in the last hour — kernel: usb 1-1.3: device descriptor read/64, error -71",
        ])
    }

    @Test func warnReportOnlyKuma() throws {
        let summary = try Self.parse(#"""
            {"status": "warn", "sections": {"kuma": {"status": "warn", "counts": {"up": 47, "pending": 1},
             "problems": [{"name": "ntfy", "group": "Infra", "state": "pending", "message": "line one\nline two"}]}}}
            """#)
        #expect(summary.status == .warn)
        #expect(summary.problems == ["Kuma: ntfy pending — line one"])
        #expect(summary.generated == nil)
    }

    @Test func sectionReasonsAndFallbacks() throws {
        let summary = try Self.parse(PiStatusFixtures.reasons)
        #expect(summary.kumaUp == nil)
        #expect(summary.kumaTotal == nil)
        #expect(summary.problems == [
            "Kuma: Kuma status page unreachable (URLError)",
            "Containers: Cannot connect to the Docker daemon",
            "Backup: 20260920T033000Z not verified",
            "Backup: latest attempt failed",
            "Host: warn",
            "future: warn",
        ])
    }

    @Test func longMessagesAreClipped() throws {
        let message = String(repeating: "x", count: 200)
        let summary = try Self.parse(#"{"status": "warn", "sections": {"kuma": {"status": "warn", "problems": [{"name": "a", "state": "down", "message": "\#(message)"}]}}}"#)
        let line = try #require(summary.problems.first)
        #expect(line.hasSuffix("…"))
        #expect(line.count == "Kuma: a down — ".count + PiStatusParser.messageLimit)
    }

    @Test func garbage() {
        #expect(throws: PiStatusParseError.notJSON) { try Self.parse("") }
        #expect(throws: PiStatusParseError.notJSON) { try Self.parse("Permission denied (publickey).") }
        #expect(throws: PiStatusParseError.notJSON) { try Self.parse(#"["ok"]"#) }
        #expect(throws: PiStatusParseError.notJSON) { try Self.parse(String(PiStatusFixtures.ok.prefix(200))) }
        #expect(throws: PiStatusParseError.missingStatus) { try Self.parse(#"{"sections": {}}"#) }
        #expect(throws: PiStatusParseError.missingStatus) { try Self.parse(#"{"status": 1}"#) }
        #expect(throws: PiStatusParseError.unknownStatus("great")) { try Self.parse(#"{"status": "great"}"#) }
    }

    @Test func garbageSectionsAreSkippedNotFatal() throws {
        let summary = try Self.parse(#"{"status": "warn", "sections": {"kuma": "nope", "containers": {"status": "warn", "containers": [1, "x"]}}}"#)
        #expect(summary.problems == ["Containers: warn"])
        #expect(summary.kumaTotal == nil)
    }
}

@Suite struct PiStatusClientTests {
    static let host = "pi.tailb5cfdf.ts.net"

    func fetch(_ response: CommandResult?, timeout: TimeInterval = 30) -> (PiStatus, FakeRunner) {
        let runner = FakeRunner(fallback: response)
        return (PiStatusClient(runner: runner, timeout: timeout).fetch(host: Self.host), runner)
    }

    @Test func argvUsesBatchModeConnectTimeoutAndFullPath() {
        let (status, runner) = fetch(result(0, stdout: PiStatusFixtures.ok))
        #expect(runner.recorded == [[
            "/usr/bin/ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "--", Self.host, "~/.local/bin/pi-status --json",
        ]])
        guard case .summary(let summary) = status else {
            Issue.record("expected a summary, got \(status)")
            return
        }
        #expect(summary.status == .ok)
    }

    @Test func failingReportExitsOneButIsStillRead() {
        let (status, _) = fetch(result(1, stdout: PiStatusFixtures.warn))
        guard case .summary(let summary) = status else {
            Issue.record("expected a summary, got \(status)")
            return
        }
        #expect(summary.status == .fail)
    }

    @Test func sshFailureIsUnreachable() {
        let stderr = "ssh: Could not resolve hostname bogus.invalid: nodename nor servname provided, or not known\n"
        #expect(fetch(result(255, stderr: stderr)).0 == .unreachable(
            "ssh: Could not resolve hostname bogus.invalid: nodename nor servname provided, or not known"))
        #expect(fetch(result(255)).0 == .unreachable("ssh exited with status 255"))
    }

    @Test func timeoutIsUnreachable() {
        let timedOut = CommandResult(argv: [], termination: .signaled(9), timedOut: true)
        #expect(fetch(timedOut, timeout: 30).0 == .unreachable("ssh timed out after 30 s"))
    }

    @Test func launchFailureIsUnreachable() {
        #expect(fetch(nil).0 == .unreachable("could not start /usr/bin/ssh: no fake response"))
    }

    @Test func garbageOutputIsUnreadable() {
        #expect(fetch(result(0, stdout: "hello")).0 == .unreadable("pi-status output: not a JSON object"))
        #expect(fetch(result(127, stderr: "bash: /home/me/.local/bin/pi-status: No such file or directory")).0
            == .unreadable("pi-status exited with status 127: bash: /home/me/.local/bin/pi-status: No such file or directory"))
    }

    @Test func checkStampsHostAndTime() {
        let runner = FakeRunner(fallback: result(0, stdout: PiStatusFixtures.ok))
        let now = TimeFixtures.utc("2026-09-26T23:41:00Z")
        let check = PiStatusClient(runner: runner).check(host: "pi", now: { now })
        #expect(check.host == "pi")
        #expect(check.checkedAt == now)
        #expect(check.severity == .ok)
    }
}

@Suite struct PiRowTests {
    let formatter = StatusFormatter(calendar: TimeFixtures.montreal, home: "/Users/me")
    static let now = TimeFixtures.utc("2026-09-26T23:41:00Z")

    func check(_ status: PiStatus) -> PiCheck {
        PiCheck(host: "pi.tailb5cfdf.ts.net", checkedAt: Self.now.addingTimeInterval(-120), status: status)
    }

    func summary(_ json: String) throws -> PiStatus {
        .summary(try PiStatusParser.parse(Data(json.utf8)))
    }

    @Test func rows() throws {
        #expect(formatter.piRow(nil) == "Pi — checking…")
        #expect(formatter.piRow(check(try summary(PiStatusFixtures.ok))) == "Pi — ok · 48/48 up")
        #expect(formatter.piRow(check(.summary(PiSummary(status: .ok)))) == "Pi — ok")
        #expect(formatter.piRow(check(try summary(PiStatusFixtures.warn)))
            == "Pi — fail: Kuma: Forgejo pending — timeout of 48000ms exceeded")
        #expect(formatter.piRow(check(.summary(PiSummary(status: .warn)))) == "Pi — warn: no details")
        #expect(formatter.piRow(check(.unreachable("ssh: connect timed out"))) == "Pi — unreachable over Tailscale")
        #expect(formatter.piRow(check(.unreadable("pi-status output: not a JSON object"))) == "Pi — cannot read pi-status")
    }

    @Test func severityIsAmberAtMost() throws {
        #expect(check(try summary(PiStatusFixtures.ok)).severity == .ok)
        #expect(check(try summary(PiStatusFixtures.warn)).severity == .warning)
        #expect(check(.summary(PiSummary(status: .unknown))).severity == .warning)
        #expect(check(.unreachable("x")).severity == .warning)
        #expect(check(.unreadable("x")).severity == .warning)
    }

    @Test func overallIncludesPiAsWarningAtMost() {
        let warn = check(.unreachable("x")), ok = check(.summary(PiSummary(status: .ok)))
        #expect(OverallStatus.ok.including(nil) == .ok)
        #expect(OverallStatus.ok.including(ok) == .ok)
        #expect(OverallStatus.ok.including(warn) == .warning)
        #expect(OverallStatus.warning.including(warn) == .warning)
        #expect(OverallStatus.failing.including(warn) == .failing)
        #expect(OverallStatus.unknown.including(warn) == .unknown)
    }

    @Test func menuInfo() throws {
        #expect(formatter.piMenuInfo(nil, now: Self.now) == [["Not checked yet"]])
        let ok = formatter.piMenuInfo(check(try summary(PiStatusFixtures.ok)), now: Self.now)
        #expect(ok[0] == ["No problems"])
        #expect(ok[1][0] == "Kuma: 48/48 monitors up")
        #expect(ok[1][1].hasPrefix("pi-status report: "))
        #expect(Array(ok[1].suffix(2)) == ["Host: pi.tailb5cfdf.ts.net", "Checked: 2 min ago"])
        let down = formatter.piMenuInfo(check(.unreachable("ssh: connect to host pi port 22: Operation timed out")), now: Self.now)
        #expect(down == [["ssh: connect to host pi port 22: Operation timed out"], ["Host: pi.tailb5cfdf.ts.net", "Checked: 2 min ago"]])
        let warn = formatter.piMenuInfo(check(try summary(PiStatusFixtures.warn)), now: Self.now)
        #expect(warn[0].count == 9)
    }

    @Test func statusTextAddsPiRowAndProblems() throws {
        let snapshot = SnapshotFixtures.builder([], prints: [:]).buildSync(config: SnapshotFixtures.config([:]), state: .empty, ledger: .readOnly)
        #expect(!formatter.statusText(snapshot).contains("Pi —"))
        let text = formatter.statusText(snapshot, pi: check(.unreachable("ssh: Could not resolve hostname bogus")))
        #expect(text.contains("\nPi — unreachable over Tailscale\n  ssh: Could not resolve hostname bogus"))
    }

    @Test func statusJSONPiKeys() throws {
        let snapshot = SnapshotFixtures.builder([], prints: [:]).buildSync(config: SnapshotFixtures.config([:]), state: .empty, ledger: .readOnly)
        let plain = try #require(try JSONSerialization.jsonObject(with: Data(formatter.statusJSON(snapshot).utf8)) as? [String: Any])
        #expect(plain["pi"] == nil)

        let json = formatter.statusJSON(snapshot, pi: check(try summary(PiStatusFixtures.ok)))
        let object = try #require(try JSONSerialization.jsonObject(with: Data(json.utf8)) as? [String: Any])
        let pi = try #require(object["pi"] as? [String: Any])
        #expect(Set(pi.keys) == ["host", "checkedAt", "status", "severity", "row", "problems", "kumaUp", "kumaTotal", "generated"])
        #expect(pi["status"] as? String == "ok")
        #expect(pi["kumaUp"] as? Int == 48)

        let down = formatter.statusJSON(snapshot, pi: check(.unreachable("x")))
        let downObject = try #require(try JSONSerialization.jsonObject(with: Data(down.utf8)) as? [String: Any])
        let downPi = try #require(downObject["pi"] as? [String: Any])
        #expect(downPi["status"] as? String == "unreachable")
        #expect(downPi["kumaUp"] is NSNull)
        #expect(downPi["problems"] as? [String] == ["x"])
    }
}
