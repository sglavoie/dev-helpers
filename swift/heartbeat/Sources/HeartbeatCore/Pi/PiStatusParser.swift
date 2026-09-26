import Foundation

/// A level as `pi-status` writes it, for the whole report and for each section.
public enum PiLevel: String, Codable, Sendable, CaseIterable {
    case ok
    case unknown
    case warn
    case fail
}

/// What the Pi row shows from one `pi-status --json` report.
public struct PiSummary: Equatable, Codable, Sendable {
    public var status: PiLevel
    /// When pi-status built the report (its `generated` Unix time).
    public var generated: Date?
    /// Kuma monitors reporting up, and all monitors with a heartbeat; `nil` when the Kuma section has no counts.
    public var kumaUp: Int?
    public var kumaTotal: Int?
    /// One line per problem, in pi-status's section order (Kuma, containers, systemd, backup, host, journal).
    public var problems: [String]

    public init(status: PiLevel, generated: Date? = nil, kumaUp: Int? = nil, kumaTotal: Int? = nil, problems: [String] = []) {
        self.status = status
        self.generated = generated
        self.kumaUp = kumaUp
        self.kumaTotal = kumaTotal
        self.problems = problems
    }
}

public enum PiStatusParseError: Error, Equatable, Sendable, CustomStringConvertible {
    case notJSON
    case missingStatus
    case unknownStatus(String)

    public var description: String {
        switch self {
        case .notJSON: "not a JSON object"
        case .missingStatus: "no \"status\" key"
        case .unknownStatus(let status): "unknown status \"\(status)\""
        }
    }
}

/// Parses `~/.local/bin/pi-status --json` on the Pi:
/// `{"status": "ok|unknown|warn|fail", "generated": 1790466071.77, "sections": {"kuma": {"status", "counts", "problems"},
/// "containers": {"status", "containers"}, "systemd": {"status", "failed", "timers"}, "backup", "host", "errors"}}`.
/// A section that couldn't be collected is `{"status": "unknown|fail", "reason": "..."}`.
public enum PiStatusParser {
    /// Section order and titles, as pi-status prints them.
    static let sections: [(key: String, title: String)] = [
        ("kuma", "Kuma"), ("containers", "Containers"), ("systemd", "systemd"), ("backup", "Backup"), ("host", "Host"),
        ("errors", "Journal"),
    ]
    /// Longest Kuma or journal message kept in a problem line.
    static let messageLimit = 80

    public static func parse(_ data: Data) throws -> PiSummary {
        guard let object = try? JSONSerialization.jsonObject(with: data), let json = object as? [String: Any] else {
            throw PiStatusParseError.notJSON
        }
        guard let rawStatus = json["status"] as? String else { throw PiStatusParseError.missingStatus }
        guard let status = PiLevel(rawValue: rawStatus) else { throw PiStatusParseError.unknownStatus(rawStatus) }

        let sections = json["sections"] as? [String: Any] ?? [:]
        var summary = PiSummary(status: status, generated: number(json["generated"]).map(Date.init(timeIntervalSince1970:)))
        if let counts = (sections["kuma"] as? [String: Any])?["counts"] as? [String: Any] {
            summary.kumaUp = int(counts["up"]) ?? 0
            summary.kumaTotal = counts.values.compactMap(int).reduce(0, +)
        }
        let known = Set(Self.sections.map(\.key))
        let order = Self.sections + sections.keys.filter { !known.contains($0) }.sorted().map { ($0, $0) }
        for (key, title) in order {
            guard let section = sections[key] as? [String: Any] else { continue }
            summary.problems += problems(key, title: title, section)
        }
        return summary
    }

    /// Problem lines for one section; a non-ok section always yields at least one.
    static func problems(_ key: String, title: String, _ section: [String: Any]) -> [String] {
        let level = (section["status"] as? String).map { PiLevel(rawValue: $0) ?? .unknown } ?? .unknown
        guard level != .ok else { return [] }
        if let reason = section["reason"] as? String { return ["\(title): \(reason)"] }
        let lines: [String] = switch key {
        case "kuma": kuma(section)
        case "containers": containers(section)
        case "systemd": systemd(section)
        case "backup": backup(section)
        case "host": host(section)
        case "errors": journal(section)
        default: []
        }
        return lines.isEmpty ? ["\(title): \(level.rawValue)"] : lines
    }

