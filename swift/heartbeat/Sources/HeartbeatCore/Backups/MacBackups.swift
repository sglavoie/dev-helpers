import Foundation

/// One row of `goback status --json`: the latest attempt and last success of one profile's backup kind.
public struct BackupRecord: Equatable, Sendable {
    public var profile: String
    /// `daily`, `weekly`, `monthly`, `mirror` or `companion/<id>`.
    public var backupType: String
    public var lastSuccess: Date?
    public var latestAttempt: Date?
    /// The latest attempt's exit code; -1 when it was interrupted.
    public var exitCode: Int?

    public init(profile: String, backupType: String, lastSuccess: Date? = nil, latestAttempt: Date? = nil,
                exitCode: Int? = nil) {
        self.profile = profile
        self.backupType = backupType
        self.lastSuccess = lastSuccess
        self.latestAttempt = latestAttempt
        self.exitCode = exitCode
    }

    /// The goback profile the mirror is recorded under.
    public static let mirrorProfile = "global"

    /// Daily, weekly and monthly snapshots: the only rows that set the row's severity.
    public var freshness: BackupFreshness? {
        profile == Self.mirrorProfile ? nil : BackupFreshness.policies.first { $0.kind == backupType }
    }

    public var isCompanion: Bool { backupType.hasPrefix("companion/") }

    /// The latest attempt exited nonzero or was interrupted.
    public var latestFailed: Bool { exitCode.map { $0 != 0 } ?? false }
}

/// How old a kind's last successful run may be: green up to `okDays`, amber up to `warningDays`, red beyond that
/// or when no success was ever recorded. goback runs by hand, so these leave a day or two of slack.
public struct BackupFreshness: Equatable, Sendable {
    public var kind: String
    public var okDays: Int
    public var warningDays: Int

    public static let daily = BackupFreshness(kind: "daily", okDays: 2, warningDays: 4)
    public static let weekly = BackupFreshness(kind: "weekly", okDays: 9, warningDays: 14)
    public static let monthly = BackupFreshness(kind: "monthly", okDays: 35, warningDays: 45)
    public static let policies = [daily, weekly, monthly]

    public func severity(lastSuccess: Date?, now: Date) -> Severity {
        guard let lastSuccess else { return .failing }
        let age = now.timeIntervalSince(lastSuccess)
        if age <= TimeInterval(okDays) * 86_400 { return .ok }
        if age <= TimeInterval(warningDays) * 86_400 { return .warning }
        return .failing
    }
}

/// The parsed `goback status --json` document.
public struct BackupStatusReport: Equatable, Sendable {
    public var generatedAt: Date?
    public var dbPath: String
    public var dbExists: Bool
    public var backups: [BackupRecord]

    public init(generatedAt: Date? = nil, dbPath: String, dbExists: Bool, backups: [BackupRecord]) {
        self.generatedAt = generatedAt
        self.dbPath = dbPath
        self.dbExists = dbExists
        self.backups = backups
    }
}

/// The outcome of asking goback for its status.
public enum MacBackupsResult: Equatable, Sendable {
    case report(BackupStatusReport)
    /// No goback binary, or it couldn't be started.
    case unavailable(String)
    /// goback ran but failed, timed out, or printed something that isn't a status report.
    case unreadable(String)
}

/// One snapshot kind judged against its `BackupFreshness`.
public struct BackupAssessment: Equatable, Sendable {
    public var record: BackupRecord
    public var policy: BackupFreshness
    public var severity: Severity

    /// How far through its green window the kind is; never-recorded sorts last.
    var urgency: Double {
        guard let age = age else { return .infinity }
        return age / (TimeInterval(policy.okDays) * 86_400)
    }

    /// Since the last success; `nil` when there is none.
    public var age: TimeInterval?
}

public struct MacBackupsCheck: Equatable, Sendable {
    /// The goback binary that was run, if one was found.
    public var binary: String?
    public var checkedAt: Date
    public var result: MacBackupsResult

    public init(binary: String?, checkedAt: Date, result: MacBackupsResult) {
        self.binary = binary
        self.checkedAt = checkedAt
        self.result = result
    }

    public var report: BackupStatusReport? {
        if case .report(let report) = result { return report }
        return nil
    }

