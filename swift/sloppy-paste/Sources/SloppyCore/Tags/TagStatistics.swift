import Foundation

/// Usage figures for one tag, for the Manage Tags view.
public struct TagStatistics: Sendable, Hashable {
    public var tag: String
    public var snippetCount: Int
    /// Most recent `lastUsedAt` among the tag's snippets; nil when none was used.
    public var lastUsedAt: Int64?
    public var totalUsageCount: Int

    public var neverUsed: Bool { lastUsedAt == nil }

    public init(tag: String, snippetCount: Int, lastUsedAt: Int64?, totalUsageCount: Int) {
        self.tag = tag
        self.snippetCount = snippetCount
        self.lastUsedAt = lastUsedAt
        self.totalUsageCount = totalUsageCount
    }

    /// Statistics for each of `tags`, in the same order. Matches exact tags only.
    public static func compute(snippets: [Snippet], tags: [String]) -> [TagStatistics] {
        tags.map { tag in
            let matching = snippets.filter { $0.tags.contains(tag) }
            return TagStatistics(
                tag: tag,
                snippetCount: matching.count,
                lastUsedAt: matching.compactMap(\.lastUsedAt).max(),
                totalUsageCount: matching.reduce(0) { $0 + $1.useCount }
            )
        }
    }
}

extension TagStatistics {
    /// Tags of `tags` that no unarchived snippet carries (TS getUnusedTags),
    /// counted in the root "Manage Tags (n unused)" action.
    public static func unusedTags(snippets: [Snippet], tags: [String]) -> [String] {
        let used = Set(snippets.lazy.filter { !$0.isArchived }.flatMap(\.tags))
        return tags.filter { !used.contains($0) }
    }
}

public enum TagSortOption: String, Sendable, CaseIterable {
    case nameAsc = "name-asc"
    case snippetCountDesc = "snippet-count-desc"
    case lastUsedDesc = "last-used-desc"
    case lastUsedAsc = "last-used-asc"
    case usageCountDesc = "usage-count-desc"
    case neverUsedFirst = "never-used-first"

    public var label: String {
        switch self {
        case .nameAsc: "Name (A-Z)"
        case .snippetCountDesc: "Most Snippets"
        case .lastUsedDesc: "Recently Used"
        case .lastUsedAsc: "Least Recently Used"
        case .usageCountDesc: "Most Used"
        case .neverUsedFirst: "Never Used First"
        }
    }
}

extension TagStatistics {
    /// Sorts statistics; ties fall back to tag name.
    public static func sorted(_ stats: [TagStatistics], by option: TagSortOption) -> [TagStatistics] {
        let byName: (TagStatistics, TagStatistics) -> Bool = { TagNormalization.localeLess($0.tag, $1.tag) }

        switch option {
        case .nameAsc:
            return stats.stableSorted(by: byName)
        case .snippetCountDesc:
            return stats.stableSorted { a, b in
                a.snippetCount != b.snippetCount ? a.snippetCount > b.snippetCount : byName(a, b)
            }
        case .lastUsedDesc:
            // Never-used tags go last.
            return stats.stableSorted { a, b in
                switch (a.lastUsedAt, b.lastUsedAt) {
                case (nil, nil): byName(a, b)
                case (nil, _): false
                case (_, nil): true
                case let (at?, bt?): at != bt ? at > bt : byName(a, b)
                }
            }
        case .lastUsedAsc:
            // Never-used tags go first: the strongest rediscovery signal.
            return stats.stableSorted { a, b in
                switch (a.lastUsedAt, b.lastUsedAt) {
                case (nil, nil): byName(a, b)
                case (nil, _): true
                case (_, nil): false
                case let (at?, bt?): at != bt ? at < bt : byName(a, b)
                }
            }
        case .usageCountDesc:
            return stats.stableSorted { a, b in
                a.totalUsageCount != b.totalUsageCount ? a.totalUsageCount > b.totalUsageCount : byName(a, b)
            }
        case .neverUsedFirst:
            return stats.stableSorted { a, b in
                a.neverUsed != b.neverUsed ? a.neverUsed : byName(a, b)
            }
        }
    }
}
