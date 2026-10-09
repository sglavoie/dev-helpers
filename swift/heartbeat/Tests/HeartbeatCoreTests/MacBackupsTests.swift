import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct MacBackupsTests {
    /// 2026-10-08T17:49:08-06:00, when the fixture was generated.
    static let now = ISO8601DateFormatter().date(from: "2026-10-08T17:49:08-06:00")!
    static let day: TimeInterval = 86_400
    static let binary = "/Users/me/.local/bin/goback"
    /// Real `goback status --json` output from this Mac.
    static let json = """
    {
      "generated_at": "2026-10-08T17:49:08-06:00",
      "db_path": "/Users/me/.goback.db",
      "db_exists": true,
      "backups": [
        {"profile": "default", "backup_type": "daily", "last_success": "2026-10-06T11:21:24-06:00",
         "latest_attempt": "2026-10-06T11:21:24-06:00", "exit_code": 0, "result": "succeeded", "freshness": "not assessed"},
        {"profile": "default", "backup_type": "weekly", "last_success": null, "latest_attempt": null, "exit_code": null,
         "result": "never run", "freshness": "no recorded success"},
        {"profile": "default", "backup_type": "monthly", "last_success": null, "latest_attempt": null, "exit_code": null,
         "result": "never run", "freshness": "no recorded success"},
        {"profile": "default", "backup_type": "companion/apple-photos", "last_success": null,
         "latest_attempt": "2026-10-06T11:21:24-06:00", "exit_code": 3, "result": "failed (exit 3)",
         "freshness": "no recorded success"},
        {"profile": "global", "backup_type": "mirror", "last_success": null, "latest_attempt": null, "exit_code": null,
         "result": "never run", "freshness": "no recorded success"},
        {"backup_type": "no profile"}
      ]
    }
    """

    static var formatter: StatusFormatter {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "America/Mexico_City")!
        return StatusFormatter(calendar: calendar, home: "/Users/me")
    }

    static func check(_ backups: [BackupRecord]) -> MacBackupsCheck {
        MacBackupsCheck(binary: binary, checkedAt: now,
                        result: .report(BackupStatusReport(dbPath: "/Users/me/.goback.db", dbExists: true, backups: backups)))
    }

    static func snapshots(daily: TimeInterval?, weekly: TimeInterval?, monthly: TimeInterval?) -> [BackupRecord] {
        [("daily", daily), ("weekly", weekly), ("monthly", monthly)].map { kind, age in
            let date = age.map { now.addingTimeInterval(-$0) }
            return BackupRecord(profile: "default", backupType: kind, lastSuccess: date, latestAttempt: date,
                                exitCode: date == nil ? nil : 0)
        }
    }

    @Test func parsesTheReportAndSkipsRowsWithoutAProfile() throws {
        let report = try BackupStatusParser.parse(Data(Self.json.utf8))
        #expect(report.dbPath == "/Users/me/.goback.db")
        #expect(report.dbExists)
        #expect(report.generatedAt == Self.now)
        #expect(report.backups.map(\.backupType) == ["daily", "weekly", "monthly", "companion/apple-photos", "mirror"])
        #expect(report.backups[0].lastSuccess == ISO8601DateFormatter().date(from: "2026-10-06T17:21:24Z"))
        #expect(report.backups[0].exitCode == 0)
        #expect(report.backups[1].lastSuccess == nil && report.backups[1].exitCode == nil)
        #expect(report.backups[3].exitCode == 3 && report.backups[3].isCompanion && report.backups[3].latestFailed)
        #expect(report.backups[4].freshness == nil)
        #expect(throws: BackupStatusParser.ParseError.self) { try BackupStatusParser.parse(Data("[]".utf8)) }
        #expect(throws: BackupStatusParser.ParseError.self) { try BackupStatusParser.parse(Data("{\"rows\": []}".utf8)) }
        #expect(throws: BackupStatusParser.ParseError.self) { try BackupStatusParser.parse(Data("not json".utf8)) }
    }

    @Test(arguments: [
        (BackupFreshness.daily, 2.0, Severity.ok), (.daily, 2.001, .warning), (.daily, 4.0, .warning), (.daily, 4.001, .failing),
        (.weekly, 9.0, .ok), (.weekly, 9.001, .warning), (.weekly, 14.0, .warning), (.weekly, 14.001, .failing),
        (.monthly, 35.0, .ok), (.monthly, 35.001, .warning), (.monthly, 45.0, .warning), (.monthly, 45.001, .failing),
    ])
    func severityBoundariesAreInclusive(policy: BackupFreshness, days: Double, expected: Severity) {
        #expect(policy.severity(lastSuccess: Self.now.addingTimeInterval(-days * Self.day), now: Self.now) == expected)
    }

    @Test func neverSucceededIsFailing() {
        for policy in BackupFreshness.policies {
            #expect(policy.severity(lastSuccess: nil, now: Self.now) == .failing)
        }
    }

    @Test func theRowNamesTheWorstKindAndIgnoresCompanionsAndTheMirror() throws {
        let real = MacBackupsCheck(binary: Self.binary, checkedAt: Self.now,
                                   result: .report(try BackupStatusParser.parse(Data(Self.json.utf8))))
        #expect(real.severity(now: Self.now) == .failing)
        #expect(Self.formatter.macBackupsRow(real, now: Self.now) == "Mac backups — weekly never recorded")

        let fresh = Self.check(Self.snapshots(daily: 1 * Self.day, weekly: 3 * Self.day, monthly: 34 * Self.day)
            + [BackupRecord(profile: "default", backupType: "companion/apple-photos", latestAttempt: Self.now, exitCode: 3),
               BackupRecord(profile: "global", backupType: "mirror")])
        #expect(fresh.severity(now: Self.now) == .ok)
        // Every kind is green; the monthly is furthest through its window (34/35 against 1/2 and 3/9).
        #expect(Self.formatter.macBackupsRow(fresh, now: Self.now) == "Mac backups — monthly 34 days ago")

        let lateDaily = Self.check(Self.snapshots(daily: 3 * Self.day, weekly: 1 * Self.day, monthly: 1 * Self.day))
        #expect(lateDaily.severity(now: Self.now) == .warning)
        #expect(Self.formatter.macBackupsRow(lateDaily, now: Self.now) == "Mac backups — daily 3 days ago")

        var failedAttempt = Self.snapshots(daily: 40 * 3600, weekly: Self.day, monthly: Self.day)
        failedAttempt[0].latestAttempt = Self.now
        failedAttempt[0].exitCode = 23
        let row = Self.formatter.macBackupsRow(Self.check(failedAttempt), now: Self.now)
        #expect(row == "Mac backups — daily 1 day ago, last attempt failed")
        #expect(Self.formatter.macBackupsMenuInfo(Self.check(failedAttempt), now: Self.now)[0].prefix(2) == [
            "Daily: last success 2026-10-07 01:49:08 (1 day ago)",
            "    last attempt 2026-10-08 17:49:08 (0 s ago) · exit 23",
        ])

        var failedOnly = Self.snapshots(daily: Self.day, weekly: Self.day, monthly: Self.day)
        failedOnly[1].lastSuccess = nil
        failedOnly[1].exitCode = -1
        #expect(Self.formatter.macBackupsRow(Self.check(failedOnly), now: Self.now) == "Mac backups — weekly never succeeded")
    }

    @Test func unreadableOrUnconfiguredIsAmberAndOnlyRedFoldsIntoTheIcon() {
        let unreadable = MacBackupsCheck(binary: Self.binary, checkedAt: Self.now, result: .unreadable("boom"))
        #expect(unreadable.severity(now: Self.now) == .warning)
        #expect(Self.formatter.macBackupsRow(unreadable, now: Self.now) == "Mac backups — cannot read goback status")
        let missing = MacBackupsCheck(binary: nil, checkedAt: Self.now, result: .unavailable("goback not found"))
        #expect(Self.formatter.macBackupsRow(missing, now: Self.now) == "Mac backups — goback not found")
        let empty = Self.check([BackupRecord(profile: "global", backupType: "mirror")])
        #expect(empty.severity(now: Self.now) == .warning)
        #expect(Self.formatter.macBackupsRow(empty, now: Self.now) == "Mac backups — none configured")
        #expect(Self.formatter.macBackupsRow(nil, now: Self.now) == "Mac backups — checking…")

        let failing = Self.check(Self.snapshots(daily: Self.day, weekly: nil, monthly: Self.day))
        let warning = Self.check(Self.snapshots(daily: 3 * Self.day, weekly: Self.day, monthly: Self.day))
        #expect(OverallStatus.ok.including(backups: failing, now: Self.now) == .warning)
        #expect(OverallStatus.ok.including(backups: warning, now: Self.now) == .ok)
        #expect(OverallStatus.ok.including(backups: unreadable, now: Self.now) == .ok)
        #expect(OverallStatus.ok.including(backups: nil, now: Self.now) == .ok)
        #expect(OverallStatus.failing.including(backups: failing, now: Self.now) == .failing)
    }

    @Test func menuListsEachKindThenInformationThenFacts() throws {
        let real = MacBackupsCheck(binary: Self.binary, checkedAt: Self.now,
                                   result: .report(try BackupStatusParser.parse(Data(Self.json.utf8))))
        let groups = Self.formatter.macBackupsMenuInfo(real, now: Self.now)
        #expect(groups.count == 3)
        #expect(groups[0] == [
            "Daily: last success 2026-10-06 11:21:24 (2 days ago)",
            "Weekly: never recorded",
            "Monthly: never recorded",
        ])
        #expect(groups[1] == [
            "Companion apple-photos (informational): no successful run",
            "    last attempt 2026-10-06 11:21:24 (2 days ago) · exit 3",
            "Mirror (informational): never recorded",
        ])
        #expect(groups[2] == [
            "Green/amber up to: daily 2/4 d · weekly 9/14 d · monthly 35/45 d",
            "History: ~/.goback.db",
            "goback: ~/.local/bin/goback",
            "Checked: 0 s ago",
        ])
        let unreadable = MacBackupsCheck(binary: Self.binary, checkedAt: Self.now, result: .unreadable("bad config"))
        #expect(Self.formatter.macBackupsMenuInfo(unreadable, now: Self.now).first == ["bad config"])
        #expect(Self.formatter.macBackupsMenuInfo(nil, now: Self.now) == [["Not checked yet"]])
    }

    @Test func runsTheFirstExecutableCandidateWithStatusJSON() {
        let runner = FakeRunner(fallback: result(0, stdout: Self.json))
        let client = MacBackupsClient(runner: runner, candidates: ["~/.local/bin/goback", "/opt/homebrew/bin/goback"],
                                      home: "/Users/me", isExecutable: { $0 == "/opt/homebrew/bin/goback" })
        let check = client.check(now: { Self.now })
        #expect(runner.recorded == [["/opt/homebrew/bin/goback", "status", "--json"]])
        #expect(check.binary == "/opt/homebrew/bin/goback")
        #expect(check.report?.backups.count == 5)
        #expect(MacBackupsClient(home: "/Users/me", isExecutable: { $0 == "/Users/me/.local/bin/goback" }).locate()
            == "/Users/me/.local/bin/goback")
        #expect(MacBackupsClient.command(binary: nil) == "goback status --json")
    }

    @Test func failuresMapToUnavailableOrUnreadable() {
        func check(_ runner: FakeRunner, executable: Bool = true) -> MacBackupsResult {
            MacBackupsClient(runner: runner, timeout: 20, candidates: ["/bin/goback"], home: "/Users/me",
                             isExecutable: { _ in executable }).check(now: { Self.now }).result
        }
        #expect(check(FakeRunner(), executable: false) == .unavailable("goback not found in /bin"))
        if case .unavailable(let detail) = check(FakeRunner()) {
            #expect(detail.contains("no fake response"))
        } else {
            Issue.record("a launch failure should be unavailable")
        }
        let stderr = """
        Error: configuration /Users/me/.goback.json: 1 error(s) decoding:

        * 'Profiles[default].Rsync[daily]' has invalid keys: detectlargeitems
        2026/10/08 17:49:08 Could not execute: configuration /Users/me/.goback.json: 1 error(s) decoding:

        * 'Profiles[default].Rsync[daily]' has invalid keys: detectlargeitems
        """
        #expect(check(FakeRunner(fallback: result(1, stderr: stderr))) == .unreadable(
            "configuration /Users/me/.goback.json: 1 error(s) decoding: 'Profiles[default].Rsync[daily]' has invalid keys: detectlargeitems"))
        #expect(check(FakeRunner(fallback: result(1, stderr: "Error: unknown command \"status\" for \"goback\"\nRun 'goback --help' for usage.")))
            == .unreadable("unknown command \"status\" for \"goback\""))
        #expect(check(FakeRunner(fallback: result(2))) == .unreadable("goback exited with status 2"))
        #expect(check(FakeRunner(fallback: result(0, stdout: "{oops"))) == .unreadable("goback status output: not a goback status report"))
        #expect(check(FakeRunner(fallback: result(0, timedOut: true))) == .unreadable("goback timed out after 20 s"))
        let killed = CommandResult(argv: [], termination: .signaled(9))
        #expect(check(FakeRunner(fallback: killed)) == .unreadable("goback killed by signal 9"))
    }

    @Test func heartbeatctlJSONAddsABackupsObjectOnlyWhenAsked() throws {
        let real = MacBackupsCheck(binary: Self.binary, checkedAt: Self.now,
                                   result: .report(try BackupStatusParser.parse(Data(Self.json.utf8))))
        let object = Self.formatter.macBackupsJSON(real, now: Self.now)
        #expect(Set(object.keys) == ["binary", "checkedAt", "severity", "row", "status", "detail", "dbPath", "dbExists",
                                     "kinds", "info"])
        #expect(object["severity"] as? String == "failing")
        #expect(object["status"] as? String == "ok")
        let kinds = try #require(object["kinds"] as? [[String: Any]])
        #expect(kinds.map { $0["backupType"] as? String } == ["daily", "weekly", "monthly"])
        #expect(kinds.map { $0["severity"] as? String } == ["warning", "failing", "failing"])
        #expect((object["info"] as? [[String: Any]])?.count == 2)
    }
}
