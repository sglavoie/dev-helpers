import Testing
@testable import SloppyCore

/// The root list sections, labels and empty states from the extension's index.tsx.
@Suite struct PickerListModelTests {
    static let now: Int64 = 1_700_000_000_000

    static func snippet(
        _ id: String, title: String = "Snippet", content: String = "Content", tags: [String] = [],
        lastUsedAt: Int64? = nil, isFavorite: Bool = false, isArchived: Bool = false, isPinned: Bool = false
    ) -> Snippet {
        Snippet(
            id: id, title: title, content: content, tags: tags, createdAt: now, updatedAt: now,
            lastUsedAt: lastUsedAt, isFavorite: isFavorite, isArchived: isArchived, isPinned: isPinned)
    }

    static let library = [
        snippet("p", title: "Pinned one", isPinned: true),
        snippet("r", title: "Recent one", lastUsedAt: now - 1000),
        snippet("a", title: "asl: run the plan", tags: ["work"]),
        snippet("b", title: "Other", tags: ["home"]),
    ]

    @Test func sectionsInOrderWithTitles() {
        let state = PickerListModel.build(Self.library, query: "", now: Self.now)
        #expect(state.sections.map(\.kind) == [.pinned, .recent, .main])
        #expect(state.sections.map(\.title) == ["Pinned", "Recently Used", "All Snippets"])
        #expect(state.sections[0].subtitle == "1 snippet")
        #expect(state.rows.map(\.id).prefix(2) == ["p", "r"])
        #expect(Set(state.rows.map(\.id)) == ["p", "r", "a", "b"])
        #expect(state.emptyState == nil)
    }

    @Test func mainSectionHasNoHeaderWithoutPinnedOrRecent() {
        let snippets = [Self.snippet("a"), Self.snippet("b")]
        let state = PickerListModel.build(snippets, query: "", now: Self.now)
        #expect(state.sections.count == 1)
        #expect(state.sections[0].title == nil)
    }

    @Test func recentSectionNeedsThreeUnarchivedSnippets() {
        let snippets = [Self.snippet("r", lastUsedAt: Self.now), Self.snippet("b"), Self.snippet("c", isArchived: true)]
        let state = PickerListModel.build(snippets, query: "", now: Self.now)
        #expect(state.sections.map(\.kind) == [.main])
        // The recent snippet stays in the main section instead of vanishing.
        #expect(Set(state.rows.map(\.id)) == ["r", "b"])
    }

    @Test func recentSectionHiddenWhenTurnedOffOrArchivedView() {
        let off = PickerListModel.build(Self.library, query: "", showRecentSection: false, now: Self.now)
        #expect(!off.sections.map(\.kind).contains(.recent))
        #expect(off.rows.count == 4)

        let archived = PickerListModel.build(
            Self.library + [Self.snippet("x", lastUsedAt: Self.now, isArchived: true)], query: "",
            options: SnippetFilterOptions(showArchivedSnippets: true), now: Self.now)
        #expect(archived.sections.map(\.kind) == [.main])
        #expect(archived.sections[0].title == "⊟ Archived (1)")
    }

    @Test func typingFilters() {
        let state = PickerListModel.build(Self.library, query: "plan", now: Self.now)
        #expect(state.rows.map(\.id) == ["a"])
        #expect(state.sections.map(\.title) == [nil])
    }

    @Test func structuredOperatorsMarkTheTitleAndPlaceholder() {
        let state = PickerListModel.build(Self.library, query: "tag:work", now: Self.now)
        #expect(state.rows.map(\.id) == ["a"])
        #expect(state.sections[0].title == "All Snippets — operators active")
        #expect(state.searchPlaceholder == "Operators active")
    }

    @Test func viewFilterLabels() {
        let favorites = SnippetFilterOptions(showOnlyFavorites: true)
        #expect(PickerListModel.searchPlaceholder(options: favorites, hasStructuredOperators: true)
            == "★ Bookmarked — ⇧⌘F to show all")
        #expect(PickerListModel.mainSectionTitle(
            options: favorites, filteredCount: 2, hasPinnedOrRecent: false, hasStructuredOperators: true)
            == "★ Bookmarked (2)")
        #expect(PickerListModel.mainSectionTitle(
            options: SnippetFilterOptions(showNeedsAttention: true), filteredCount: 0, hasPinnedOrRecent: true,
            hasStructuredOperators: false) == "⚠ Needs Attention (0)")
    }

    @Test func emptyStates() {
        #expect(PickerListModel.build([], query: "", now: Self.now).emptyState?.title == "No snippets yet")
        #expect(PickerListModel.build(Self.library, query: "zzz", now: Self.now).emptyState?.title
            == "No matching snippets")
        #expect(PickerListModel.build(
            Self.library, query: "", options: SnippetFilterOptions(showOnlyFavorites: true), now: Self.now
        ).emptyState?.title == "No bookmarks yet")
    }

    @Test func suggestionsUseTagsAndContexts() {
        let tags = PickerListModel.build(Self.library, query: "tag:", now: Self.now)
        #expect(tags.suggestions.map(\.title) == ["tag:home", "tag:work"])
        let contexts = PickerListModel.build(Self.library, query: "ctx:", now: Self.now)
        #expect(contexts.suggestions.map(\.completion) == ["ctx:asl "])
    }

    @Test(arguments: [
        ("plain", 0),
        ("Hi {{name}} on {{DATE}}", 1),
        ("{{a}} {{b|default}} {{c}}", 2),
        ("{{#if flag}}x{{/if}}", 0),
    ])
    func requiredInputCount(content: String, expected: Int) {
        #expect(PickerListModel.requiredInputCount(Self.snippet("x", content: content)) == expected)
    }
}
