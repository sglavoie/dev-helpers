import Foundation

/// Logic behind the root screen's ⌘G tag filter chooser: which filters to
/// offer, with how many snippets each would show.
public enum TagFilterChooser {
    public enum Entry: Sendable, Hashable, Identifiable {
        case allTags
        case untagged(count: Int)
        case tag(String, count: Int)

        public var id: String {
            switch self {
            case .allTags: "all"
            case .untagged: "untagged"
            case .tag(let tag, _): "tag:\(tag)"
            }
        }

        /// The `SnippetFilterOptions.selectedTag` value this entry applies.
        public var selectedTag: String? {
            switch self {
            case .allTags: nil
            case .untagged: TagHierarchy.untaggedSentinel
            case .tag(let tag, _): tag
            }
        }
    }

    /// All Tags, Untagged, then every tag with its parents, sorted by path.
    /// Counts cover the snippets in the current archive view only and include
    /// child tags, matching what the filter shows. Other filters and the query
    /// are ignored so every tag stays reachable. The selected tag stays listed
    /// even when no snippet in the view carries it.
    public static func entries(
        snippets: [Snippet], showArchived: Bool, selectedTag: String?, searchText: String
    ) -> [Entry] {
        let pool = snippets.filter { $0.isArchived == showArchived }
        var tags = TagHierarchy.expandTagsWithParents(pool.flatMap(\.tags))
        if let selectedTag, selectedTag != TagHierarchy.untaggedSentinel, !tags.contains(selectedTag) {
            tags = TagHierarchy.expandTagsWithParents(tags + [selectedTag])
        }

        let query = searchText.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        func matches(_ text: String) -> Bool { query.isEmpty || text.lowercased().contains(query) }

        var entries: [Entry] = []
        if matches("All Tags") {
            entries.append(.allTags)
        }
        let untaggedCount = TagHierarchy.filterSnippets(pool, byTag: TagHierarchy.untaggedSentinel).count
        if matches("Untagged"), untaggedCount > 0 || selectedTag == TagHierarchy.untaggedSentinel {
            entries.append(.untagged(count: untaggedCount))
        }
        for tag in tags where matches(tag) {
            entries.append(.tag(tag, count: TagHierarchy.filterSnippets(pool, byTag: tag).count))
        }
        return entries
    }
}
