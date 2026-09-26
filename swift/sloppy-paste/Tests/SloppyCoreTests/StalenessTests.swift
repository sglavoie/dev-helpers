import Testing
@testable import SloppyCore

@Suite struct StalenessTests {
    static let now: Int64 = 1_700_000_000_000
    static let day: Int64 = 24 * 60 * 60 * 1000

    static func snippet(createdDaysAgo: Int64 = 60, lastUsedDaysAgo: Int64? = 1, useCount: Int = 5, isPinned: Bool = false) -> Snippet {
        Snippet(
            id: "test-id", title: "Test Snippet", content: "Test content",
            createdAt: now - createdDaysAgo * day, updatedAt: now,
            lastUsedAt: lastUsedDaysAgo.map { now - $0 * day },
            useCount: useCount, isPinned: isPinned)
    }

    @Test func recentlyUsed() {
        let snippet = Self.snippet(lastUsedDaysAgo: 2, useCount: 10)
        let analytics = Staleness.analyze(snippet, now: Self.now)
        #expect(analytics.snippet == snippet)
        #expect(analytics.daysUnused == 2)
        #expect(!analytics.isStale)
        #expect(analytics.stalenessReason == nil)
    }

    @Test func unusedNinetyDays() {
        let analytics = Staleness.analyze(Self.snippet(lastUsedDaysAgo: 100), now: Self.now)
        #expect(analytics.isStale)
        #expect(analytics.daysUnused == 100)
        #expect(analytics.stalenessReason == "Not used in 100 days")
    }

    @Test func neverUsedOldSnippet() {
        let analytics = Staleness.analyze(Self.snippet(createdDaysAgo: 45, lastUsedDaysAgo: nil, useCount: 0), now: Self.now)
        #expect(analytics.isStale)
        #expect(analytics.daysUnused == nil)
        #expect(analytics.stalenessReason == "Never used (created 45 days ago)")
    }

    @Test func neverUsedNewSnippet() {
        let analytics = Staleness.analyze(Self.snippet(createdDaysAgo: 15, lastUsedDaysAgo: nil, useCount: 0), now: Self.now)
        #expect(!analytics.isStale)
    }

    @Test func epochLastUsedIsAncientNotNever() throws {
        var snippet = Self.snippet(createdDaysAgo: 200, useCount: 5)
        snippet.lastUsedAt = 0
        let analytics = Staleness.analyze(snippet, now: Self.now)
        let days = try #require(analytics.daysUnused)
        #expect(days > 0)
        #expect(analytics.isStale)
        #expect(analytics.stalenessReason?.hasPrefix("Not used in") == true)
    }

    @Test func pinnedIsNeverStale() {
        #expect(!Staleness.analyze(Self.snippet(lastUsedDaysAgo: 200, isPinned: true), now: Self.now).isStale)
        #expect(!Staleness.analyze(
            Self.snippet(createdDaysAgo: 90, lastUsedDaysAgo: nil, useCount: 0, isPinned: true), now: Self.now).isStale)
    }

    @Test(arguments: [(89, false), (90, true)])
    func staleThreshold(daysAgo: Int64, stale: Bool) {
        #expect(Staleness.analyze(Self.snippet(lastUsedDaysAgo: daysAgo), now: Self.now).isStale == stale)
    }
}
