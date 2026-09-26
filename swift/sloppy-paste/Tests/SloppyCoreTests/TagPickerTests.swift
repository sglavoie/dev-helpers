import Testing
@testable import SloppyCore

@Suite struct TagPickerTests {
    typealias Row = TagPicker.Row

    @Test func marksSelectedAndUnselected() {
        #expect(TagPicker.buildRows(knownTags: ["personal", "work"], selectedTags: ["work"]) == [
            Row(tag: "personal", state: .unselected),
            Row(tag: "work", state: .selected),
        ])
    }

    @Test func marksParentsImplied() {
        #expect(TagPicker.buildRows(knownTags: ["work/projects"], selectedTags: ["work/projects"]) == [
            Row(tag: "work", state: .implied, impliedBy: "work/projects"),
            Row(tag: "work/projects", state: .selected),
        ])
    }

    @Test func keepsExplicitParentSelected() {
        #expect(TagPicker.buildRows(knownTags: ["work", "work/projects"], selectedTags: ["work"]) == [
            Row(tag: "work", state: .selected),
            Row(tag: "work/projects", state: .unselected),
        ])
    }

    @Test func includesUnknownSelectedTags() {
        let rows = TagPicker.buildRows(knownTags: ["personal"], selectedTags: ["work/projects/client-a"])
        #expect(rows.map(\.tag) == ["personal", "work", "work/projects", "work/projects/client-a"])
        #expect(rows.filter { $0.state == .implied }.map(\.impliedBy) == ["work/projects/client-a", "work/projects/client-a"])
    }

    @Test func dropsRedundantParentFromSelection() {
        #expect(TagPicker.buildRows(knownTags: [], selectedTags: ["work", "work/projects"]) == [
            Row(tag: "work", state: .implied, impliedBy: "work/projects"),
            Row(tag: "work/projects", state: .selected),
        ])
    }

    @Test func deduplicates() {
        #expect(TagPicker.buildRows(knownTags: ["work", "work"], selectedTags: ["work"]) == [
            Row(tag: "work", state: .selected),
        ])
    }

    @Test func filterRows() {
        let rows = TagPicker.buildRows(knownTags: ["work/projects/client-a", "personal/reading"], selectedTags: [])
        #expect(TagPicker.filterRows(rows, searchText: "   ").count == rows.count)
        #expect(TagPicker.filterRows(rows, searchText: "projects").map(\.tag) == ["work/projects", "work/projects/client-a"])
        #expect(TagPicker.filterRows(rows, searchText: "READING").map(\.tag) == ["personal/reading"])
    }

    @Test(arguments: [
        ("   ", ["work"], nil as TagPicker.CreateCandidate?),
        ("work", ["work"], nil),
        ("Work", ["work"], nil),
        ("work/new-thing", ["work"], .tag("work/new-thing")),
        ("Has Spaces", [], .tag("has-spaces")),
        ("Has Spaces", ["has-spaces"], nil),
        ("a/b/c/d/e/f", [], .error("Tag hierarchy too deep (max 5 levels)")),
    ])
    func createCandidate(text: String, known: [String], expected: TagPicker.CreateCandidate?) {
        #expect(TagPicker.createCandidate(searchText: text, knownTags: known) == expected)
    }

    @Test func createCandidateRejectsUnsupportedCharacters() {
        guard case .error = TagPicker.createCandidate(searchText: "work!", knownTags: []) else {
            Issue.record("expected an error candidate")
            return
        }
    }

    @Test(arguments: [
        (["personal"], "work", TagPicker.ToggleResult(tags: ["personal", "work"], changed: true)),
        (["personal", "work"], "work", .init(tags: ["personal"], changed: true)),
        (["work"], "work/projects", .init(tags: ["work/projects"], changed: true)),
        (["work/projects"], "work", .init(tags: ["work/projects"], changed: false, impliedBy: "work/projects")),
        (["work"], "WORK", .init(tags: [], changed: true)),
        (["work/projects"], "work/projects", .init(tags: [], changed: true)),
    ])
    func toggle(selected: [String], tag: String, expected: TagPicker.ToggleResult) {
        #expect(TagPicker.toggle(selectedTags: selected, tag: tag) == expected)
    }
}
