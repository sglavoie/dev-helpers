import Foundation
import Testing
@testable import SloppyCore

@Suite struct TagStatisticsTests {
    static func snippet(_ id: String, _ tags: [String], lastUsedAt: Int64?, useCount: Int) -> Snippet {
        Snippet(id: id, title: "Snippet \(id)", content: "Content \(id)", tags: tags,
                createdAt: 1000, updatedAt: 2000, lastUsedAt: lastUsedAt, useCount: useCount)
    }

    @Test func computesStatistics() throws {
        let snippets = [
            Self.snippet("1", ["work", "dev"], lastUsedAt: 5000, useCount: 10),
            Self.snippet("2", ["work", "api"], lastUsedAt: 8000, useCount: 25),
            Self.snippet("3", ["personal"], lastUsedAt: 3000, useCount: 5),
        ]
        let stats = TagStatistics.compute(snippets: snippets, tags: ["work", "dev", "api", "personal"])
        #expect(stats.count == 4)
        #expect(stats[0] == TagStatistics(tag: "work", snippetCount: 2, lastUsedAt: 8000, totalUsageCount: 35))
        #expect(stats[1] == TagStatistics(tag: "dev", snippetCount: 1, lastUsedAt: 5000, totalUsageCount: 10))
        #expect(stats[3] == TagStatistics(tag: "personal", snippetCount: 1, lastUsedAt: 3000, totalUsageCount: 5))
        #expect(!stats[0].neverUsed)
    }

    @Test func unusedTagsIgnoreArchivedSnippets() {
        var archived = Self.snippet("2", ["old", "work"], lastUsedAt: nil, useCount: 0)
        archived.isArchived = true
        let snippets = [Self.snippet("1", ["work"], lastUsedAt: nil, useCount: 0), archived]
        #expect(TagStatistics.unusedTags(snippets: snippets, tags: ["old", "work", "orphan"]) == ["old", "orphan"])
        #expect(TagStatistics.unusedTags(snippets: [], tags: []).isEmpty)
    }

    @Test func tagsWithoutSnippets() {
        let stats = TagStatistics.compute(
            snippets: [Self.snippet("1", ["work"], lastUsedAt: 5000, useCount: 10)],
            tags: ["work", "personal", "dev"])
        #expect(stats[1] == TagStatistics(tag: "personal", snippetCount: 0, lastUsedAt: nil, totalUsageCount: 0))
        #expect(stats[1].neverUsed)
        #expect(stats[2] == TagStatistics(tag: "dev", snippetCount: 0, lastUsedAt: nil, totalUsageCount: 0))
    }

    @Test func neverUsedWhenSnippetsUnused() {
        let stats = TagStatistics.compute(
            snippets: [
                Self.snippet("1", ["work"], lastUsedAt: nil, useCount: 0),
                Self.snippet("2", ["work"], lastUsedAt: nil, useCount: 0),
            ],
            tags: ["work"])
        #expect(stats == [TagStatistics(tag: "work", snippetCount: 2, lastUsedAt: nil, totalUsageCount: 0)])
        #expect(stats[0].neverUsed)
    }

    @Test func empty() {
        #expect(TagStatistics.compute(snippets: [], tags: []).isEmpty)
    }

    @Test func epochLastUsedIsNotNeverUsed() {
        let stats = TagStatistics.compute(
            snippets: [Self.snippet("1", ["work"], lastUsedAt: 0, useCount: 1)], tags: ["work"])
        #expect(!stats[0].neverUsed)
        #expect(stats[0].lastUsedAt == 0)
    }

    @Test func sumsUsage() {
        let stats = TagStatistics.compute(
            snippets: [
                Self.snippet("1", ["work"], lastUsedAt: 5000, useCount: 100),
                Self.snippet("2", ["work"], lastUsedAt: 6000, useCount: 50),
                Self.snippet("3", ["work"], lastUsedAt: 7000, useCount: 25),
            ],
            tags: ["work"])
        #expect(stats[0].totalUsageCount == 175)
    }

