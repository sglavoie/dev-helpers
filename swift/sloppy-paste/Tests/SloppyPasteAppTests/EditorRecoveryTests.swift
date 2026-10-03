import Testing
import SloppyCore
@testable import SloppyPasteApp

@Suite @MainActor struct EditorRecoveryTests {
    @Test func reopeningPickerKeepsDirtyEditorAndStartsAtRoot() {
        let navigator = Navigator()
        navigator.push(.editor(.new))
        let session = navigator.startEditor(.new, draft: SnippetDraft())
        session.draft.title = "Unfinished"
        session.draft.content = "  Keep this\n"

        navigator.reset()

        #expect(navigator.current == .root)
        #expect(navigator.session == 1)
        #expect(navigator.pendingEditor === session)
        #expect(navigator.startEditor(.new, draft: SnippetDraft()) === session)
        #expect(session.draft.content == "  Keep this\n")
        navigator.push(.editor(session.mode))
        #expect(navigator.current == .editor(.new))
    }

    @Test func clipboardDraftSurvivesBeforeAnyEdits() {
        let navigator = Navigator()
        let session = navigator.startEditor(.fromClipboard, draft: SnippetDraft(title: "Clipboard", content: "Text"))
        navigator.reset()
        #expect(navigator.pendingEditor === session)
    }

    @Test func cleanEditorsAreReleasedAndSavedOrDiscardedDraftsDoNotReturn() {
        let navigator = Navigator()
        let clean = navigator.startEditor(.edit(snippetID: "saved"), draft: SnippetDraft(title: "Saved", content: "text"))
        let dirty = navigator.startEditor(.new, draft: SnippetDraft())
        dirty.draft.title = "Draft"
        navigator.reset()
        #expect(navigator.editorSession(for: clean.mode) == nil)
        navigator.finishEditor(dirty.mode)
        navigator.reset()
        #expect(navigator.pendingEditor == nil)
        #expect(navigator.editorSessions.isEmpty)
    }

    @Test func differentEditorsDoNotOverwriteEachOther() {
        let navigator = Navigator()
        let first = navigator.startEditor(.new, draft: SnippetDraft())
        first.draft.title = "First"
        let second = navigator.startEditor(.edit(snippetID: "other"), draft: SnippetDraft(title: "Other", content: "text"))
        second.draft.content = "Changed"
        navigator.reset()
        #expect(navigator.pendingEditor === second)
        navigator.finishEditor(second.mode)
        #expect(navigator.pendingEditor === first)
    }
}
