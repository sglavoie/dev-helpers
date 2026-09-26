import Testing
@testable import SloppyCore

@Suite struct PlaceholderHistoryRankingTests {
    static let now: Int64 = 1_700_000_000_000
    static let hour: Int64 = 60 * 60 * 1000
    static let day: Int64 = 24 * hour

    static func value(_ value: String, _ useCount: Int, ago: Int64 = 0, createdAgo: Int64? = nil) -> PlaceholderHistoryValue {
        PlaceholderHistoryValue(value: value, useCount: useCount, lastUsed: now - ago, createdAt: now - (createdAgo ?? ago))
    }

    typealias R = PlaceholderHistoryRanking

    @Test func ranksByFrequencyWhenEquallyRecent() {
        let values = [Self.value("low-freq", 1), Self.value("high-freq", 10), Self.value("mid-freq", 5)]
        #expect(R.rank(values, now: Self.now).map(\.value) == ["high-freq", "mid-freq", "low-freq"])
    }

    @Test func recentBeatsOldFrequent() {
        let values = [Self.value("old-frequent", 100, ago: 30 * Self.day), Self.value("recent-rare", 1)]
        #expect(R.rank(values, now: Self.now).first?.value == "recent-rare")
    }

    @Test func emptyAndSingle() {
        #expect(R.rank([], now: Self.now).isEmpty)
        #expect(R.rank([Self.value("only", 1)], now: Self.now).map(\.value) == ["only"])
    }

    @Test func combinesFrequencyAndRecency() {
        let values = [
            Self.value("very-old-frequent", 50, ago: 60 * Self.day),
            Self.value("recent-frequent", 50, ago: Self.day),
            Self.value("very-recent-rare", 1, ago: Self.hour),
        ]
        #expect(R.rank(values, now: Self.now).first?.value == "recent-frequent")
    }

    @Test func rankedValues() {
        #expect(R.rankedValues([Self.value("first", 10), Self.value("second", 5)], now: Self.now) == ["first", "second"])
        let four = [Self.value("first", 10), Self.value("second", 9), Self.value("third", 8), Self.value("fourth", 7)]
        #expect(R.rankedValues(four, now: Self.now, limit: 2) == ["first", "second"])
        #expect(R.rankedValues([], now: Self.now).isEmpty)
    }

    @Test func filter() {
        #expect(R.filter(["Alice", "Bob", "alice@example.com", "charlie"], query: "alice") == ["Alice", "alice@example.com"])
        #expect(R.filter(["user@example.com", "admin@example.com", "test@test.com"], query: "example")
            == ["user@example.com", "admin@example.com"])
        #expect(R.filter(["one", "two", "three"], query: "") == ["one", "two", "three"])
        #expect(R.filter(["one", "two", "three"], query: "   ").count == 3)
        #expect(R.filter(["one", "two", "three"], query: "xyz").isEmpty)
    }

    @Test func topRankedValue() {
        #expect(R.topRankedValue([Self.value("second", 5), Self.value("first", 10)], now: Self.now) == "first")
        #expect(R.topRankedValue([], now: Self.now) == nil)
        #expect(R.topRankedValue([Self.value("only", 1)], now: Self.now) == "only")
    }

    @Test func lastUsedValue() {
        let values = [
            Self.value("old", 100, ago: 30 * Self.day),
            Self.value("recent", 1, ago: Self.hour),
            Self.value("very-old", 50, ago: 60 * Self.day),
        ]
        #expect(R.lastUsedValue(values) == "recent")
        #expect(R.lastUsedValue([]) == nil)
        #expect(R.lastUsedValue([Self.value("only", 1)]) == "only")
        let tied = [Self.value("first-tied", 10, createdAgo: Self.day), Self.value("second-tied", 5)]
        #expect(R.lastUsedValue(tied) == "first-tied")
    }

    @Test func lastUsedIgnoresFrequency() {
        let values = [
            Self.value("most-frequent", 100, ago: 10 * Self.day, createdAgo: 30 * Self.day),
            Self.value("most-recent", 1),
        ]
        #expect(R.lastUsedValue(values) == "most-recent")
        #expect(R.topRankedValue(values, now: Self.now) != nil)
    }

    @Test func keyStats() {
        let stats = PlaceholderKeyStats(key: "testKey", values: [
            Self.value("val1", 5, ago: Self.day),
            Self.value("val2", 10),
            Self.value("val3", 3, ago: 2 * Self.day),
        ])
        #expect(stats == PlaceholderKeyStats(key: "testKey", valueCount: 3, totalUseCount: 18, lastUsed: Self.now))
        #expect(PlaceholderKeyStats(key: "emptyKey", values: [])
            == PlaceholderKeyStats(key: "emptyKey", valueCount: 0, totalUseCount: 0, lastUsed: nil))
        #expect(PlaceholderKeyStats(key: "singleKey", values: [Self.value("val", 7)])
            == PlaceholderKeyStats(key: "singleKey", valueCount: 1, totalUseCount: 7, lastUsed: Self.now))
    }

    static func stat(_ key: String, _ lastUsed: Int64?, valueCount: Int = 1, totalUseCount: Int = 1) -> PlaceholderKeyStats {
        PlaceholderKeyStats(key: key, valueCount: valueCount, totalUseCount: totalUseCount, lastUsed: lastUsed)
    }

    @Test(arguments: [
        ([stat("c", 1), stat("a", 1), stat("b", 1)], PlaceholderKeySortOption.nameAsc, ["a", "b", "c"]),
        ([stat("a", 1, valueCount: 2), stat("b", 1, valueCount: 5), stat("c", 1, valueCount: 3)], .valueCountDesc, ["b", "c", "a"]),
        ([stat("a", 1, totalUseCount: 2), stat("b", 1, totalUseCount: 10), stat("c", 1, totalUseCount: 5)], .usageDesc, ["b", "c", "a"]),
        ([stat("old", 100), stat("never", nil), stat("recent", 1_000_000)], .lastUsedDesc, ["recent", "old", "never"]),
        ([stat("epoch", 0), stat("never", nil), stat("recent", 1_000_000)], .lastUsedDesc, ["recent", "epoch", "never"]),
        ([stat("a", nil), stat("b", nil)], .lastUsedDesc, ["a", "b"]),
    ])
    func sortKeyStats(stats: [PlaceholderKeyStats], option: PlaceholderKeySortOption, expected: [String]) {
        #expect(PlaceholderKeyStats.sorted(stats, by: option).map(\.key) == expected)
    }
}
