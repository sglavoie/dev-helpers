import Testing
@testable import SloppyCore

@Suite struct TitleContextTests {
    @Test(arguments: [
        ("asl: run", "asl" as String?, "run"),
        ("Refactor: debt triage", "Refactor", "debt triage"),
        ("asl: run: with args", "asl", "run: with args"),
        ("asl:   run", "asl", "run"),
        ("Dead Code: exports", "Dead Code", "exports"),
        ("dead-code: exports", "dead-code", "exports"),
        ("dead_code: exports", "dead_code", "exports"),
        ("Go To Def: usage", "Go To Def", "usage"),
        ("日本: テスト", nil, "日本: テスト"),
        ("emoji: 🎉 party", "emoji", "🎉 party"),
    ])
    func parses(title: String, context: String?, displayTitle: String) {
        #expect(TitleContext.parse(title) == TitleContext(context: context, displayTitle: displayTitle))
    }

    @Test(arguments: [
        "asl commit",
        "https://example.com/docs",
        "Note:something",
        "asl:",
        "asl: ",
        String(repeating: "a", count: 25) + ": value",
        "Remember to do this: now",
        "-asl: run",
        "",
        "asl: run\nsecond line",
    ])
    func nonMatches(title: String) {
        #expect(TitleContext.parse(title) == TitleContext(context: nil, displayTitle: title))
    }

    @Test func acceptsTwentyFourCharacterPrefix() {
        let prefix = String(repeating: "a", count: 24)
        #expect(TitleContext.parse("\(prefix): value").context == prefix)
    }

    @Test func normalize() {
        #expect(TitleContext.normalize("  Refactor  ") == "refactor")
        #expect(TitleContext.normalize("ASL") == "asl")
    }

    @Test(arguments: [
        ("asl", "ASL"), ("PLAN", "PLAN"), ("plan", "PLAN"), ("Plan", "PLAN"),
        ("build", "BUILD"), ("Go To", "GO TO"), ("  asl  ", "ASL"),
        ("workflow", "WORK"), ("Refactor", "REFA"), ("planning", "PLAN"),
        ("Dead Code", "DC"), ("go to def", "GTD"), ("dead-code", "DC"), ("dead_code", "DC"),
        ("R&D", "R&D"), ("   ", ""),
    ])
    func abbreviation(context: String, expected: String) {
        #expect(TitleContext.abbreviation(context) == expected)
    }

    @Test(arguments: ["a b c d e f", "workflow", "Dead Code", "build", "a-b-c-d-e-f-g"])
    func abbreviationFitsBadge(context: String) {
        let label = TitleContext.abbreviation(context)
        #expect(label.count <= 5)
        #expect(label == label.uppercased())
    }

    @Test func colorIsStableAndCaseInsensitive() {
        #expect(TitleContext.color("asl") == TitleContext.color("asl"))
        #expect(TitleContext.color("Refactor") == TitleContext.color("  refactor  "))
        #expect(TitleContext.color("asl") != TitleContext.color("Refactor"))
    }

    @Test func colorIsHex() {
        let color = TitleContext.color("asl")
        for hex in [color.light, color.dark] {
            #expect(hex.wholeMatch(of: /#[0-9A-Fa-f]{6}/) != nil)
        }
    }

    /// Palette indices computed with the TS `hashContext` in node; `refactor` overflows Int32.
    @Test(arguments: [("asl", 2), ("Refactor", 6), ("documentation", 2), ("very long context", 5)])
    func colorMatchesJavaScriptHash(context: String, paletteIndex: Int) {
        #expect(TitleContext.color(context) == TitleContext.palette[paletteIndex])
    }

    @Test(arguments: [("A", 16.0), ("AB", 14.0), ("ASL", 12.0), ("WORK", 10.0), ("BUILD", 9.0)])
    func badgeFontSize(abbreviation: String, expected: Double) {
        #expect(TitleContext.badgeFontSize(forAbbreviation: abbreviation) == expected)
    }

    static func snippet(_ title: String) -> Snippet {
        Snippet(id: "test-id", title: title, content: "Test content", createdAt: 0, updatedAt: 0)
    }

    @Test func allContexts() {
        let snippets = ["Refactor: debt triage", "asl: run", "refactor: dead code", "asl commit"].map(Self.snippet)
        #expect(TitleContext.allContexts(snippets) == ["asl", "refactor"])
        #expect(TitleContext.allContexts([Self.snippet("plain title")]).isEmpty)
    }

    @Test func snippetContext() {
        #expect(TitleContext.snippetContext(Self.snippet("Refactor: debt")) == "refactor")
        #expect(TitleContext.snippetContext(Self.snippet("plain")) == nil)
    }
}
