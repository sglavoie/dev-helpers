import Foundation

/// Tag normalisation shared by storage, migrations and import.
public enum TagNormalization {
    public static func normalizeTag(_ tag: String) -> String {
        tag.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    }

    /// Normalises every tag and drops the ones that end up empty.
    public static func normalizeTags(_ tags: [String]) -> [String] {
        tags.map(normalizeTag).filter { !$0.isEmpty }
    }

    /// Removes duplicates after normalisation, keeping first-seen order.
    public static func deduplicateTags(_ tags: [String]) -> [String] {
        var seen = Set<String>()
        return tags.map(normalizeTag).filter { seen.insert($0).inserted }
    }

    /// Normalises, deduplicates, drops parents that have a child in the list, and sorts.
    public static func removeRedundantParents(_ tags: [String]) -> [String] {
        let normalized = deduplicateTags(normalizeTags(tags))
        return normalized
            .filter { tag in !normalized.contains { $0 != tag && $0.hasPrefix(tag + "/") } }
            .sorted(by: localeLess)
    }

    /// Ordering equivalent to JS `a.localeCompare(b) < 0`.
    public static func localeLess(_ a: String, _ b: String) -> Bool {
        a.localizedCompare(b) == .orderedAscending
    }
}
