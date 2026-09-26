import Foundation

/// Staleness of one snippet, for the stale badge and the "needs attention" filter.
public struct SnippetAnalytics: Sendable, Hashable {
    public var snippet: Snippet
    /// Whole days since the last use; nil when the snippet was never used.
    public var daysUnused: Int?
    public var isStale: Bool
    public var stalenessReason: String?

    public init(snippet: Snippet, daysUnused: Int?, isStale: Bool, stalenessReason: String?) {
        self.snippet = snippet
        self.daysUnused = daysUnused
        self.isStale = isStale
        self.stalenessReason = stalenessReason
    }
}

public enum Staleness {
    static let msPerDay: Int64 = 24 * 60 * 60 * 1000
    public static let staleThresholdDays = 90
    public static let neverUsedAgeThresholdDays = 30

    /// A snippet is stale when it was never used and is at least 30 days old,
    /// or was last used 90 or more days ago. Pinned snippets are never stale.
    public static func analyze(_ snippet: Snippet, now: Int64) -> SnippetAnalytics {
        let daysUnused = snippet.lastUsedAt.map { Int(Formatting.floorDiv(now - $0, msPerDay)) }
        let daysSinceCreation = Int(Formatting.floorDiv(now - snippet.createdAt, msPerDay))

        var reason: String?
        if !snippet.isPinned {
            if snippet.useCount == 0 && daysSinceCreation >= neverUsedAgeThresholdDays {
                reason = "Never used (created \(daysSinceCreation) days ago)"
            } else if let daysUnused, daysUnused >= staleThresholdDays {
                reason = "Not used in \(daysUnused) days"
            }
        }
        return SnippetAnalytics(snippet: snippet, daysUnused: daysUnused, isStale: reason != nil, stalenessReason: reason)
    }
}
