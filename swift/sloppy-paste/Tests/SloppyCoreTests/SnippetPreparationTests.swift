import Foundation
import Testing
@testable import SloppyCore

@Suite struct SnippetPreparationTests {
    /// 2024-03-15T14:30:45.000Z.
    static let now: Int64 = 1_710_513_045_000
    static let utc = TimeZone(identifier: "UTC")!

    private func snippet(_ content: String, id: String = "s1") -> Snippet {
        Snippet(id: id, title: "T", content: content, createdAt: 0, updatedAt: 0)
    }

    private func history(_ pairs: [String: String]) -> PlaceholderHistory {
        pairs.mapValues { [PlaceholderHistoryValue(value: $0, useCount: 1, lastUsed: 5, createdAt: 1)] }
    }

    @Test func directPlanResolvesSystemPlaceholdersAndConditionals() {
        let plan = SnippetPreparation.plan(for: snippet("On {{DATE}}\n"), now: Self.now,
            timeZone: Self.utc)
        #expect(plan == .direct(PreparedSnippet(content: "On 2024-03-15\n")))
    }

    @Test func formPlanCarriesProcessedContentAndUserPlaceholders() {
        let plan = SnippetPreparation.plan(for: snippet("{{YEAR}} {{name}}"), now: Self.now, timeZone: Self.utc)
        guard case .form(let request) = plan else {
            Issue.record("expected a form plan")
            return
        }
        #expect(request.snippet.content == "2024 {{name}}")
        #expect(request.snippet.id == "s1")
        #expect(request.placeholders.map(\.key) == ["name"])
    }

    @Test func lastValuesAvailability() {
        let s = snippet("{{a}} {{b}} {{opt|x}} {{DATE}}")
        #expect(SnippetPreparation.lastValuesAvailability(for: snippet("no {{opt|x}}"), history: [:], now: Self.now)
            == .notApplicable)
        #expect(SnippetPreparation.lastValuesAvailability(for: s, history: history(["a": "1"]), now: Self.now)
            == .unavailable)
        #expect(SnippetPreparation.lastValuesAvailability(for: s, history: history(["a": "1", "b": "2"]),
            now: Self.now) == .available)
    }

    @Test func historyAvailabilityListsSnippetsWithAllRequiredHistory() {
        let snippets = [
            snippet("{{a}}", id: "has"),
            snippet("{{a}} {{b}}", id: "missing"),
            snippet("plain", id: "plain"),
            snippet("{{opt|x}}", id: "optional-only"),
        ]
        #expect(SnippetPreparation.historyAvailability(for: snippets, history: history(["a": "1"]), now: Self.now)
            == ["has"])
    }

    @Test func prepareWithLastValuesFillsRequiredAndDefaults() throws {
        let values: PlaceholderHistory = ["name": [
            PlaceholderHistoryValue(value: "Old", useCount: 9, lastUsed: 1, createdAt: 1),
            PlaceholderHistoryValue(value: "Ada", useCount: 1, lastUsed: 9, createdAt: 1),
        ]]
        let prepared = try SnippetPreparation.prepareWithLastValues(
            snippet("{{DATE}} {{name}}{{(:role:)|dev}}{{#if name}}!{{/if}}{{opt|}}"),
            history: values, now: Self.now, timeZone: Self.utc)
        #expect(prepared.content == "2024-03-15 Ada(dev)!")
        #expect(prepared.placeholderValues == [
            .init(key: "name", value: "Ada"), .init(key: "role", value: "dev"), .init(key: "opt", value: ""),
        ])
    }

    @Test func prepareWithLastValuesThrowsForMissingHistory() {
        #expect(throws: MissingPlaceholderHistoryError(placeholderKey: "b")) {
            try SnippetPreparation.prepareWithLastValues(snippet("{{a}} {{b}}"), history: history(["a": "1"]),
                now: Self.now)
        }
        #expect(MissingPlaceholderHistoryError(placeholderKey: "b").description == "No history for {{b}}")
    }
}
