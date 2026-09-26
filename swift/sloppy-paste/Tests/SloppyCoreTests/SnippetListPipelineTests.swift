import Testing
@testable import SloppyCore

/// Ported from useSnippetFiltering.test.ts, plus the section split and sort orders.
@Suite struct SnippetListPipelineTests {
    static let now: Int64 = 1_700_000_000_000
    static let day: Int64 = 24 * 60 * 60 * 1000

    static func snippet(
        _ id: String, title: String = "Test Snippet", content: String = "Test content", tags: [String] = [],
        createdAt: Int64 = now, updatedAt: Int64 = now, lastUsedAt: Int64? = nil, useCount: Int = 0,
        isFavorite: Bool = false, isArchived: Bool = false, isPinned: Bool = false
    ) -> Snippet {
        Snippet(
            id: id, title: title, content: content, tags: tags, createdAt: createdAt, updatedAt: updatedAt,
            lastUsedAt: lastUsedAt, useCount: useCount, isFavorite: isFavorite, isArchived: isArchived,
            isPinned: isPinned)
    }

    static func ids(_ snippets: [Snippet], _ query: String, _ options: SnippetFilterOptions = SnippetFilterOptions()) -> [String] {
        SnippetListPipeline.filter(snippets, query: QueryParser.parse(query), options: options, now: now).map(\.id)
    }

    // MARK: Without search operators

    @Test func tagFilter() {
        let snippets = [
            Self.snippet("1", tags: ["work"]), Self.snippet("2", tags: ["personal"]),
            Self.snippet("3", tags: ["work/projects"]),
        ]
        #expect(Self.ids(snippets, "", SnippetFilterOptions(selectedTag: "work")) == ["1", "3"])
    }

    @Test func untaggedSentinel() {
        let snippets = [Self.snippet("1", tags: ["work"]), Self.snippet("2")]
        #expect(Self.ids(snippets, "", SnippetFilterOptions(selectedTag: TagHierarchy.untaggedSentinel)) == ["2"])
    }

    @Test func favoritesFilter() {
        let snippets = [Self.snippet("1", isFavorite: true), Self.snippet("2")]
        #expect(Self.ids(snippets, "", SnippetFilterOptions(showOnlyFavorites: true)) == ["1"])
    }

    @Test func defaultViewHidesArchived() {
        let snippets = [Self.snippet("1"), Self.snippet("2", isArchived: true), Self.snippet("3")]
        #expect(Self.ids(snippets, "") == ["1", "3"])
    }

    // MARK: Search operators with the dropdown and toggles (regression)

    @Test func tagDropdownAndFuzzyQuery() {
        let snippets = [
            Self.snippet("1", title: "API notes", tags: ["work"]),
            Self.snippet("2", title: "API recipes", tags: ["personal"]),
            Self.snippet("3", title: "Random", tags: ["work"]),
        ]
        #expect(QueryParser.parse("api").hasOperators)
        #expect(Self.ids(snippets, "api", SnippetFilterOptions(selectedTag: "work")) == ["1"])
    }

    @Test func tagDropdownAndTagOperator() {
        let snippets = [
            Self.snippet("1", tags: ["work", "client"]), Self.snippet("2", tags: ["work"]),
            Self.snippet("3", tags: ["personal", "client"]),
        ]
        #expect(Self.ids(snippets, "tag:client", SnippetFilterOptions(selectedTag: "work")) == ["1"])
    }

    @Test func favoritesToggleAndQuery() {
        let snippets = [
            Self.snippet("1", content: "api docs", isFavorite: true),
            Self.snippet("2", content: "api docs"),
            Self.snippet("3", content: "other content", isFavorite: true),
        ]
        #expect(Self.ids(snippets, "api", SnippetFilterOptions(showOnlyFavorites: true)) == ["1"])
    }

    // MARK: Archive view

    @Test func archiveViewShowsOnlyArchived() {
        let snippets = [
            Self.snippet("1", isArchived: true), Self.snippet("2"), Self.snippet("3", isArchived: true),
        ]
        #expect(Self.ids(snippets, "", SnippetFilterOptions(showArchivedSnippets: true)) == ["1", "3"])
    }

    @Test func defaultViewExcludesArchived() {
        #expect(Self.ids([Self.snippet("1", isArchived: true), Self.snippet("2")], "") == ["2"])
    }

    // MARK: Needs attention

    @Test func needsAttentionKeepsStale() {
        let longAgo = Self.now - 200 * Self.day
        let snippets = [
            Self.snippet("stale", createdAt: longAgo, updatedAt: longAgo, lastUsedAt: longAgo, useCount: 3),
            Self.snippet("fresh", lastUsedAt: Self.now, useCount: 5),
            Self.snippet("new"),
        ]
        #expect(Self.ids(snippets, "", SnippetFilterOptions(showNeedsAttention: true)) == ["stale"])
    }

    @Test func needsAttentionCombinesWithTag() {
        let longAgo = Self.now - 200 * Self.day
        let snippets = [
            Self.snippet("stale-work", tags: ["work"], createdAt: longAgo, updatedAt: longAgo, lastUsedAt: longAgo, useCount: 1),
            Self.snippet("stale-personal", tags: ["personal"], createdAt: longAgo, updatedAt: longAgo, lastUsedAt: longAgo, useCount: 1),
            Self.snippet("fresh-work", tags: ["work"], lastUsedAt: Self.now, useCount: 1),
        ]
        let options = SnippetFilterOptions(selectedTag: "work", showNeedsAttention: true)
        #expect(Self.ids(snippets, "", options) == ["stale-work"])
    }

    // MARK: Sections