    static func kuma(_ section: [String: Any]) -> [String] {
        objects(section["problems"]).map { problem in
            let name = problem["name"] as? String ?? "?"
            let state = problem["state"] as? String ?? "unknown"
            let message = clip(problem["message"] as? String ?? "")
            return "Kuma: \(name) \(state)" + (message.isEmpty ? "" : " — \(message)")
        }
    }

    static func containers(_ section: [String: Any]) -> [String] {
        objects(section["containers"]).filter { ($0["level"] as? String) != "ok" }.map { row in
            let status = (row["status"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? row["state"] as? String ?? "?"
            return "Container \(row["name"] as? String ?? "?"): \(status)"
        }
    }

    static func systemd(_ section: [String: Any]) -> [String] {
        let failed = objects(section["failed"])
        var lines = failed.map { "systemd: \($0["unit"] as? String ?? "?") failed (\($0["scope"] as? String ?? "?"))" }
        let listed = Set(failed.compactMap { $0["unit"] as? String })
        for timer in objects(section["timers"]) where (timer["level"] as? String) != "ok" {
            let service = (timer["service"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? timer["timer"] as? String ?? "?"
            guard !listed.contains(service) else { continue }
            let result = timer["result"] as? String ?? "?"
            let exit = timer["exit_status"].map { " (exit \($0))" } ?? ""
            lines.append("systemd: \(service) \(result)\(exit)")
        }
        return lines
    }

    static func backup(_ section: [String: Any]) -> [String] {
        var lines: [String] = []
        if section["verified"] as? Bool == false {
            lines.append("Backup: \(section["id"] as? String ?? "latest") not verified")
        } else if let age = number(section["age"]) {
            lines.append("Backup: last verified \(ScheduleDescription.age(Int(age))) ago")
        }
        if let attempt = section["attempt"] as? String {
            lines.append("Backup: latest attempt \(attempt)")
        }
        return lines
    }

    /// The host metrics past pi-status's warning thresholds.
    static func host(_ section: [String: Any]) -> [String] {
        var parts: [String] = []
        if let load = section["load"] as? [Any], load.count > 1, let average = number(load[1]),
           let cores = int(section["cores"]), average > Double(cores) {
            parts.append("load \(String(format: "%.2f", average)) on \(cores) cores")
        }
        if let memory = section["memory"] as? [String: Any], let total = number(memory["total"]),
           let available = number(memory["available"]), available < total * 0.1 {
            parts.append("memory \(Int((available / total * 100).rounded()))% available")
        }
        if let disk = section["disk"] as? [String: Any], let used = number(disk["used_fraction"]), used >= 0.85 {
            parts.append("disk \(Int((used * 100).rounded()))% used")
        }
        if let temperature = number(section["temperature_c"]), temperature >= 70 {
            parts.append("\(String(format: "%.1f", temperature)) °C")
        }
        if let throttled = section["throttled"] as? String, throttled != "0x0", !throttled.isEmpty {
            parts.append("throttled \(throttled)")
        }
        return parts.isEmpty ? [] : ["Host: " + parts.joined(separator: ", ")]
    }

    static func journal(_ section: [String: Any]) -> [String] {
        let count = (section["lines"] as? [Any])?.count ?? 0
        var line = "Journal: \(count) \(count == 1 ? "error" : "errors") in the last hour"
        if let message = objects(section["distinct"]).last?["message"] as? String, !message.isEmpty {
            line += " — \(clip(message))"
        }
        return [line]
    }

    // MARK: Helpers

    static func objects(_ value: Any?) -> [[String: Any]] {
        (value as? [Any])?.compactMap { $0 as? [String: Any] } ?? []
    }

    /// A JSON number that isn't a boolean.
    static func number(_ value: Any?) -> Double? {
        guard let number = value as? NSNumber, CFGetTypeID(number) != CFBooleanGetTypeID() else { return nil }
        return number.doubleValue
    }

    static func int(_ value: Any?) -> Int? {
        number(value).map { Int($0) }
    }

    static func clip(_ text: String) -> String {
        let line = text.split(whereSeparator: \.isNewline).first.map(String.init) ?? ""
        let trimmed = line.trimmingCharacters(in: .whitespaces)
        return trimmed.count > messageLimit ? String(trimmed.prefix(messageLimit - 1)) + "…" : trimmed
    }
}
