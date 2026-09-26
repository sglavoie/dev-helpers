import Testing
@testable import SloppyCore

/// Ported from searchFilter.test.ts.
@Suite struct SearchFilterTests {
    static func snippet(
        _ id: String = "test-id", title: String = "Test Snippet", content: String = "Test content",
        tags: [String] = [], isFavorite: Bool = false, isArchived: Bool = false
    ) -> Snippet {
        Snippet(
            id: id, title: title, content: content, tags: tags, createdAt: 0, updatedAt: 0,
            isFavorite: isFavorite, isArchived: isArchived)
    }

    static func ids(_ snippets: [Snippet], _ query: ParsedQuery) -> [String] {
        SearchFilter.apply(snippets, query: query).map(\.id)
    }

    // MARK: Tags

    @Test func requiredTag() {
        let snippets = [Self.snippet("1", tags: ["work"]), Self.snippet("2", tags: ["personal"])]
        #expect(Self.ids(snippets, ParsedQuery(tags: ["work"])) == ["1"])
    }

    @Test func hierarchicalTag() {
        let snippets = [Self.snippet("1", tags: ["work/projects"]), Self.snippet("2", tags: ["personal"])]
        #expect(Self.ids(snippets, ParsedQuery(tags: ["work"])) == ["1"])
    }

    @Test func requiresAllTags() {
        let snippets = [
            Self.snippet("1", tags: ["work", "client"]), Self.snippet("2", tags: ["work"]),
            Self.snippet("3", tags: ["client"]),
        ]
        #expect(Self.ids(snippets, ParsedQuery(tags: ["work", "client"])) == ["1"])
    }

    @Test func missingTag() {
        #expect(Self.ids([Self.snippet(tags: ["work"])], ParsedQuery(tags: ["nonexistent"])).isEmpty)
    }

    @Test func excludedTag() {
        let snippets = [Self.snippet("1", tags: ["work"]), Self.snippet("2", tags: ["personal"])]
        #expect(Self.ids(snippets, ParsedQuery(notTags: ["personal"])) == ["1"])
    }

    @Test func excludedHierarchicalTag() {
        let snippets = [Self.snippet("1", tags: ["work/client"]), Self.snippet("2", tags: ["personal"])]
        #expect(Self.ids(snippets, ParsedQuery(notTags: ["work"])) == ["2"])
    }

    @Test func multipleExclusions() {
        let snippets = [
            Self.snippet("1", tags: ["work"]), Self.snippet("2", tags: ["personal"]), Self.snippet("3", tags: ["hobby"]),
        ]
        #expect(Self.ids(snippets, ParsedQuery(notTags: ["work", "personal"])) == ["3"])
    }

    @Test func positiveAndNegativeTags() {
        let snippets = [
            Self.snippet("1", tags: ["work", "client"]), Self.snippet("2", tags: ["work", "internal"]),
            Self.snippet("3", tags: ["personal"]),
        ]
        #expect(Self.ids(snippets, ParsedQuery(tags: ["work"], notTags: ["client"])) == ["2"])
    }

    // MARK: Contexts

    static let contextSnippets = [
        snippet("1", title: "asl: run"),
        snippet("2", title: "Refactor: debt triage"),
        snippet("3", title: "refactor: dead code"),
        snippet("4", title: "asl commit"),
    ]

    @Test(arguments: [
        (["asl"], ["1"]),
        (["refactor"], ["2", "3"]), // case-insensitive
        (["asl", "refactor"], []),
    ])
    func requiredContexts(_ contexts: [String], _ expected: [String]) {
        #expect(Self.ids(Self.contextSnippets, ParsedQuery(contexts: contexts)) == expected)
    }

    // Unlike tag:, contexts are flat: no prefix or hierarchy matching.
    @Test func contextNeedsExactEquality() {
        #expect(Self.ids([Self.snippet("1", title: "asl-run: go")], ParsedQuery(contexts: ["asl"])).isEmpty)
    }

    static let negativeContextSnippets = [
        snippet("1", title: "asl: run"),
        snippet("2", title: "Refactor: debt triage"),
        snippet("3", title: "asl commit"),
    ]

    @Test func excludedContextKeepsSnippetsWithoutContext() {
        #expect(Self.ids(Self.negativeContextSnippets, ParsedQuery(notContexts: ["asl"])) == ["2", "3"])
    }

    @Test func positiveAndNegativeContexts() {
        let query = ParsedQuery(contexts: ["asl"], notContexts: ["refactor"])
        #expect(Self.ids(Self.negativeContextSnippets, query) == ["1"])
    }

