import Testing
@testable import SloppyCore

/// Ported from queryParser.test.ts.
@Suite struct QueryParserTests {
    @Test(arguments: ["", "   "])
    func emptyQueries(_ query: String) {
        let result = QueryParser.parse(query)
        #expect(result == ParsedQuery())
        #expect(!result.hasOperators)
        #expect(!result.hasStructuredOperators)
        #expect(result.isEmpty)
    }

    struct TagCase: Sendable, CustomTestStringConvertible {
        var query: String
        var tags: [String] = []
        var notTags: [String] = []
        var contexts: [String] = []
        var notContexts: [String] = []
        var fuzzy = ""
        var testDescription: String { query }
    }

    @Test(arguments: [
        TagCase(query: "tag:work", tags: ["work"]),
        TagCase(query: "tag:work/projects", tags: ["work/projects"]),
        TagCase(query: "tag:work tag:client", tags: ["work", "client"]),
        TagCase(query: "tag:Work", tags: ["work"]),
        TagCase(query: "tag:WORK/PROJECTS", tags: ["work/projects"]),
        TagCase(query: "tag:work api", tags: ["work"], fuzzy: "api"),
        TagCase(query: "not:tag:personal", notTags: ["personal"]),
        TagCase(query: "not:tag:work/client", notTags: ["work/client"]),
        TagCase(query: "not:tag:work not:tag:personal", notTags: ["work", "personal"]),
        TagCase(query: "not:tag:PERSONAL", notTags: ["personal"]),
        TagCase(query: "tag:work not:tag:client", tags: ["work"], notTags: ["client"]),
        TagCase(query: "ctx:asl", contexts: ["asl"]),
        TagCase(query: "ctx:Refactor", contexts: ["refactor"]),
        TagCase(query: "ctx:asl ctx:refactor", contexts: ["asl", "refactor"]),
        TagCase(query: "ctx:asl run", contexts: ["asl"], fuzzy: "run"),
        TagCase(query: "ctx:asl tag:work", tags: ["work"], contexts: ["asl"]),
        TagCase(query: "not:ctx:asl", notContexts: ["asl"]),
        TagCase(query: "not:ctx:REFACTOR", notContexts: ["refactor"]),
        TagCase(query: "not:ctx:asl not:ctx:refactor", notContexts: ["asl", "refactor"]),
        TagCase(query: "ctx:asl not:ctx:refactor", contexts: ["asl"], notContexts: ["refactor"]),
    ])
    func tagAndContextOperators(_ c: TagCase) {
        let result = QueryParser.parse(c.query)
        #expect(result.tags == c.tags)
        #expect(result.notTags == c.notTags)
        #expect(result.contexts == c.contexts)
        #expect(result.notContexts == c.notContexts)
        #expect(result.fuzzyText == c.fuzzy)
        #expect(result.hasOperators)
        #expect(result.hasStructuredOperators)
    }

    // Regression: not:ctx: must be matched before the generic not: branch,
    // otherwise it fails the boolean check and degrades to fuzzy text.
    @Test func notContextDoesNotDegradeToFuzzyText() {
        let result = QueryParser.parse("not:ctx:asl")
        #expect(result.fuzzyText == "")
        #expect(result.notFilters.isEmpty)
        #expect(result.contexts.isEmpty)
    }

    @Test func distinguishesNotContextFromNotBooleanAndNotTag() {
        let result = QueryParser.parse("not:archived not:tag:work not:ctx:asl")
        #expect(result.notFilters == [.archived])
        #expect(result.notTags == ["work"])
        #expect(result.notContexts == ["asl"])
        #expect(result.fuzzyText == "")
    }

    @Test func distinguishesNotTagFromNotBoolean() {
        let result = QueryParser.parse("not:archived not:tag:work")
        #expect(result.notFilters == [.archived])
        #expect(result.notTags == ["work"])
    }

    @Test(arguments: [
        ("is:favorite", [BooleanFilter.favorite]),
        ("is:archived", [.archived]),
        ("is:untagged", [.untagged]),
        ("is:bookmarked", [.bookmarked]),
        ("is:favorite is:archived", [.favorite, .archived]),
    ])
    func isOperators(_ query: String, _ expected: [BooleanFilter]) {
        let result = QueryParser.parse(query)
        #expect(result.isFilters == expected)
        #expect(result.hasOperators)
    }

