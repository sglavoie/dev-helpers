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
        ])
        #expect(summary.journal == PiJournal(status: .warn, count: 3, entries: [
            .init(message: "kernel: usb 1-1.3: device descriptor read/64, error -71", count: 3),
        ]))
    }

    @Test func journalDoesNotCountTowardHealth() throws {
        let summary = try Self.parse(PiStatusFixtures.journalOnly)
        #expect(summary.status == .ok)
        #expect(summary.problems.isEmpty)
        let journal = try #require(summary.journal)
        #expect(journal.status == .warn)
        #expect(journal.count == 15)
        #expect(journal.truncated)
        #expect(journal.entries.map(\.message) == [
            "systemd: Failed to start pi-music-probe@music.service - Pi privileged music sha…",
            "systemd: Failed to start pi-music-probe@mac-storage.service - Probe.",
            "navidrome: Error getting fs for library — stat /mnt/music: host is down",
        ])
        #expect(journal.entries.map(\.count) == [10, 2, 1])
        #expect(journal.last == TimeFixtures.utc("2026-09-27T23:41:00Z"))
    }

    @Test func journalLimitAndReason() throws {
        let sent = try Self.parse(#"{"status": "ok", "sections": {"errors": {"status": "warn", "lines": ["a"], "distinct": [], "limit": 1, "truncated": false}}}"#)
        #expect(sent.journal?.truncated == false)
        #expect(sent.status == .ok)
        let denied = try Self.parse(#"{"status": "ok", "sections": {"errors": {"status": "unknown", "reason": "Not in the adm group"}}}"#)
        #expect(denied.journal == PiJournal(status: .unknown, reason: "Not in the adm group"))
        #expect(try Self.parse(#"{"status": "ok", "sections": {}}"#).journal == nil)
    }

    @Test func journalMessages() {
        #expect(PiStatusParser.journalMessage("pi kernel: usb 1-1: error -71") == "kernel: usb 1-1: error -71")
        #expect(PiStatusParser.journalMessage("kernel: usb 1-1: error -71") == "kernel: usb 1-1: error -71")
        #expect(PiStatusParser.journalMessage("pi sshd[42]: msg=\"x\" error=\"y\"") == "sshd: x — y")
        #expect(PiStatusParser.journalMessage("no colon here") == "no colon here")
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

    @Test func journalRunsTheErrQueryNewestFirstAndReturnsOldestFirst() {
        let runner = FakeRunner(fallback: result(0, stdout: "c newest\nb\n\na oldest\n"))
        let log = PiStatusClient(runner: runner).journal(host: Self.host)
        #expect(runner.recorded == [[
            "/usr/bin/ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "--", Self.host,
            "journalctl -p err --since -1h -n 300 -r --no-pager -o short-iso -q",
        ]])
        #expect(log == .success(PiJournalLog(lines: ["a oldest", "b", "c newest"])))
    }

    @Test func journalTruncatedDropsThePartialOldestLine() {
        var cut = result(0, stdout: "c\nb\na par")
        cut.stdoutTruncated = true
        #expect(PiStatusClient(runner: FakeRunner(fallback: cut)).journal(host: Self.host)
            == .success(PiJournalLog(lines: ["b", "c"], truncated: true)))
    }

    @Test func journalFailures() {
        func journal(_ response: CommandResult?) -> Result<PiJournalLog, PiJournalFetchError> {
            PiStatusClient(runner: FakeRunner(fallback: response), timeout: 30).journal(host: Self.host)
        }
        #expect(journal(result(255, stderr: "ssh: connect timed out\n")) == .failure(.init(description: "ssh: connect timed out")))
        #expect(journal(result(1, stderr: "No journal files were opened")) == .failure(
            .init(description: "journalctl exited with status 1: No journal files were opened")))
        #expect(journal(CommandResult(argv: [], termination: .signaled(9), timedOut: true))
            == .failure(.init(description: "ssh timed out after 30 s")))
        #expect(journal(result(0)) == .success(PiJournalLog(lines: [])))
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
        #expect(warn[0].count == 8)
    }

    @Test func journalRows() throws {
        #expect(formatter.piJournalRow(nil) == "Pi journal — checking…")
        #expect(formatter.piJournalRow(check(try summary(PiStatusFixtures.ok))) == "Pi journal — no errors in the last hour")
        #expect(formatter.piJournalRow(check(try summary(PiStatusFixtures.journalOnly))) == "Pi journal — 15+ errors, last 19:41")
        #expect(formatter.piJournalRow(check(try summary(PiStatusFixtures.warn))) == "Pi journal — 3 errors")
        let one = PiSummary(status: .ok, journal: PiJournal(status: .warn, count: 1))
        #expect(formatter.piJournalRow(check(.summary(one))) == "Pi journal — 1 error")
        #expect(formatter.piJournalRow(check(.summary(PiSummary(status: .ok)))) == "Pi journal — not reported")
        let denied = PiSummary(status: .ok, journal: PiJournal(status: .unknown, reason: "no group"))
        #expect(formatter.piJournalRow(check(.summary(denied))) == "Pi journal — unknown")
        #expect(formatter.piJournalRow(check(.unreachable("x"))) == "Pi journal — unknown")
    }

    @Test func journalSeverityNeverTouchesOverall() throws {
        let journal = check(try summary(PiStatusFixtures.journalOnly))
        #expect(journal.severity == .ok)
        #expect(journal.journalSeverity == .warning)
        #expect(OverallStatus.ok.including(journal) == .ok)
        #expect(check(try summary(PiStatusFixtures.ok)).journalSeverity == .ok)
        #expect(check(.summary(PiSummary(status: .ok, journal: PiJournal(status: .unknown, reason: "x")))).journalSeverity == nil)
        #expect(check(.unreachable("x")).journalSeverity == nil)
    }

    @Test func journalMenuInfo() throws {
        #expect(formatter.piJournalMenuInfo(nil, now: Self.now) == [["Not checked yet"]])
        let full = formatter.piJournalMenuInfo(check(try summary(PiStatusFixtures.journalOnly)), now: Self.now)
        #expect(full[0] == [
            "systemd: Failed to start pi-music-probe@music.service - Pi privileged music sha… ×10 · 19:41",
            "systemd: Failed to start pi-music-probe@mac-storage.service - Probe. ×2 · 19:40",
            "navidrome: Error getting fs for library — stat /mnt/music: host is down · 19:32",
        ])
        #expect(full[1] == ["Window: last hour, journal priority err and above", "Only the newest 15 lines were read",
                            "Checked: 2 min ago"])
        let quiet = formatter.piJournalMenuInfo(check(try summary(PiStatusFixtures.ok)), now: Self.now)
        #expect(quiet[0] == ["No errors in the last hour"])
        #expect(formatter.piJournalMenuInfo(check(.unreachable("x")), now: Self.now)[0] == ["The Pi couldn't be checked"])
    }

    @Test func statusTextAddsPiRowAndProblems() throws {
        let snapshot = SnapshotFixtures.builder([], prints: [:]).buildSync(config: SnapshotFixtures.config([:]), state: .empty, ledger: .readOnly)
        #expect(!formatter.statusText(snapshot).contains("Pi —"))
        let text = formatter.statusText(snapshot, pi: check(.unreachable("ssh: Could not resolve hostname bogus")))
        #expect(text.contains("\nPi — unreachable over Tailscale\n  ssh: Could not resolve hostname bogus\nPi journal — unknown"))
        let journal = formatter.statusText(snapshot, pi: check(try summary(PiStatusFixtures.journalOnly)))
        #expect(journal.contains("\nPi — ok · 48/48 up\nPi journal — 15+ errors, last 19:41\n  systemd: Failed to start"))
    }

    @Test func statusJSONPiKeys() throws {
        let snapshot = SnapshotFixtures.builder([], prints: [:]).buildSync(config: SnapshotFixtures.config([:]), state: .empty, ledger: .readOnly)
        let plain = try #require(try JSONSerialization.jsonObject(with: Data(formatter.statusJSON(snapshot).utf8)) as? [String: Any])
        #expect(plain["pi"] == nil)

        let json = formatter.statusJSON(snapshot, pi: check(try summary(PiStatusFixtures.ok)))
        let object = try #require(try JSONSerialization.jsonObject(with: Data(json.utf8)) as? [String: Any])
        let pi = try #require(object["pi"] as? [String: Any])
        #expect(Set(pi.keys) == ["host", "checkedAt", "status", "severity", "row", "problems", "kumaUp", "kumaTotal", "generated",
                                 "journal"])
        #expect(pi["status"] as? String == "ok")
        #expect(pi["kumaUp"] as? Int == 48)
        let quiet = try #require(pi["journal"] as? [String: Any])
        #expect(Set(quiet.keys) == ["status", "severity", "row", "count", "truncated", "reason", "entries"])
        #expect(quiet["severity"] as? String == "ok")

        let noisy = formatter.statusJSON(snapshot, pi: check(try summary(PiStatusFixtures.journalOnly)))
        let noisyObject = try #require(try JSONSerialization.jsonObject(with: Data(noisy.utf8)) as? [String: Any])
        let noisyPi = try #require(noisyObject["pi"] as? [String: Any])
        #expect(noisyPi["severity"] as? String == "ok")
        let journal = try #require(noisyPi["journal"] as? [String: Any])
        #expect(journal["severity"] as? String == "warning")
        #expect(journal["count"] as? Int == 15)
        #expect(journal["truncated"] as? Bool == true)
        let first = try #require((journal["entries"] as? [[String: Any]])?.first)
        #expect(first["count"] as? Int == 10)
        #expect(first["last"] as? String == "2026-09-27T19:41:00-04:00")

        let down = formatter.statusJSON(snapshot, pi: check(.unreachable("x")))
        let downObject = try #require(try JSONSerialization.jsonObject(with: Data(down.utf8)) as? [String: Any])
        let downPi = try #require(downObject["pi"] as? [String: Any])
        #expect(downPi["status"] as? String == "unreachable")
        #expect(downPi["kumaUp"] is NSNull)
        #expect(downPi["problems"] as? [String] == ["x"])
        #expect(downPi["journal"] is NSNull)
    }
}
