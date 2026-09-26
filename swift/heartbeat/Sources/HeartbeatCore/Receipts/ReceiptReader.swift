import Foundation

/// What a job's receipt file says about its last run (rule 8).
public enum ReceiptStatus: Equatable, Codable, Sendable {
    /// Reported to Kuma with an ok status.
    case ok(finished: Date?)
    /// The job ran but couldn't report to Uptime Kuma.
    case notReported(finished: Date?)
    /// The job reported a status outside `okValues`, e.g. "down".
    case badStatus(String, finished: Date?)
    case missing(path: String)
    /// The file exists but isn't a JSON object with the configured keys.
    case unreadable(reason: String)

    /// The receipt's `finished` time, usable as evidence of a run.
    public var finished: Date? {
        switch self {
        case .ok(let finished), .notReported(let finished), .badStatus(_, let finished): finished
        case .missing, .unreadable: nil
        }
    }
}

/// Reads a JSON receipt such as pi-backup-fetch's
/// `{"finished": "...", "msg": "...", "reported": true, "status": "up"}`.
public enum ReceiptReader {
    public static func read(_ config: ReceiptConfig) -> ReceiptStatus {
        let data: Data
        do {
            data = try Data(contentsOf: URL(fileURLWithPath: config.path))
        } catch CocoaError.fileReadNoSuchFile {
            return .missing(path: config.path)
        } catch {
            return .unreadable(reason: error.localizedDescription)
        }
        return parse(data, config: config)
    }

    public static func parse(_ data: Data, config: ReceiptConfig) -> ReceiptStatus {
        guard let object = try? JSONSerialization.jsonObject(with: data), let json = object as? [String: Any] else {
            return .unreadable(reason: "not a JSON object")
        }
        let finished = (json["finished"] as? String).flatMap(parseDate)

        // A status outside okValues is worse than a missing report, so check it first.
        guard let rawStatus = json[config.statusKey] else {
            return .unreadable(reason: "no \"\(config.statusKey)\" key")
        }
        guard let status = rawStatus as? String else {
            return .unreadable(reason: "\"\(config.statusKey)\" is not a string")
        }
        if !config.okValues.contains(status) {
            return .badStatus(status, finished: finished)
        }

        guard let rawReported = json[config.reportedKey] else {
            return .unreadable(reason: "no \"\(config.reportedKey)\" key")
        }
        // JSONSerialization gives NSNumber for both booleans and numbers; accept only real booleans.
        guard let number = rawReported as? NSNumber, CFGetTypeID(number) == CFBooleanGetTypeID() else {
            return .unreadable(reason: "\"\(config.reportedKey)\" is not a boolean")
        }
        return number.boolValue ? .ok(finished: finished) : .notReported(finished: finished)
    }

    /// ISO 8601 with or without fractional seconds (Python's `isoformat()` writes microseconds).
    static func parseDate(_ text: String) -> Date? {
        let fractional = ISO8601DateFormatter()
        fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let date = fractional.date(from: text) { return date }
        return ISO8601DateFormatter().date(from: text)
    }
}
