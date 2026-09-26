import Foundation
import Testing
@testable import SloppyCore

@Suite struct SnippetDraftTests {
    @Test(arguments: [
        ("Hello world\nsecond line", "Hello world"),
        ("   padded  \nrest", "padded"),
        (String(repeating: "a", count: 50), String(repeating: "a", count: 50)),
        (String(repeating: "b", count: 51), String(repeating: "b", count: 47) + "..."),
        ("\nstarts with newline", ""),
        ("😀 émoji 漢字", "😀 émoji 漢字"),
    ])
    func suggestedTitle(input: String, expected: String) {
        #expect(SnippetDraft.suggestedTitle(for: input) == expected)
    }

    @Test func clipboardDraftKeepsRawContent() throws {
        let draft = try #require(SnippetDraft(clipboard: "  line one\nline two  "))
        #expect(draft.title == "line one")
        #expect(draft.content == "  line one\nline two  ")
        #expect(SnippetDraft(clipboard: nil) == nil)
        #expect(SnippetDraft(clipboard: " \n\t") == nil)
    }

    @Test func parseTagsNormalisesAndDeduplicates() {
        let parsed = SnippetDraft.parseTags(" Work/Projects, my tag,, work/projects ,email ")
        #expect(parsed.tags == ["work/projects", "my-tag", "email"])
        #expect(parsed.error == nil)
        #expect(SnippetDraft.parseTags("").tags.isEmpty)
    }

    @Test func parseTagsReportsFirstInvalidTag() {
        let parsed = SnippetDraft.parseTags("ok, bad!, /also-bad")
        #expect(parsed.tags == ["ok"])
        #expect(parsed.error == "bad!: Tag can only contain letters, numbers, hyphens, underscores, and slashes")
    }

    @Test func errorsByField() {
        var draft = SnippetDraft(title: "  ", content: "Hi {{tone[A||B]}}", tags: [])
        draft.tagsText = "a//b"
        #expect(draft.errors[.title] == "Title is required")
        #expect(draft.errors[.content] != nil)
        #expect(draft.errors[.tags] == "a//b: Tag cannot contain consecutive slashes")
        #expect(draft.errors[.description] == nil)
        #expect(draft.firstErrorField == .title)

        draft.title = "T"
        #expect(draft.firstErrorField == .content)
        draft.content = "Hi {{name}}"
        #expect(draft.firstErrorField == .tags)
        draft.tagsText = "a/b"
        #expect(draft.errors.isEmpty)
    }

    @Test func createTrimsAndNormalisesTags() throws {
        var repository = SnippetRepository()
        var draft = SnippetDraft(title: "  Greeting ", content: "\nHello {{name}}\n", description: " desc ")
        draft.tagsText = "work, work/email"
        let snippet = try draft.create(in: &repository, now: 1_000, id: "snippet-1")
        #expect(snippet.title == "Greeting")
        #expect(snippet.content == "Hello {{name}}")
        #expect(snippet.description == "desc")
        #expect(snippet.tags == ["work/email"])
        #expect(snippet.createdAt == 1_000)
        #expect(repository.snippets == [snippet])
    }

    @Test func createRejectsInvalidDraft() {
        var repository = SnippetRepository()
        let draft = SnippetDraft(title: "", content: "x")
        #expect(throws: SnippetDraftError.invalid(field: .title, message: "Title is required")) {
            try draft.create(in: &repository, now: 1)
        }
        #expect(repository.snippets.isEmpty)
    }

    @Test func updateKeepsUsageAndFlags() throws {
        var repository = SnippetRepository()
        var original = repository.addSnippet(title: "Old", content: "old", tags: ["x"], now: 1, id: "s")
        original = try repository.updateSnippet(id: "s", now: 2) {
            $0.useCount = 4
            $0.isPinned = true
        }
        var draft = SnippetDraft(snippet: original)
        #expect(draft.tagsText == "x")
        draft.title = "New"
        draft.content = "new "
        draft.tagsText = ""
        let updated = try draft.update(id: "s", in: &repository, now: 3)
        #expect(updated.title == "New")
        #expect(updated.content == "new")
        #expect(updated.tags == [])
        #expect(updated.useCount == 4)
        #expect(updated.isPinned)
        #expect(updated.createdAt == 1)
        #expect(updated.updatedAt == 3)
    }

    @Test func updateMissingSnippetThrows() {
        var repository = SnippetRepository()
        #expect(throws: StorageError.snippetNotFound) {
            try SnippetDraft(title: "T", content: "c").update(id: "gone", in: &repository, now: 1)
        }
    }

    @Test func syntaxHelpersSelectTheirKey() {
        #expect(PlaceholderSyntaxHelper.all.map(\.key) == Array("1234567"))
        for helper in PlaceholderSyntaxHelper.all {
            let selected = (helper.content as NSString).substring(with: helper.keyRange)
            #expect(selected == (helper.key == "7" ? "tone" : "key"))
            // Every helper is valid placeholder syntax on its own.
            #expect(Validation.validateContent(helper.content).isValid, "\(helper.title)")
        }
        #expect(PlaceholderSyntaxHelper.all[3].keyRange == NSRange(location: 9, length: 3))
    }
}
