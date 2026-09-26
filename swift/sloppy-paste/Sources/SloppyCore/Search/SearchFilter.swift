import Foundation

/// Applies a `ParsedQuery` to snippets. Every condition must hold (AND).
public enum SearchFilter {
    public static func apply(_ snippets: [Snippet], query: ParsedQuery) -> [Snippet] {
        snippets.filter { matches($0, query: query) }
    }

    public static func matches(_ snippet: Snippet, query: ParsedQuery) -> Bool {
        // Tags are hierarchical: tag:work matches work/projects.
        for tag in query.tags where !hasTag(snippet, tag) { return false }
        for tag in query.notTags where hasTag(snippet, tag) { return false }

        // Contexts are flat, so plain equality.
        if !query.contexts.isEmpty || !query.notContexts.isEmpty {
            let context = TitleContext.snippetContext(snippet)
            for required in query.contexts where context != required { return false }
            for excluded in query.notContexts where context == excluded { return false }
        }

        for filter in query.isFilters where !has(snippet, filter) { return false }
        for filter in query.notFilters where has(snippet, filter) { return false }

        let title = snippet.title.lowercased()
        let content = snippet.content.lowercased()
        for phrase in query.exactPhrases {
            let lowered = phrase.lowercased()
            if !title.contains(lowered) && !content.contains(lowered) { return false }
        }

        return matchesFuzzy(snippet, text: query.fuzzyText)
    }

    /// Whether every whitespace-separated word appears (case-insensitively)
    /// in the title, content or a tag. Blank text matches everything.
    public static func matchesFuzzy(_ snippet: Snippet, text: String) -> Bool {
        let words = text.lowercased().split(whereSeparator: \.isWhitespace)
        guard !words.isEmpty else { return true }
        let title = snippet.title.lowercased()
        let content = snippet.content.lowercased()
        let tags = snippet.tags.map { $0.lowercased() }
        return words.allSatisfy { word in
            title.contains(word) || content.contains(word) || tags.contains { $0.contains(word) }
        }
    }

    static func hasTag(_ snippet: Snippet, _ tag: String) -> Bool {
        snippet.tags.contains { $0 == tag || TagHierarchy.isChildOf($0, tag) }
    }

    static func has(_ snippet: Snippet, _ filter: BooleanFilter) -> Bool {
        switch filter {
        case .favorite, .bookmarked: snippet.isFavorite
        case .archived: snippet.isArchived
        case .untagged: snippet.tags.isEmpty
        }
    }
}
