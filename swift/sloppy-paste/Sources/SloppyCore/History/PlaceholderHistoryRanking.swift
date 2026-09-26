import Foundation

/// Ranking and statistics over remembered placeholder values.
public enum PlaceholderHistoryRanking {
    static let oneDayMs: Double = 24 * 60 * 60 * 1000
    static let frequencyWeight = 1.0
    static let recencyWeight = 2.0

    /// Highest score first, where
    /// `score = useCount + 2 * 100 / max(1, daysSinceLastUse)`.
    public static func rank(_ values: [PlaceholderHistoryValue], now: Int64) -> [PlaceholderHistoryValue] {
        values
            .map { value in
                let daysAgo = max(1, Double(now - value.lastUsed) / oneDayMs)
                let score = frequencyWeight * Double(value.useCount) + recencyWeight * (1 / daysAgo) * 100
                return (value, score)
            }
            .stableSorted { $0.1 > $1.1 }
            .map(\.0)
    }

    /// Ranked value strings for autocomplete, optionally capped at `limit`.
    public static func rankedValues(_ values: [PlaceholderHistoryValue], now: Int64, limit: Int? = nil) -> [String] {
        let ranked = rank(values, now: now).map(\.value)
        guard let limit, limit > 0 else { return ranked }
        return Array(ranked.prefix(limit))
    }

    /// Case-insensitive substring filter; a blank query keeps everything.
    public static func filter(_ values: [String], query: String) -> [String] {
        guard !query.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return values }
        let lowered = query.lowercased()
        return values.filter { $0.lowercased().contains(lowered) }
    }

    /// The best-ranked value, used to pre-fill a form field.
    public static func topRankedValue(_ values: [PlaceholderHistoryValue], now: Int64) -> String? {
        rank(values, now: now).first?.value
    }

    /// The value with the latest `lastUsed`, ignoring frequency; the first wins a tie.
    public static func lastUsedValue(_ values: [PlaceholderHistoryValue]) -> String? {
        values.reduce(nil as PlaceholderHistoryValue?) { latest, current in
            guard let latest, current.lastUsed <= latest.lastUsed else { return current }
            return latest
        }?.value
    }
}

/// Summary of one placeholder key's history.
public struct PlaceholderKeyStats: Sendable, Hashable {
    public var key: String
    public var valueCount: Int
    public var totalUseCount: Int
    /// nil means no recorded use; 0 is a valid (epoch) timestamp.
    public var lastUsed: Int64?

    public init(key: String, valueCount: Int, totalUseCount: Int, lastUsed: Int64?) {
        self.key = key
        self.valueCount = valueCount
        self.totalUseCount = totalUseCount
        self.lastUsed = lastUsed
    }

    public init(key: String, values: [PlaceholderHistoryValue]) {
        self.init(
            key: key,
            valueCount: values.count,
            totalUseCount: values.reduce(0) { $0 + $1.useCount },
            lastUsed: values.map(\.lastUsed).max()
        )
    }
}

public enum PlaceholderKeySortOption: String, Sendable, CaseIterable {
    case nameAsc = "name-asc"
    case valueCountDesc = "value-count-desc"
    case usageDesc = "usage-desc"
    case lastUsedDesc = "last-used-desc"
}

extension PlaceholderKeyStats {
    /// Stable sort; for `lastUsedDesc`, keys without a recorded use go last.
    public static func sorted(_ stats: [PlaceholderKeyStats], by option: PlaceholderKeySortOption) -> [PlaceholderKeyStats] {
        switch option {
        case .nameAsc:
            stats.stableSorted { TagNormalization.localeLess($0.key, $1.key) }
        case .valueCountDesc:
            stats.stableSorted { $0.valueCount > $1.valueCount }
        case .usageDesc:
            stats.stableSorted { $0.totalUseCount > $1.totalUseCount }
        case .lastUsedDesc:
            stats.stableSorted { a, b in
                switch (a.lastUsed, b.lastUsed) {
                case (nil, _): false
                case (_, nil): true
                case let (at?, bt?): at > bt
                }
            }
        }
    }
}