    @Test func contextAndTag() {
        let snippets = [
            Self.snippet("1", title: "asl: run", tags: ["work"]),
            Self.snippet("2", title: "asl: commit", tags: ["personal"]),
        ]
        #expect(Self.ids(snippets, ParsedQuery(tags: ["work"], contexts: ["asl"])) == ["1"])
    }

    /// End to end through the parser: `not:ctx:` must exclude rather than fuzzy-match.
    @Test func parsedNotContextExcludes() {
        #expect(Self.ids(Self.negativeContextSnippets, QueryParser.parse("not:ctx:ASL")) == ["2", "3"])
    }

    // MARK: Booleans

    static let flagSnippets = [
        snippet("fav", isFavorite: true),
        snippet("arch", isArchived: true),
        snippet("tagged", tags: ["work"]),
    ]

    @Test(arguments: [
        (BooleanFilter.favorite, ["fav"]),
        (.bookmarked, ["fav"]),
        (.archived, ["arch"]),
        (.untagged, ["fav", "arch"]),
    ])
    func isFilter(_ filter: BooleanFilter, _ expected: [String]) {
        #expect(Self.ids(Self.flagSnippets, ParsedQuery(isFilters: [filter])) == expected)
    }

    @Test(arguments: [
        (BooleanFilter.favorite, ["arch", "tagged"]),
        (.bookmarked, ["arch", "tagged"]),
        (.archived, ["fav", "tagged"]),
        (.untagged, ["tagged"]),
    ])
    func notFilter(_ filter: BooleanFilter, _ expected: [String]) {
        #expect(Self.ids(Self.flagSnippets, ParsedQuery(notFilters: [filter])) == expected)
    }

    @Test func multipleIsFilters() {
        let snippets = [
            Self.snippet("1", isFavorite: true, isArchived: true),
            Self.snippet("2", isFavorite: true, isArchived: false),
            Self.snippet("3", isFavorite: false, isArchived: true),
        ]
        #expect(Self.ids(snippets, ParsedQuery(isFilters: [.favorite, .archived])) == ["1"])
    }

    @Test func contradictoryFilters() {
        let snippets = [Self.snippet("1", isArchived: true), Self.snippet("2", isArchived: false)]
        #expect(Self.ids(snippets, ParsedQuery(isFilters: [.archived], notFilters: [.archived])).isEmpty)
    }

    // MARK: Exact phrases

    @Test func phraseInTitle() {
        let snippets = [
            Self.snippet("1", title: "Meeting notes from yesterday"), Self.snippet("2", title: "Random notes"),
        ]
        #expect(Self.ids(snippets, ParsedQuery(exactPhrases: ["meeting notes"])) == ["1"])
    }

    @Test func phraseInContent() {
        let snippets = [
            Self.snippet("1", content: "This has api documentation"),
            Self.snippet("2", content: "This has something else"),
        ]
        #expect(Self.ids(snippets, ParsedQuery(exactPhrases: ["api documentation"])) == ["1"])
    }

    @Test func phraseIsCaseInsensitive() {
        let snippets = [Self.snippet("1", title: "API Documentation"), Self.snippet("2", title: "Something else")]
        #expect(Self.ids(snippets, ParsedQuery(exactPhrases: ["api documentation"])) == ["1"])
    }

    @Test func requiresAllPhrases() {
        let snippets = [
            Self.snippet("1", content: "meeting notes and api docs"),
            Self.snippet("2", content: "meeting notes only"),
            Self.snippet("3", content: "api docs only"),
        ]
        #expect(Self.ids(snippets, ParsedQuery(exactPhrases: ["meeting notes", "api docs"])) == ["1"])
    }

    @Test func missingPhrase() {
        let snippets = [Self.snippet(title: "Test", content: "Content")]
        #expect(Self.ids(snippets, ParsedQuery(exactPhrases: ["nonexistent phrase"])).isEmpty)
    }

    // MARK: Fuzzy text

    @Test func fuzzyWordInTitle() {
        let snippets = [Self.snippet("1", title: "API Documentation"), Self.snippet("2", title: "Meeting Notes")]
        #expect(Self.ids(snippets, ParsedQuery(fuzzyText: "api")) == ["1"])
    }

    @Test func fuzzyWordInContent() {
        let snippets = [
            Self.snippet("1", content: "Contains python code"), Self.snippet("2", content: "Contains javascript code"),
        ]
        #expect(Self.ids(snippets, ParsedQuery(fuzzyText: "python")) == ["1"])
    }