    @Test func sectionsSplitPinnedRecentAndRest() {
        let snippets = [
            Self.snippet("b-pin", title: "beta", lastUsedAt: Self.now, isPinned: true),
            Self.snippet("a-pin", title: "Alpha", isPinned: true),
            Self.snippet("old", updatedAt: Self.now - 10 * Self.day),
            Self.snippet("used0", updatedAt: Self.now - 20 * Self.day, lastUsedAt: 0),
            Self.snippet("used1", lastUsedAt: Self.now - 1),
            Self.snippet("new", updatedAt: Self.now + 5),
        ]
        let list = SnippetListPipeline.build(snippets, query: "", now: Self.now)
        #expect(list.pinned.map(\.id) == ["a-pin", "b-pin"])
        // lastUsedAt 0 is an ancient use, not "never used".
        #expect(list.recent.map(\.id) == ["used1", "used0"])
        #expect(list.sorted.map(\.id) == ["new", "old"])
        #expect(list.rows.count == snippets.count)
    }

    @Test func recentSectionOff() {
        let snippets = [Self.snippet("1", lastUsedAt: Self.now), Self.snippet("2")]
        let list = SnippetListPipeline.build(snippets, query: "", showRecentSection: false, now: Self.now)
        #expect(list.recent.isEmpty)
        #expect(list.sorted.map(\.id) == ["1", "2"])
    }

    @Test func recentCapsAtFive() {
        let snippets = (0..<7).map { Self.snippet("s\($0)", lastUsedAt: Int64($0)) }
        #expect(SnippetListPipeline.recentSnippets(snippets, excluding: []).map(\.id) == ["s6", "s5", "s4", "s3", "s2"])
    }

    @Test(arguments: [
        (SortOption.updatedDesc, ["used", "c", "a", "b"]),
        (.mostUsedDesc, ["b", "used", "a", "c"]),
        (.alphabetical, ["a", "b", "c", "used"]),
        (.createdDesc, ["c", "b", "a", "used"]),
    ])
    func sortOrders(_ sort: SortOption, _ expected: [String]) {
        let snippets = [
            Self.snippet("a", title: "apple", createdAt: 2, updatedAt: 20, useCount: 1),
            Self.snippet("b", title: "Banana", createdAt: 3, updatedAt: 10, useCount: 9),
            Self.snippet("c", title: "cherry", createdAt: 4, updatedAt: 30, useCount: 0),
            Self.snippet("used", title: "date", createdAt: 1, updatedAt: 5, lastUsedAt: 40, useCount: 2),
        ]
        #expect(SnippetListPipeline.sorted(snippets, by: sort).map(\.id) == expected)
    }

    @Test func sortKeepsTiesInOrder() {
        let snippets = (0..<20).map { Self.snippet("s\($0)") }
        #expect(SnippetListPipeline.sorted(snippets, by: .mostUsedDesc).map(\.id) == snippets.map(\.id))
    }

    @Test func tagsAndStructuredFlag() {
        let snippets = [
            Self.snippet("1", title: "asl: run", tags: ["work/projects"]),
            Self.snippet("2", tags: ["personal"]),
        ]
        let list = SnippetListPipeline.build(snippets, query: "ctx:asl", now: Self.now)
        #expect(list.filtered.map(\.id) == ["1"])
        #expect(list.visibleTags == ["work", "work/projects"])
        #expect(list.allTags == ["personal", "work", "work/projects"])
        #expect(list.hasStructuredOperators)
        #expect(!SnippetListPipeline.build(snippets, query: "run", now: Self.now).hasStructuredOperators)
    }
}

/// Ports the useSearchSuggestions branches (no TS test file exists).
@Suite struct SearchSuggestionsTests {
    static let tags = ["work", "personal", "a", "b", "c"]
    static let contexts = ["asl", "refactor"]

    static func suggest(_ query: String, tags: [String] = tags) -> [SearchSuggestion] {
        SearchSuggestions.suggestions(for: query, allTags: tags, allContexts: contexts)
    }

    @Test func tagCompletions() {
        let items = Self.suggest("api tag:")
        #expect(items.map(\.title) == ["tag:work", "tag:personal", "tag:a", "tag:b"])
        #expect(items[0].completion == "api tag:work ")
        #expect(items[0].subtitle == "Filter to snippets tagged work")
    }

    @Test func contextCompletions() {
        let items = Self.suggest("ctx: ")
        #expect(items.map(\.title) == ["ctx:asl", "ctx:refactor"])
        #expect(items[0].completion == "ctx:asl ")
        #expect(items[0].subtitle == "Filter to snippets in the asl context")
    }

    @Test func negatedContextCompletions() {
        let items = Self.suggest("x not:ctx:")
        #expect(items.map(\.title) == ["not:ctx:asl", "not:ctx:refactor"])
        #expect(items[0].completion == "x not:ctx:asl ")
        #expect(items[0].subtitle == "Exclude snippets in the asl context")
    }

    @Test func isCompletions() {
        let items = Self.suggest("is:")
        #expect(items.map(\.title) == ["is:favorite", "is:bookmarked", "is:archived", "is:untagged"])
        #expect(Self.suggest("a is:")[0].completion == "a is:favorite ")
    }

    @Test func notCompletionsAppendFirstTag() {
        #expect(Self.suggest("not:").map(\.title) == ["not:archived", "not:favorite", "not:untagged", "not:tag:work"])
        #expect(Self.suggest("not:", tags: []).map(\.title) == ["not:archived", "not:favorite", "not:untagged"])
        #expect(Self.suggest("q not:")[3].completion == "q not:tag:work ")
    }

    @Test(arguments: ["", "api", "this:", "xis:", "anot:", "tag:work"])
    func noCompletions(_ query: String) {
        #expect(Self.suggest(query).isEmpty)
    }
}
