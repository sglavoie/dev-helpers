import Foundation
import Testing
@testable import SloppyCore

@Suite struct SnippetRepositoryRecordUseTests {
    let now: Int64 = 1_700_000_000_000

    @Test func recordsUsageAndHistoryWithOneTimestamp() throws {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "Test", content: "Content", now: 1)

        try repo.recordUse(id: snippet.id, placeholderValues: [.init(key: "name", value: "Ada", isSaved: true)], now: now)

        let stored = try #require(repo.snippet(id: snippet.id))
        #expect(stored.useCount == 1)
        #expect(stored.lastUsedAt == now)
        #expect(stored.updatedAt == now)
        #expect(repo.data.placeholderHistory(forKey: "name") == [
            PlaceholderHistoryValue(value: "Ada", useCount: 1, lastUsed: now, createdAt: now),
        ])
    }

    @Test func skipsBlankAndOptedOutValues() throws {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "Test", content: "Content", now: 1)

        try repo.recordUse(id: snippet.id, placeholderValues: [
            .init(key: "blank", value: "  ", isSaved: true),
            .init(key: "not-saved", value: "value", isSaved: false),
        ], now: now)

        #expect(repo.snippets[0].useCount == 1)
        #expect(repo.data.placeholderHistory(forKey: "blank").isEmpty)
        #expect(repo.data.placeholderHistory(forKey: "not-saved").isEmpty)
    }

    @Test func deduplicatesRepeatedPairsWithinOneUse() throws {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "Test", content: "Content", now: 1)

        try repo.recordUse(id: snippet.id, placeholderValues: [
            .init(key: "name", value: "Ada"), .init(key: "name", value: "Ada"),
        ], now: now)

        #expect(repo.data.placeholderHistory(forKey: "name").map(\.useCount) == [1])
    }

    @Test func updatesExistingHistoryWithTheNewTimestamp() throws {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "Test", content: "Content", now: now)
        let other = repo.addSnippet(title: "History", content: "Content", now: now)
        try repo.recordUse(id: other.id, placeholderValues: [.init(key: "name", value: "Ada")], now: now)

        try repo.recordUse(id: snippet.id, placeholderValues: [.init(key: "name", value: "Ada")], now: now + 1_000)

        #expect(repo.data.placeholderHistory(forKey: "name") == [
            PlaceholderHistoryValue(value: "Ada", useCount: 2, lastUsed: now + 1_000, createdAt: now),
        ])
    }

    @Test func missingSnippetChangesNothing() throws {
        var repo = SnippetRepository()
        let other = repo.addSnippet(title: "History", content: "Content", now: 1)
        try repo.recordUse(id: other.id, placeholderValues: [.init(key: "existing", value: "keep")], now: 2)
        let before = repo

        #expect(throws: StorageError.snippetNotFound) {
            try repo.recordUse(id: "missing", placeholderValues: [.init(key: "new", value: "value")], now: 3)
        }
        #expect(repo == before)
        #expect(StorageError.snippetNotFound.localizedDescription == "Snippet not found")
    }
}

@Suite struct SnippetRepositorySnippetTests {
    @Test func newSnippetIDsUseTheRaycastFormat() {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "T", content: "C", now: 1_700_000_000_000)
        #expect(snippet.id.wholeMatch(of: /snippet-1700000000000-[0-9a-z]{9}/) != nil)
        #expect(snippet.createdAt == 1_700_000_000_000)
        #expect(snippet.lastUsedAt == nil)
    }

    @Test func updateNormalizesChangedTagsAndBumpsUpdatedAt() throws {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "T", content: "C", now: 1)

        let updated = try repo.updateSnippet(id: snippet.id, now: 5) {
            $0.title = "New"
            $0.tags = ["Work", "work/projects"]
        }

        #expect(updated.title == "New")
        #expect(updated.tags == ["work/projects"])
        #expect(updated.updatedAt == 5)
        #expect(updated.createdAt == 1)
        #expect(repo.snippets == [updated])
    }

    @Test func duplicateResetsUsageAndFlags() throws {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "T", content: "C", tags: ["a"], now: 1)
        try repo.recordUse(id: snippet.id, now: 2)
        try repo.toggle(\.isPinned, id: snippet.id, now: 3)

        let copy = try repo.duplicateSnippet(id: snippet.id, now: 10, newID: "copy")

        #expect(copy == Snippet(id: "copy", title: "T (Copy)", content: "C", tags: ["a"], createdAt: 10, updatedAt: 10))
        #expect(repo.snippets.count == 2)
    }

    @Test func toggleFlipsFlagAndReturnsNewValue() throws {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "T", content: "C", now: 1)

        let favorited = try repo.toggle(\.isFavorite, id: snippet.id, now: 2)
        let unfavorited = try repo.toggle(\.isFavorite, id: snippet.id, now: 3)
        let archived = try repo.toggle(\.isArchived, id: snippet.id, now: 4)
        #expect(favorited)
        #expect(!unfavorited)
        #expect(archived)
        #expect(repo.snippets[0].updatedAt == 4)
    }

    @Test func deleteRemovesSnippet() {
        var repo = SnippetRepository()
        let snippet = repo.addSnippet(title: "T", content: "C", now: 1)
        repo.deleteSnippet(id: snippet.id)
        #expect(repo.snippets.isEmpty)
    }
}