    static let sortFixture = [
        TagStatistics(tag: "zebra", snippetCount: 5, lastUsedAt: 1000, totalUsageCount: 50),
        TagStatistics(tag: "alpha", snippetCount: 10, lastUsedAt: 5000, totalUsageCount: 100),
        TagStatistics(tag: "beta", snippetCount: 3, lastUsedAt: nil, totalUsageCount: 0),
        TagStatistics(tag: "gamma", snippetCount: 8, lastUsedAt: 8000, totalUsageCount: 200),
    ]

    @Test(arguments: [
        (TagSortOption.nameAsc, ["alpha", "beta", "gamma", "zebra"]),
        (.snippetCountDesc, ["alpha", "gamma", "zebra", "beta"]),
        (.lastUsedDesc, ["gamma", "alpha", "zebra", "beta"]),
        (.lastUsedAsc, ["beta", "zebra", "alpha", "gamma"]),
        (.usageCountDesc, ["gamma", "alpha", "zebra", "beta"]),
        (.neverUsedFirst, ["beta", "alpha", "gamma", "zebra"]),
    ])
    func sorts(option: TagSortOption, expected: [String]) {
        #expect(TagStatistics.sorted(Self.sortFixture, by: option).map(\.tag) == expected)
    }

    @Test func tieBreaksByName() {
        let equalTimes = [
            TagStatistics(tag: "zebra", snippetCount: 1, lastUsedAt: 1000, totalUsageCount: 1),
            TagStatistics(tag: "alpha", snippetCount: 1, lastUsedAt: 1000, totalUsageCount: 1),
        ]
        #expect(TagStatistics.sorted(equalTimes, by: .lastUsedAsc).map(\.tag) == ["alpha", "zebra"])

        let neverUsed = [
            TagStatistics(tag: "zebra", snippetCount: 1, lastUsedAt: nil, totalUsageCount: 0),
            TagStatistics(tag: "alpha", snippetCount: 1, lastUsedAt: nil, totalUsageCount: 0),
        ]
        #expect(TagStatistics.sorted(neverUsed, by: .lastUsedAsc).map(\.tag) == ["alpha", "zebra"])

        let equalCounts = [
            TagStatistics(tag: "zebra", snippetCount: 5, lastUsedAt: 1000, totalUsageCount: 10),
            TagStatistics(tag: "alpha", snippetCount: 5, lastUsedAt: 2000, totalUsageCount: 20),
        ]
        #expect(TagStatistics.sorted(equalCounts, by: .snippetCountDesc).map(\.tag) == ["alpha", "zebra"])
    }

    @Test func labels() {
        #expect(TagSortOption.lastUsedAsc.label == "Least Recently Used")
    }
}

@Suite struct FormattingTests {
    static let now: Int64 = 1_700_000_000_000
    static let second: Int64 = 1000
    static let hour: Int64 = 60 * 60 * 1000
    static let day: Int64 = 24 * hour

    static let relativeTimeCases: [(Int64, String)] = [
        (30 * second, "just now"),
        (5 * 60 * second, "5m ago"),
        (3 * hour, "3h ago"),
        (2 * day, "2d ago"),
        (4 * day, "4d ago"),
        (14 * day, "2w ago"),
        (28 * day, "4w ago"),
        (29 * day, "4w ago"),
        (30 * day, "1mo ago"),
        (60 * day, "2mo ago"),
        (400 * day, "1y ago"),
        (2 * 365 * day, "2y ago"),
        (-second, "just now"),
    ]

    @Test(arguments: relativeTimeCases)
    func relativeTime(ago: Int64, expected: String) {
        #expect(Formatting.relativeTime(Self.now - ago, now: Self.now) == expected)
    }

    @Test(arguments: [(0, "0"), (100, "100"), (1000, "1,000"), (1_234_567, "1,234,567")])
    func number(value: Int, expected: String) {
        #expect(Formatting.number(value) == expected)
    }

    @Test(arguments: [(999, "999 bytes"), (1500, "1.5 KB"), (2_250_000, "2.25 MB")])
    func size(bytes: Int, expected: String) {
        #expect(Formatting.size(bytes) == expected)
    }

    @Test func absoluteDate() throws {
        let utc = try #require(TimeZone(identifier: "UTC"))
        #expect(Formatting.absoluteDate(0, timeZone: utc) == "Jan 1, 1970, 12:00 AM")
    }
}
