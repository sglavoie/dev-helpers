import Foundation

/// The newest sign that an agent ran. launchd keeps no "last run time", so the caller combines
/// stdout/stderr log mtimes, config `evidencePaths` and the ledger time when `runs` went up.
public struct Evidence: Equatable, Codable, Sendable {
    /// Whether the agent has any evidence source at all (logs, evidence paths or a runs ledger).
    public var hasSource: Bool
    /// The newest timestamp across the sources; `nil` when none of them has produced one yet.
    public var latest: Date?
    /// Where `latest` came from (a path or "runs ledger"), for `explain`.
    public var origin: String?

    public init(hasSource: Bool, latest: Date? = nil, origin: String? = nil) {
        self.hasSource = hasSource
        self.latest = latest
        self.origin = origin
    }

    public static let none = Evidence(hasSource: false)
}
