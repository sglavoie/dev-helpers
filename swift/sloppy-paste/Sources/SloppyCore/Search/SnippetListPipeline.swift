import Foundation

/// The toggles and dropdowns that sit beside the search field.
public struct SnippetFilterOptions: Sendable, Hashable {
    /// nil shows every tag; `TagHierarchy.untaggedSentinel` shows untagged snippets.
    public var selectedTag: String?
    public var showOnlyFavorites: Bool
    /// Shows only archived snippets instead of only unarchived ones.
    public var showArchivedSnippets: Bool
    public var showNeedsAttention: Bool

    public init(
        selectedTag: String? = nil, showOnlyFavorites: Bool = false, showArchivedSnippets: Bool = false,
        showNeedsAttention: Bool = false
    ) {
        self.selectedTag = selectedTag
        self.showOnlyFavorites = showOnlyFavorites
        self.showArchivedSnippets = showArchivedSnippets
        self.showNeedsAttention = showNeedsAttention
    }
}

/// The sections of the root list, in display order.
public struct SnippetList: Sendable, Hashable {
    /// Everything that passed the filters, in storage order.
    public var filtered: [Snippet]
    /// Pinned snippets, alphabetical.
    public var pinned: [Snippet]
    /// Up to five recently used snippets that aren't pinned (empty when the section is off).
    public var recent: [Snippet]
    /// The rest, in the chosen sort order.
    public var sorted: [Snippet]
    /// Tags on the filtered snippets, with their parents.
    public var visibleTags: [String]
    /// Tags on all snippets, with their parents.
    public var allTags: [String]
    public var hasStructuredOperators: Bool

    /// Every row in display order.
    public var rows: [Snippet] { pinned + recent + sorted }
}

/// Ports `useSnippetFiltering`: filter, then split into pinned, recent and sorted sections.
public enum SnippetListPipeline {
    public static let recentLimit = 5

    public static func build(
        _ snippets: [Snippet], query: String, options: SnippetFilterOptions = SnippetFilterOptions(),
        sort: SortOption = .updatedDesc, showRecentSection: Bool = true, now: Int64
    ) -> SnippetList {
        let parsed = QueryParser.parse(query)
        let filtered = filter(snippets, query: parsed, options: options, now: now)

        let pinned = stableSorted(filtered.filter(\.isPinned)) { $0.title.localizedCompare($1.title) }
        let pinnedIDs = Set(pinned.map(\.id))
        let recent = showRecentSection ? recentSnippets(filtered, excluding: pinnedIDs) : []
        let recentIDs = Set(recent.map(\.id))
        let remaining = filtered.filter { !pinnedIDs.contains($0.id) && !recentIDs.contains($0.id) }

        return SnippetList(
            filtered: filtered,
            pinned: pinned,
            recent: recent,
            sorted: self.sorted(remaining, by: sort),
            visibleTags: TagHierarchy.expandTagsWithParents(Array(Set(filtered.flatMap(\.tags)))),
            allTags: TagHierarchy.expandTagsWithParents(Array(Set(snippets.flatMap(\.tags)))),
            hasStructuredOperators: parsed.hasStructuredOperators)
    }

    /// The archive view, tag dropdown and favorites toggle apply before the
    /// query, so they keep working once the query holds operators such as `tag:foo`.
    public static func filter(
        _ snippets: [Snippet], query: ParsedQuery, options: SnippetFilterOptions, now: Int64
    ) -> [Snippet] {
        var result = snippets.filter { $0.isArchived == options.showArchivedSnippets }
        if let tag = options.selectedTag {
            result = TagHierarchy.filterSnippets(result, byTag: tag)
        }
        if options.showOnlyFavorites {
            result = result.filter(\.isFavorite)
        }
        if query.hasOperators {
            result = SearchFilter.apply(result, query: query)
        }
        if options.showNeedsAttention {
            result = result.filter { Staleness.analyze($0, now: now).isStale }
        }
        return result
    }

    /// The most recently used snippets, newest first. A `lastUsedAt` of 0 still counts as used.
    public static func recentSnippets(_ filtered: [Snippet], excluding pinnedIDs: Set<String>, limit: Int = recentLimit) -> [Snippet] {
        let used = filtered.filter { $0.lastUsedAt != nil && !pinnedIDs.contains($0.id) }
        return Array(stableSorted(used) { compare($1.lastUsedAt ?? 0, $0.lastUsedAt ?? 0) }.prefix(limit))
    }

    public static func sorted(_ snippets: [Snippet], by sort: SortOption) -> [Snippet] {
        switch sort {
        case .updatedDesc: stableSorted(snippets) { compare(lastActivity($1), lastActivity($0)) }
        case .mostUsedDesc: stableSorted(snippets) { compare($1.useCount, $0.useCount) }
        case .alphabetical: stableSorted(snippets) { $0.title.localizedCompare($1.title) }
        case .createdDesc: stableSorted(snippets) { compare($1.createdAt, $0.createdAt) }
        }
    }

    static func lastActivity(_ snippet: Snippet) -> Int64 {
        max(snippet.updatedAt, snippet.lastUsedAt ?? 0)
    }

    static func compare<T: Comparable>(_ a: T, _ b: T) -> ComparisonResult {
        a < b ? .orderedAscending : a > b ? .orderedDescending : .orderedSame
    }

    /// A sort that keeps ties in their original order, like JS `Array.prototype.sort`.
    static func stableSorted(_ snippets: [Snippet], by order: (Snippet, Snippet) -> ComparisonResult) -> [Snippet] {
        snippets.enumerated()
            .sorted { lhs, rhs in
                switch order(lhs.element, rhs.element) {
                case .orderedAscending: true
                case .orderedDescending: false
                case .orderedSame: lhs.offset < rhs.offset
                }
            }
            .map(\.element)
    }
}
