import Foundation
import Testing
@testable import SloppyCore

@Suite struct PasteOutcomeTests {
    static let decisions: [(trusted: Bool, secureInput: Bool, expected: PasteOutcome)] = [
        (true, false, .pasted),
        (true, true, .copiedOnly(.secureInput)),
        (false, false, .copiedOnly(.accessibilityMissing)),
        // Missing Accessibility wins: granting it is the fix the HUD should point to.
        (false, true, .copiedOnly(.accessibilityMissing)),
    ]

    @Test(arguments: decisions)
    func decidesWhetherToPostCommandV(_ testCase: (trusted: Bool, secureInput: Bool, expected: PasteOutcome)) {
        let outcome = PasteOutcome.decide(accessibilityTrusted: testCase.trusted, secureInputEnabled: testCase.secureInput)
        #expect(outcome == testCase.expected)
    }

    @Test func pastedShowsNoHUD() {
        #expect(PasteOutcome.pasted.hud == nil)
    }

    @Test func copyOnlyHUDsExplainTheFallback() throws {
        let accessibility = try #require(PasteOutcome.copiedOnly(.accessibilityMissing).hud)
        #expect(accessibility.title == "Copied to Clipboard")
        #expect(accessibility.message?.contains("Accessibility") == true)

        let secure = try #require(PasteOutcome.copiedOnly(.secureInput).hud)
        #expect(secure.title == "Copied to Clipboard")
        #expect(secure.message?.contains("Secure input") == true)

        let copied = try #require(PasteOutcome.copied.hud)
        #expect(copied.title == "Copied to Clipboard")
        #expect(copied.message == nil)
    }

    /// The picker's ↵ path for a snippet without placeholders: plan, deliver,
    /// then record the use through the repository.
    @Test func directPasteRecordsOneUsePerDelivery() async throws {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "Greeting", content: "Hello {{DATE}}", now: 1)
        guard case .direct(let prepared) = SnippetPreparation.plan(for: snippet, now: 0, timeZone: .gmt) else {
            Issue.record("expected a direct plan")
            return
        }
        #expect(prepared.content == "Hello 1970-01-01")

        let box = RepositoryBox(repo)
        let use = SnippetUse(record: { id, values in try box.recordUse(id: id, values: values, now: 50) }, log: { _ in })
        for _ in 0..<2 {
            let succeeded = await use.run(
                snippetID: snippet.id, placeholderValues: prepared.placeholderValues,
                prepare: { prepared }, primaryOperation: { _ in },
                onPreparationFailure: { _ in }, onPrimaryFailure: { _ in })
            #expect(succeeded)
        }

        let stored = try #require(box.snippet(id: snippet.id))
        #expect(stored.useCount == 2)
        #expect(stored.lastUsedAt == 50)
    }
}

/// Sendable wrapper so the test's record closure can mutate a repository.
private final class RepositoryBox: @unchecked Sendable {
    private let lock = NSLock()
    private var repo: SnippetRepository

    init(_ repo: SnippetRepository) { self.repo = repo }

    func recordUse(id: String, values: [PlaceholderValueToRecord], now: Int64) throws {
        try lock.withLock { try repo.recordUse(id: id, placeholderValues: values, now: now) }
    }

    func snippet(id: String) -> Snippet? {
        lock.withLock { repo.snippet(id: id) }
    }
}