    /// Each configured daily, weekly and monthly backup, in that order.
    public func assessments(now: Date) -> [BackupAssessment] {
        guard let report else { return [] }
        let order = Dictionary(uniqueKeysWithValues: BackupFreshness.policies.enumerated().map { ($1.kind, $0) })
        return report.backups.compactMap { record -> BackupAssessment? in
            guard let policy = record.freshness else { return nil }
            return BackupAssessment(record: record, policy: policy,
                                    severity: policy.severity(lastSuccess: record.lastSuccess, now: now),
                                    age: record.lastSuccess.map { now.timeIntervalSince($0) })
        }.sorted { a, b in
            a.record.profile != b.record.profile ? a.record.profile < b.record.profile
                : order[a.policy.kind]! < order[b.policy.kind]!
        }
    }

    /// The kind the row names: the worst severity, then the furthest through its green window.
    public func worst(now: Date) -> BackupAssessment? {
        assessments(now: now).max { a, b in
            a.severity != b.severity ? a.severity < b.severity : a.urgency < b.urgency
        }
    }

    /// The row's dot, from the age of each kind's last *successful* run. Amber when goback can't be read or no
    /// snapshot backup is configured. The mirror and companions are informational and never count.
    public func severity(now: Date) -> Severity {
        guard report != nil else { return .warning }
        return worst(now: now)?.severity ?? .warning
    }
}

extension OverallStatus {
    /// The overall with the Mac backups row folded in: a red backups row turns an ok icon amber. Amber and not red,
    /// because the red icon and its count mean "an agent failed"; and only red, because goback runs by hand and a
    /// day or two late is a reminder, not an alarm. Nothing else watches these backups, unlike the Pi rows.
    public func including(backups check: MacBackupsCheck?, now: Date) -> OverallStatus {
        self == .ok && check?.severity(now: now) == .failing ? .warning : self
    }
}

public enum BackupStatusParser {
    public struct ParseError: Error, Equatable, CustomStringConvertible {
        public var description: String
    }

    /// `{generated_at, db_path, db_exists, backups: [{profile, backup_type, last_success, latest_attempt,
    /// exit_code, ...}]}` with RFC 3339 times; rows without a profile or type are skipped.
    public static func parse(_ data: Data) throws -> BackupStatusReport {
        guard let object = try? JSONSerialization.jsonObject(with: data), let map = object as? [String: Any],
              let rows = map["backups"] as? [Any] else {
            throw ParseError(description: "not a goback status report")
        }
        let formatter = ISO8601DateFormatter()
        func date(_ value: Any?) -> Date? { (value as? String).flatMap(formatter.date(from:)) }
        let backups = rows.compactMap { value -> BackupRecord? in
            guard let row = value as? [String: Any], let profile = row["profile"] as? String,
                  let type = row["backup_type"] as? String else { return nil }
            return BackupRecord(profile: profile, backupType: type, lastSuccess: date(row["last_success"]),
                                latestAttempt: date(row["latest_attempt"]),
                                exitCode: (row["exit_code"] as? NSNumber)?.intValue)
        }
        return BackupStatusReport(generatedAt: date(map["generated_at"]), dbPath: map["db_path"] as? String ?? "",
                                  dbExists: map["db_exists"] as? Bool ?? false, backups: backups)
    }
}

/// Runs `goback status --json` on this Mac. goback reads `~/.goback.json` and opens `~/.goback.db` read-only, so a
/// check never changes either. GUI apps don't get a login shell's PATH, so the binary is looked for where
/// `just install` (`~/.local/bin`), `go install` and Homebrew put it.
public struct MacBackupsClient: Sendable {
    public static let candidates = ["~/.local/bin/goback", "~/go/bin/goback", "/opt/homebrew/bin/goback",
                                    "/usr/local/bin/goback"]
    public static let arguments = ["status", "--json"]
    /// The database changes only when a backup runs by hand.
    public static let interval: TimeInterval = 300

    public var runner: any CommandRunning
    public var timeout: TimeInterval
    public var candidates: [String]
    public var home: String
    public var isExecutable: @Sendable (String) -> Bool

    public init(runner: any CommandRunning = CommandRunner(environment: HealthCheck.environment()),
                timeout: TimeInterval = 20, candidates: [String] = MacBackupsClient.candidates,
                home: String = NSHomeDirectory(),
                isExecutable: @escaping @Sendable (String) -> Bool = { FileManager.default.isExecutableFile(atPath: $0) }) {
        self.runner = runner
        self.timeout = timeout
        self.candidates = candidates
        self.home = home
        self.isExecutable = isExecutable
    }

