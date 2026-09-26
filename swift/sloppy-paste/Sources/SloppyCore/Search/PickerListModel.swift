import Foundation

/// One section of the picker's root list.
public struct PickerListSection: Sendable, Hashable, Identifiable {
    public enum Kind: Sendable, Hashable {
        case pinned, recent, main
    }

    public var kind: Kind
    /// nil hides the header (a plain list with no pinned or recent rows).
    public var title: String?
    public var subtitle: String?
    public var snippets: [Snippet]

    public var id: Kind { kind }

    public init(kind: Kind, title: String?, subtitle: String?, snippets: [Snippet]) {
        self.kind = kind
        self.title = title
        self.subtitle = subtitle
        self.snippets = snippets
    }
}

/// The empty-list message, which depends on the active view filter.
public struct PickerEmptyState: Sendable, Hashable {
    public var title: String
    public var message: String

    public init(title: String, message: String) {
        self.title = title
        self.message = message
    }
}

/// Everything the root screen renders for one query and filter state.
public struct PickerListState: Sendable, Hashable {
    public var list: SnippetList
    public var suggestions: [SearchSuggestion]
    public var sections: [PickerListSection]
    public var searchPlaceholder: String
    /// Set when no snippet row is shown.
    public var emptyState: PickerEmptyState?

    /// Snippet rows in display order.
    public var rows: [Snippet] { sections.flatMap(\.snippets) }
}

/// Ports the section and label logic of the extension's `index.tsx` root list.
public enum PickerListModel {
    /// The recent section needs this many unarchived snippets to be worth showing.
    public static let recentSectionMinimumSnippets = 3

    public static func build(
        _ snippets: [Snippet], query: String, options: SnippetFilterOptions = SnippetFilterOptions(),
        sort: SortOption = .updatedDesc, showRecentSection: Bool = true, now: Int64
    ) -> PickerListState {
        // TS computed the recent snippets and then hid the section, which dropped
        // them from the list; deciding up front keeps them in the main section.
        let recentVisible = showRecentSection && !options.showArchivedSnippets && !options.showNeedsAttention
            && snippets.count(where: { !$0.isArchived }) >= recentSectionMinimumSnippets
        let list = SnippetListPipeline.build(
            snippets, query: query, options: options, sort: sort, showRecentSection: recentVisible, now: now)

        var sections: [PickerListSection] = []
        if !list.pinned.isEmpty {
            sections.append(PickerListSection(
                kind: .pinned, title: "Pinned", subtitle: count(list.pinned.count), snippets: list.pinned))
        }
        if !list.recent.isEmpty {
            sections.append(PickerListSection(
                kind: .recent, title: "Recently Used", subtitle: count(list.recent.count), snippets: list.recent))
        }
        if !list.sorted.isEmpty {
            sections.append(PickerListSection(
                kind: .main,
                title: mainSectionTitle(
                    options: options, filteredCount: list.filtered.count,
                    hasPinnedOrRecent: !list.pinned.isEmpty || !list.recent.isEmpty,
                    hasStructuredOperators: list.hasStructuredOperators),
                subtitle: nil, snippets: list.sorted))
        }

        return PickerListState(
            list: list,
            suggestions: SearchSuggestions.suggestions(
                for: query, allTags: list.allTags, allContexts: TitleContext.allContexts(snippets)),
            sections: sections,
            searchPlaceholder: searchPlaceholder(
                options: options, hasStructuredOperators: list.hasStructuredOperators),
            emptyState: sections.isEmpty
                ? emptyState(options: options, hasQuery: !QueryParser.parse(query).isEmpty) : nil)
    }

    public static func searchPlaceholder(options: SnippetFilterOptions, hasStructuredOperators: Bool) -> String {
        if options.showOnlyFavorites { return "★ Bookmarked — ⇧⌘F to show all" }
        if options.showArchivedSnippets { return "⊟ Archived — ⌘B to show all" }
        if options.showNeedsAttention { return "⚠ Needs Attention — ⇧⌘N to show all" }
        if hasStructuredOperators { return "Operators active" }
        return "Search… (tag:, ctx:, is:, not:, \"exact\")"
    }

    public static func mainSectionTitle(
        options: SnippetFilterOptions, filteredCount: Int, hasPinnedOrRecent: Bool, hasStructuredOperators: Bool
    ) -> String? {
        let title: String? =
            if options.showArchivedSnippets { "⊟ Archived (\(filteredCount))" }
            else if options.showNeedsAttention { "⚠ Needs Attention (\(filteredCount))" }
            else if options.showOnlyFavorites { "★ Bookmarked (\(filteredCount))" }
            else if hasPinnedOrRecent { "All Snippets" }
            else { nil }
        if hasStructuredOperators && !options.showArchivedSnippets && !options.showNeedsAttention
            && !options.showOnlyFavorites
        {
            return "\(title ?? "All Snippets") — operators active"
        }
        return title
    }

    public static func emptyState(options: SnippetFilterOptions, hasQuery: Bool = false) -> PickerEmptyState {
        if hasQuery {
            return PickerEmptyState(title: "No matching snippets", message: "Try fewer words or different operators")
        }
        if options.showOnlyFavorites {
            return PickerEmptyState(
                title: "No bookmarks yet",
                message: "Bookmark snippets with ⇧⌘V or press ⇧⌘F to view all snippets")
        }
        if options.showNeedsAttention {
            return PickerEmptyState(
                title: "No snippets need attention",
                message: "All snippets are in good shape. Press ⇧⌘N to return to the full list")
        }
        return PickerEmptyState(title: "No snippets yet", message: "Press ⌘N to create a snippet or ⇧⌘I to import")
    }

    /// Number of required inputs the user must fill (system placeholders excluded), for the `⌨ n` badge.
    public static func requiredInputCount(_ snippet: Snippet) -> Int {
        let systemKeys = Set(SystemPlaceholders.names)
        return PlaceholderSyntaxParser.extractPlaceholders(snippet.content)
            .count { $0.isRequired && !systemKeys.contains($0.key) }
    }

    private static func count(_ n: Int) -> String {
        "\(n) snippet\(n == 1 ? "" : "s")"
    }
}