@Suite struct SnippetRepositoryTagTests {
    func repo(_ tagLists: [[String]]) -> SnippetRepository {
        var repo = SnippetRepository()
        for (index, tags) in tagLists.enumerated() {
            repo.addSnippet(title: "Test \(index + 1)", content: "Content", tags: tags, now: 1, id: "s\(index)")
        }
        return repo
    }

    @Test(arguments: [
        ([], []),
        ([["work", "important"], ["personal", "work"]], ["important", "personal", "work"]),
        ([[], ["work"]], ["work"]),
        ([["zebra", "apple", "banana"]], ["apple", "banana", "zebra"]),
        ([["work/projects"]], ["work/projects"]),
    ] as [([[String]], [String])])
    func tagsAreUniqueAndSorted(tagLists: [[String]], expected: [String]) {
        #expect(repo(tagLists).tags == expected)
    }

    @Test(arguments: [
        (["work", "work/projects"], ["work/projects"]),
        (["work", "work/projects", "work/projects/client-a"], ["work/projects/client-a"]),
        (["work/projects"], ["work/projects"]),
        (["work/projects", "personal/notes"], ["personal/notes", "work/projects"]),
        (["work", "personal"], ["personal", "work"]),
    ])
    func addRemovesRedundantParents(tags: [String], expected: [String]) {
        #expect(repo([tags]).snippets[0].tags == expected)
    }

    @Test func deleteTagRemovesFromAllSnippets() {
        var r = repo([["work", "important"], ["work", "personal"]])
        r.deleteTag("work", now: 2)
        #expect(r.snippets.map(\.tags) == [["important"], ["personal"]])
        #expect(r.tags == ["important", "personal"])
    }

    @Test func deleteTagOnlyBumpsAffectedSnippets() {
        var r = repo([["work"], ["personal"]])
        r.deleteTag("work", now: 5)
        #expect(r.snippets.map(\.updatedAt) == [5, 1])
        #expect(r.snippets[1].tags == ["personal"])
    }

    @Test(arguments: [
        ([["work"], ["important", "work"]], "work", "office", 2, [["office"], ["important", "office"]]),
        ([["work", "work/projects", "work/projects/client-a"]], "work", "office", 1, [["office/projects/client-a"]]),
        ([["work", "work/projects"]], "work/projects", "work/tasks", 1, [["work/tasks"]]),
        ([["work"], ["work/admin"], ["personal"]], "work", "office", 2, [["office"], ["office/admin"], ["personal"]]),
        ([["work/admin", "office/admin"]], "work", "office", 1, [["office/admin"]]),
        ([["work"], ["personal"]], "work", "office", 1, [["office"], ["personal"]]),
    ] as [([[String]], String, String, Int, [[String]])])
    func renameTagCascadesToDescendants(
        tagLists: [[String]], from: String, to: String, affected: Int, expected: [[String]]
    ) {
        var r = repo(tagLists)
        let count = r.renameTag(from, to: to, now: 2)
        #expect(count == affected)
        #expect(r.snippets.map(\.tags) == expected)
    }

    @Test(arguments: [
        ([["work"], ["office"]], 1, [["office"], ["office"]]),
        ([["work", "office"]], 1, [["office"]]),
        ([["work", "important"], ["office", "important"], ["work", "office"]], 2, [["important", "office"], ["important", "office"], ["office"]]),
        ([["work"], ["personal"]], 1, [["office"], ["personal"]]),
        ([["work"], ["work/projects"], ["work/projects/client-a"], ["personal"]], 3,
         [["office"], ["office/projects"], ["office/projects/client-a"], ["personal"]]),
        ([["work/admin", "office/admin"]], 1, [["office/admin"]]),
    ] as [([[String]], Int, [[String]])])
    func mergeTagsIntoOffice(tagLists: [[String]], affected: Int, expected: [[String]]) throws {
        var r = repo(tagLists)
        let count = try r.mergeTags("work", into: "office", now: 2)
        #expect(count == affected)
        #expect(r.snippets.map(\.tags) == expected)
    }