    @Test func fuzzyWordInTags() {
        let snippets = [Self.snippet("1", tags: ["work/client"]), Self.snippet("2", tags: ["personal"])]
        #expect(Self.ids(snippets, ParsedQuery(fuzzyText: "client")) == ["1"])
    }

    @Test func fuzzyRequiresAllWords() {
        let snippets = [
            Self.snippet("1", title: "API", content: "rest endpoint"),
            Self.snippet("2", title: "API", content: "graphql"),
            Self.snippet("3", title: "Rest", content: "endpoint"),
        ]
        #expect(Self.ids(snippets, ParsedQuery(fuzzyText: "api rest")) == ["1"])
    }

    @Test func fuzzyIsCaseInsensitive() {
        #expect(Self.ids([Self.snippet("1", title: "API Documentation")], ParsedQuery(fuzzyText: "API")) == ["1"])
    }

    @Test func fuzzyWordsAcrossFields() {
        let snippets = [
            Self.snippet("1", title: "Meeting", tags: ["work"]), Self.snippet("2", title: "Notes", tags: ["personal"]),
        ]
        #expect(Self.ids(snippets, ParsedQuery(fuzzyText: "meeting work")) == ["1"])
    }

    @Test func fuzzyMatchesMixedCaseTag() {
        let snippets = [Self.snippet("1", title: "x", content: "y", tags: ["Work"])]
        #expect(Self.ids(snippets, ParsedQuery(fuzzyText: "work")) == ["1"])
    }

    @Test func fuzzyEmojiAndCJK() {
        let snippets = [
            Self.snippet("1", title: "Déploiement 🚀", content: "日本語のメモ"), Self.snippet("2", title: "Other"),
        ]
        #expect(Self.ids(snippets, ParsedQuery(fuzzyText: "🚀 日本語")) == ["1"])
        #expect(Self.ids(snippets, ParsedQuery(exactPhrases: ["déploiement"])) == ["1"])
    }

    // MARK: Combined

    @Test func tagBooleanAndFuzzy() {
        let snippets = [
            Self.snippet("1", content: "api docs", tags: ["work"], isFavorite: true),
            Self.snippet("2", content: "api docs", tags: ["work"], isFavorite: false),
            Self.snippet("3", content: "other", tags: ["work"], isFavorite: true),
            Self.snippet("4", content: "api docs", tags: ["personal"], isFavorite: true),
        ]
        #expect(Self.ids(snippets, ParsedQuery(tags: ["work"], isFilters: [.favorite], fuzzyText: "api")) == ["1"])
    }

    @Test func allOperatorTypes() {
        let snippets = [
            Self.snippet("1", title: "API meeting notes", content: "rest endpoint", tags: ["work", "client"], isFavorite: true),
            Self.snippet("2", title: "API meeting notes", content: "rest endpoint", tags: ["work", "internal"], isFavorite: true),
        ]
        let query = ParsedQuery(
            tags: ["work"], notTags: ["internal"], isFilters: [.favorite], notFilters: [.archived],
            exactPhrases: ["meeting notes"], fuzzyText: "api rest")
        #expect(Self.ids(snippets, query) == ["1"])
    }

    @Test func emptyQueryKeepsEverything() {
        let snippets = [Self.snippet("1"), Self.snippet("2"), Self.snippet("3")]
        #expect(Self.ids(snippets, ParsedQuery()) == ["1", "2", "3"])
    }

    @Test func emptyInput() {
        #expect(Self.ids([], ParsedQuery(tags: ["work"])).isEmpty)
    }

    @Test func untaggedSnippetFailsTagFilter() {
        #expect(Self.ids([Self.snippet("1")], ParsedQuery(tags: ["work"])).isEmpty)
    }

    // MARK: matchesFuzzy

    @Test(arguments: [
        ("API Documentation", "", [String](), "api", true),
        ("API", "rest endpoint", [], "api rest", true),
        ("Test", "", [], "", true),
        ("Test", "", [], "   ", true),
        ("API Documentation", "", [], "nonexistent", false),
        ("api documentation", "", [], "API", true),
        ("x", "y", ["Work"], "work", true),
    ])
    func matchesFuzzy(_ title: String, _ content: String, _ tags: [String], _ text: String, _ expected: Bool) {
        let snippet = Self.snippet(title: title, content: content, tags: tags)
        #expect(SearchFilter.matchesFuzzy(snippet, text: text) == expected)
    }
}