    public static func argv(binary: String) -> [String] { [binary] + arguments }

    /// The command as the tooltip shows it.
    public static func command(binary: String?) -> String {
        ([binary ?? "goback"] + arguments).joined(separator: " ")
    }

    /// The first candidate that is an executable file, with `~` expanded.
    public func locate() -> String? {
        candidates.lazy.map { $0.hasPrefix("~/") ? home + $0.dropFirst() : $0 }.first(where: isExecutable)
    }

    /// Blocks for up to `timeout`; call it off the main thread. Never throws.
    public func check(now: () -> Date = Date.init) -> MacBackupsCheck {
        guard let binary = locate() else {
            let places = candidates.map { ($0 as NSString).deletingLastPathComponent }.joined(separator: ", ")
            return MacBackupsCheck(binary: nil, checkedAt: now(), result: .unavailable("goback not found in \(places)"))
        }
        return MacBackupsCheck(binary: binary, checkedAt: now(), result: fetch(binary: binary))
    }

    public func fetch(binary: String) -> MacBackupsResult {
        let result: CommandResult
        do {
            result = try runner.run(Self.argv(binary: binary), timeout: timeout)
        } catch {
            return .unavailable("\(error)")
        }
        if result.timedOut { return .unreadable("goback timed out after \(Int(timeout)) s") }
        switch result.termination {
        case .exited(0):
            do { return .report(try BackupStatusParser.parse(result.stdout)) } catch {
                return .unreadable("goback status output: \(error)")
            }
        case .exited(let code):
            let message = Self.errorMessage(result.stderrText)
            return .unreadable(message.isEmpty ? "goback exited with status \(code)" : message)
        case .signaled(let signal):
            return .unreadable("goback killed by signal \(signal)")
        }
    }