    @Test(arguments: [
        ("not:favorite", [BooleanFilter.favorite]),
        ("not:archived", [.archived]),
        ("not:untagged", [.untagged]),
        ("not:favorite not:archived", [.favorite, .archived]),
    ])
    func notOperators(_ query: String, _ expected: [BooleanFilter]) {
        let result = QueryParser.parse(query)
        #expect(result.notFilters == expected)
        #expect(result.hasOperators)
    }

    /// Invalid or empty operators, unclosed quotes and unknown prefixes stay fuzzy text.
    @Test(arguments: [
        "is:invalid", "not:invalid", "\"unclosed", "unknown:operator search",
        "tag:", "is:", "not:", "not:tag:", "ctx:", "not:ctx:",
    ])
    func malformedOperatorsBecomeFuzzyText(_ query: String) {
        let result = QueryParser.parse(query)
        #expect(result.fuzzyText == query)
        #expect(!result.hasStructuredOperators)
        #expect(result.hasOperators) // fuzzy text counts as an operator
    }

    @Test(arguments: [
        ("\"meeting notes\"", ["meeting notes"], ""),
        ("\"phrase one\" \"phrase two\"", ["phrase one", "phrase two"], ""),
        ("\"api docs\" rest", ["api docs"], "rest"),
        ("\"\"", [], "\"\""),
        ("\"  meeting notes  \"", ["meeting notes"], ""),
    ])
    func exactPhrases(_ query: String, _ phrases: [String], _ fuzzy: String) {
        let result = QueryParser.parse(query)
        #expect(result.exactPhrases == phrases)
        #expect(result.fuzzyText == fuzzy)
    }

    @Test(arguments: [
        ("simple search", "simple search"),
        ("multiple words search query", "multiple words search query"),
        ("tag:work api rest", "api rest"),
        ("tag:work invalid:operator is:favorite", "invalid:operator"),
    ])
    func fuzzyText(_ query: String, _ expected: String) {
        let result = QueryParser.parse(query)
        #expect(result.fuzzyText == expected)
        #expect(result.hasOperators)
    }

    @Test func plainFuzzyTextIsNotStructured() {
        #expect(!QueryParser.parse("simple search").hasStructuredOperators)
    }

    @Test func tagIsAndFuzzy() {
        let result = QueryParser.parse("tag:work is:favorite api")
        #expect(result.tags == ["work"])
        #expect(result.isFilters == [.favorite])
        #expect(result.fuzzyText == "api")
    }

    @Test func tagNotExactAndFuzzy() {
        let result = QueryParser.parse("tag:work not:archived \"meeting\" api")
        #expect(result.tags == ["work"])
        #expect(result.notFilters == [.archived])
        #expect(result.exactPhrases == ["meeting"])
        #expect(result.fuzzyText == "api")
    }

    @Test func kitchenSink() {
        let result = QueryParser.parse(
            "tag:work tag:client not:tag:personal is:favorite not:archived \"api docs\" rest endpoint")
        #expect(result == ParsedQuery(
            tags: ["work", "client"], notTags: ["personal"], isFilters: [.favorite], notFilters: [.archived],
            exactPhrases: ["api docs"], fuzzyText: "rest endpoint"))
    }

    @Test(arguments: ["tag:work    is:favorite", "  tag:work  is:favorite ", "\ttag:work\nis:favorite"])
    func whitespaceBetweenOperators(_ query: String) {
        let result = QueryParser.parse(query)
        #expect(result.tags == ["work"])
        #expect(result.isFilters == [.favorite])
        #expect(result.fuzzyText == "")
    }

    @Test func emojiAndCJKStayFuzzy() {
        let result = QueryParser.parse("tag:日本 🚀 \"会議 メモ\"")
        #expect(result.tags == ["日本"])
        #expect(result.exactPhrases == ["会議 メモ"])
        #expect(result.fuzzyText == "🚀")
    }

    @Test(arguments: [("", true), ("tag:work", false), ("search text", false), ("tag:work api", false)])
    func isEmpty(_ query: String, _ expected: Bool) {
        #expect(QueryParser.parse(query).isEmpty == expected)
    }
}