    @Test func mergeTagsUpdatesTagList() throws {
        var r = repo([["work"], ["office"]])
        try r.mergeTags("work", into: "office", now: 2)
        #expect(r.tags == ["office"])
    }

    @Test func mergingATagWithItselfThrows() {
        var r = repo([["work"]])
        #expect(throws: StorageError.cannotMergeTagWithItself) { try r.mergeTags("work", into: "work", now: 2) }
    }

    @Test func operationsInSequence() throws {
        var r = repo([["work", "urgent"], ["work", "important"]])
        r.renameTag("work", to: "office", now: 2)
        r.deleteTag("urgent", now: 3)
        try r.mergeTags("important", into: "office", now: 4)
        #expect(r.tags == ["office"])
        #expect(r.snippets.map(\.tags) == [["office"], ["office"]])
    }
}

@Suite struct PlaceholderHistoryMutationTests {
    @Test func addEvictsLeastRecentlyUsedPastLimit() {
        var data = StorageData()
        for i in 0..<StorageConstants.maxStoredValuesPerKey {
            data.addPlaceholderValue(key: "k", value: "v\(i)", now: Int64(i + 1))
        }
        data.addPlaceholderValue(key: "k", value: "v0", now: 500)
        data.addPlaceholderValue(key: "k", value: "new", now: 1_000)

        let values = data.placeholderHistory(forKey: "k").map(\.value)
        #expect(values.count == StorageConstants.maxStoredValuesPerKey)
        #expect(values.contains("v0"))
        #expect(!values.contains("v1"))
        #expect(values.contains("new"))
    }

    @Test func rejectsEmptyKeyOrBlankValue() {
        var data = StorageData()
        let emptyKey = data.addPlaceholderValue(key: "", value: "x", now: 1)
        let blankValue = data.addPlaceholderValue(key: "k", value: " \n", now: 1)
        #expect(!emptyKey)
        #expect(!blankValue)
        #expect(data.placeholderHistory.isEmpty)
    }

    @Test func usageUpdateDeleteAndClear() throws {
        var data = StorageData()
        data.addPlaceholderValue(key: "k", value: "a", now: 1)
        data.addPlaceholderValue(key: "k", value: "b", now: 1)
        data.addPlaceholderValue(key: "j", value: "c", now: 1)

        let bumped = data.updatePlaceholderValueUsage(key: "k", value: "a", now: 9)
        let missing = data.updatePlaceholderValueUsage(key: "k", value: "zzz", now: 9)
        #expect(bumped)
        #expect(!missing)
        #expect(data.placeholderHistory(forKey: "k")[0] == PlaceholderHistoryValue(value: "a", useCount: 2, lastUsed: 9, createdAt: 1))
        #expect(data.placeholderKeys == ["j", "k"])

        try data.updatePlaceholderValue(key: "k", oldValue: "a", newValue: "a2")
        #expect(data.placeholderHistory(forKey: "k").map(\.value) == ["a2", "b"])
        #expect(throws: StorageError.duplicatePlaceholderValue) { try data.updatePlaceholderValue(key: "k", oldValue: "a2", newValue: "b") }
        #expect(throws: StorageError.emptyPlaceholderValue) { try data.updatePlaceholderValue(key: "k", oldValue: "a2", newValue: " ") }
        #expect(throws: StorageError.placeholderKeyNotFound) { try data.updatePlaceholderValue(key: "x", oldValue: "a2", newValue: "z") }
        #expect(throws: StorageError.placeholderValueNotFound) { try data.updatePlaceholderValue(key: "k", oldValue: "nope", newValue: "z") }

        data.deletePlaceholderValue(key: "k", value: "a2")
        data.deletePlaceholderValue(key: "k", value: "b")
        #expect(data.placeholderHistory["k"] == nil)

        data.clearPlaceholderHistory(forKey: "j")
        #expect(data.placeholderHistory.isEmpty)
        data.addPlaceholderValue(key: "k", value: "a", now: 1)
        data.clearAllPlaceholderHistory()
        #expect(data.placeholderHistory.isEmpty)
    }
}