    /// goback's error in one line: cobra's "Error: …" and any "* …" detail lines, without the timestamped copy
    /// log.Fatal repeats.
    static func errorMessage(_ stderr: String) -> String {
        let lines = stderr.split(whereSeparator: \.isNewline).map { $0.trimmingCharacters(in: .whitespaces) }
            .prefix { $0.range(of: #"^\d{4}/\d{2}/\d{2} "#, options: .regularExpression) == nil }
            .filter { !$0.isEmpty }
        guard let first = lines.first else { return "" }
        let head = first.hasPrefix("Error: ") ? String(first.dropFirst(7)) : first
        let details = lines.dropFirst().filter { $0.hasPrefix("* ") }.map { String($0.dropFirst(2)) }
        let message = ([head] + details).joined(separator: " ")
        return message.count > 300 ? String(message.prefix(300)) + "…" : message
    }
}

extension StatusFormatter {
    /// "Mac backups — weekly never recorded", "Mac backups — daily 3 days ago", "Mac backups — goback not found".
    public func macBackupsRow(_ check: MacBackupsCheck?, now: Date) -> String {
        guard let check else { return "Mac backups — checking…" }
        switch check.result {
        case .unavailable: return "Mac backups — goback not found"
        case .unreadable: return "Mac backups — cannot read goback status"
        case .report: break
        }
        guard let worst = check.worst(now: now) else { return "Mac backups — none configured" }
        let profiles = Set(check.assessments(now: now).map(\.record.profile))
        let name = (profiles.count > 1 ? worst.record.profile + " " : "") + worst.policy.kind
        guard let success = worst.record.lastSuccess else {
            return "Mac backups — \(name) " + (worst.record.latestAttempt == nil ? "never recorded" : "never succeeded")
        }
        return "Mac backups — \(name) \(backupAge(success, now: now))"
            + (worst.record.latestFailed ? ", last attempt failed" : "")
    }

    /// "3 days ago" from a day on, otherwise the usual "5 h ago".
    func backupAge(_ date: Date, now: Date) -> String {
        let days = Int(now.timeIntervalSince(date) / 86_400)
        return days >= 1 ? "\(days) \(days == 1 ? "day" : "days") ago" : ago(date, now: now)
    }

    /// "2026-10-06 11:21:24 (2 days ago)".
    func backupDate(_ date: Date, now: Date) -> String {
        "\(stamp(date)) (\(backupAge(date, now: now)))"
    }

    /// "exit 23", "interrupted".
    func backupExit(_ code: Int) -> String { code == -1 ? "interrupted" : "exit \(code)" }

    /// One kind's lines: its last success, then its last attempt when that is a different run or failed.
    func backupRecordLines(_ title: String, _ record: BackupRecord, now: Date) -> [String] {
        guard let attempt = record.latestAttempt else { return ["\(title): never recorded"] }
        var lines = ["\(title): " + (record.lastSuccess.map { "last success \(backupDate($0, now: now))" }
            ?? "no successful run")]
        if record.latestFailed || record.lastSuccess != attempt {
            lines.append("    last attempt \(backupDate(attempt, now: now))"
                + (record.latestFailed ? " · \(backupExit(record.exitCode ?? 0))" : ""))
        }
        return lines
    }

    /// The row's submenu: each daily/weekly/monthly kind, then the mirror and companions (informational), then the
    /// thresholds, database, binary and check time. When goback couldn't be read, why.
    public func macBackupsMenuInfo(_ check: MacBackupsCheck?, now: Date) -> [[String]] {
        guard let check else { return [["Not checked yet"]] }
        var facts: [String] = []
        let thresholds = BackupFreshness.policies.map { "\($0.kind) \($0.okDays)/\($0.warningDays) d" }
        facts.append("Green/amber up to: " + thresholds.joined(separator: " · "))
        switch check.result {
        case .unavailable(let detail), .unreadable(let detail):
            facts.append("goback: " + (check.binary.map(tilde) ?? "not found"))
            facts.append("Checked: \(ago(check.checkedAt, now: now))")
            return [[detail], facts]
        case .report(let report):
            let profiles = Set(check.assessments(now: now).map(\.record.profile))
            var kinds = check.assessments(now: now).flatMap { assessment in
                let title = (profiles.count > 1 ? "\(assessment.record.profile) " : "")
                    + assessment.policy.kind.capitalized
                return backupRecordLines(title, assessment.record, now: now)
            }
            if kinds.isEmpty { kinds = ["No daily, weekly or monthly backup configured"] }
            let info = report.backups.filter { $0.freshness == nil }.flatMap { record -> [String] in
                let title = record.isCompanion
                    ? "Companion \(record.backupType.dropFirst("companion/".count)) (informational)"
                    : record.backupType == "mirror" ? "Mirror (informational)" : "\(record.profile) \(record.backupType)"
                return backupRecordLines(title, record, now: now)
            }
            facts.append("History: \(tilde(report.dbPath))" + (report.dbExists ? "" : " (no backups recorded yet)"))
            facts.append("goback: " + (check.binary.map(tilde) ?? "?"))
            facts.append("Checked: \(ago(check.checkedAt, now: now))")
            return [kinds] + (info.isEmpty ? [] : [info]) + [facts]
        }
    }

    /// The `backups` object in `status --backups --json`.
    public func macBackupsJSON(_ check: MacBackupsCheck, now: Date) -> [String: Any] {
        func record(_ record: BackupRecord) -> [String: Any] {
            [
                "profile": record.profile,
                "backupType": record.backupType,
                "lastSuccess": record.lastSuccess.map(iso) ?? NSNull(),
                "latestAttempt": record.latestAttempt.map(iso) ?? NSNull(),
                "exitCode": record.exitCode ?? NSNull(),
            ]
        }
        var object: [String: Any] = [
            "binary": check.binary ?? NSNull(),
            "checkedAt": iso(check.checkedAt),
            "severity": check.severity(now: now).rawValue,
            "row": macBackupsRow(check, now: now),
            "detail": NSNull(),
            "dbPath": NSNull(),
            "dbExists": NSNull(),
            "kinds": [] as [Any],
            "info": [] as [Any],
        ]
        switch check.result {
        case .unavailable(let detail):
            object["status"] = "unavailable"
            object["detail"] = detail
        case .unreadable(let detail):
            object["status"] = "unreadable"
            object["detail"] = detail
        case .report(let report):
            object["status"] = "ok"
            object["dbPath"] = report.dbPath
            object["dbExists"] = report.dbExists
            object["kinds"] = check.assessments(now: now).map { assessment -> [String: Any] in
                record(assessment.record).merging([
                    "severity": assessment.severity.rawValue,
                    "okDays": assessment.policy.okDays,
                    "warningDays": assessment.policy.warningDays,
                ]) { $1 }
            }
            object["info"] = report.backups.filter { $0.freshness == nil }.map(record)
        }
        return object
    }
}
