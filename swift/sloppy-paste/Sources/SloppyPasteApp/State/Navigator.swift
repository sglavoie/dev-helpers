import Observation
import SloppyCore

/// A screen inside the picker panel.
enum Route: Hashable {
    case root
    case placeholderForm(snippetID: String, mode: PlaceholderFormMode)
    case editor(EditorMode)
    case tagPicker(snippetID: String)
    case manageTags
    case renameTag(String)
    case mergeTags(String)
    case history(key: String?)
}

enum EditorMode: Hashable {
    case new
    case edit(snippetID: String)
    case fromClipboard
}

/// An editor's working copy survives panel dismissal, but never app termination.
@MainActor
@Observable
final class SnippetEditorSession {
    let mode: EditorMode
    let original: SnippetDraft
    var draft: SnippetDraft

    init(mode: EditorMode, draft: SnippetDraft) {
        self.mode = mode
        self.original = draft
        self.draft = draft
    }

    var hasUnsavedChanges: Bool {
        // Clipboard text has not been saved even before the first keystroke.
        mode == .fromClipboard || draft != original
    }
}

/// The picker's route stack. The root screen is always at the bottom.
@MainActor
@Observable
final class Navigator {
    private(set) var stack: [Route] = [.root]
    private(set) var editorSessions: [SnippetEditorSession] = []

    var pendingEditor: SnippetEditorSession? {
        editorSessions.last { $0.hasUnsavedChanges }
    }

    func editorSession(for mode: EditorMode) -> SnippetEditorSession? {
        editorSessions.first { $0.mode == mode }
    }

    func startEditor(_ mode: EditorMode, draft: SnippetDraft) -> SnippetEditorSession {
        if let existing = editorSession(for: mode) { return existing }
        let session = SnippetEditorSession(mode: mode, draft: draft)
        editorSessions.append(session)
        return session
    }

    /// Only saving or explicitly discarding removes an unfinished editor.
    func finishEditor(_ mode: EditorMode) {
        editorSessions.removeAll { $0.mode == mode }
    }

    /// Runs before every stack change. The panel ends text editing here: a
    /// screen removed while its field is editing otherwise leaves an orphaned
    /// field editor as first responder, and the next screen cannot take focus.
    @ObservationIgnored var willChange: @MainActor () -> Void = {}
    /// Bumped each time the panel opens. The root screen takes it as its
    /// identity, so its search, filters and selection start clean on every
    /// open instead of surviving in the long-lived hosting view.
    private(set) var session = 0

    var current: Route { stack[stack.count - 1] }
    var canPop: Bool { stack.count > 1 }

    func push(_ route: Route) {
        willChange()
        stack.append(route)
    }

    /// Pops one screen. Returns false when already at the root.
    @discardableResult
    func pop() -> Bool {
        guard canPop else { return false }
        willChange()
        stack.removeLast()
        return true
    }

    /// Returns to a fresh root screen, as when the panel opens.
    func reset() {
        willChange()
        editorSessions.removeAll { !$0.hasUnsavedChanges }
        stack = [.root]
        session += 1
    }
}
