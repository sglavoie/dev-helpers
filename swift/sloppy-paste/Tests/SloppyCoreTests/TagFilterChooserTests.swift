import Foundation
import Testing
@testable import SloppyCore

@Suite struct TagFilterChooserTests {
    typealias C = TagFilterChooser

    static func snippet(_ id: String, _ tags: [String], archived: Bool = false) -> Snippet {
        var snippet = Snippet(
            id: id, title: "Snippet \(id)", content: "Content \(id)", tags: tags, createdAt: 1000, updatedAt: 2000)
        snippet.isArchived = archived
        return snippet
    }

    static let snippets = [
        snippet("1", ["work/projects/client-a"]),
        snippet("2", ["work"]),
        snippet("3", ["homework"]),
        snippet("4", []),
        snippet("5", ["old"], archived: true),
        snippet("6", [], archived: true),
    ]

    @Test func listsAllTagsUntaggedAndHierarchicalCounts() {
        let entries = C.entries(snippets: Self.snippets, showArchived: false, selectedTag: nil, searchText: "")
        #expect(entries == [
            .allTags,
            .untagged(count: 1),
            .tag("homework", count: 1),
            .tag("work", count: 2),
            .tag("work/projects", count: 1),
            .tag("work/projects/client-a", count: 1),
        ])
    }

    @Test func archiveViewUsesArchivedSnippets() {
        let entries = C.entries(snippets: Self.snippets, showArchived: true, selectedTag: nil, searchText: "")
        #expect(entries == [.allTags, .untagged(count: 1), .tag("old", count: 1)])
    }

    @Test func filtersBySubstringOfPath() {
        let entries = C.entries(snippets: Self.snippets, showArchived: false, selectedTag: nil, searchText: " WO ")
        #expect(entries.map(\.id) == ["tag:homework", "tag:work", "tag:work/projects", "tag:work/projects/client-a"])
        let untagged = C.entries(snippets: Self.snippets, showArchived: false, selectedTag: nil, searchText: "untag")
        #expect(untagged == [.untagged(count: 1)])
        let all = C.entries(snippets: Self.snippets, showArchived: false, selectedTag: nil, searchText: "all")
        #expect(all == [.allTags])
    }

    @Test func hidesUntaggedWhenEmptyUnlessSelected() {
        let tagged = [Self.snippet("1", ["work"])]
        #expect(C.entries(snippets: tagged, showArchived: false, selectedTag: nil, searchText: "")
            == [.allTags, .tag("work", count: 1)])
        #expect(
            C.entries(snippets: tagged, showArchived: false, selectedTag: TagHierarchy.untaggedSentinel, searchText: "")
                == [.allTags, .untagged(count: 0), .tag("work", count: 1)])
    }

    @Test func keepsSelectedTagMissingFromView() {
        let entries = C.entries(snippets: Self.snippets, showArchived: true, selectedTag: "work/projects", searchText: "")
        #expect(entries == [
            .allTags, .untagged(count: 1), .tag("old", count: 1), .tag("work", count: 0), .tag("work/projects", count: 0),
        ])
    }

    @Test func entriesMapToFilterValues() {
        #expect(C.Entry.allTags.selectedTag == nil)
        #expect(C.Entry.untagged(count: 0).selectedTag == TagHierarchy.untaggedSentinel)
        #expect(C.Entry.tag("work", count: 3).selectedTag == "work")
    }
}
