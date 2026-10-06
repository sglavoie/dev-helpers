import Foundation
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

enum EditorMode: Hashable, Codable {
    case new
    case edit(snippetID: String)
    case fromClipboard
}

/// An editor's working copy, persisted separately from saved snippets.
@MainActor
@Observable
final class SnippetEditorSession {
    let mode: EditorMode
    let original: SnippetDraft
    var draft: SnippetDraft { didSet { didChange() } }
    @ObservationIgnored var didChange: @MainActor () -> Void = {}

    init(mode: EditorMode, draft: SnippetDraft, original: SnippetDraft? = nil) {
        self.mode = mode
        self.original = original ?? draft
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
    private(set) var draftRecoveryError: String?
    private let draftURL: URL?
    private var canWriteDrafts = true

    static var defaultDraftURL: URL {
        StorageFile.defaultURL.deletingLastPathComponent().appending(path: "drafts.json")
    }

    private struct SavedDraft: Codable {
        var mode: EditorMode
        var original: SnippetDraft
        var draft: SnippetDraft
    }

    private struct SavedDrafts: Codable {
        var version = 1
        var drafts: [SavedDraft]
    }

    // Persistence is opt-in so isolated navigators and tests never touch live data.
    init(draftURL: URL? = nil) {
        self.draftURL = draftURL
        guard let draftURL else { return }
        do {
            let data: Data
            do { data = try Data(contentsOf: draftURL) }
            catch CocoaError.fileReadNoSuchFile { return }
            let saved = try JSONDecoder().decode(SavedDrafts.self, from: data)
            guard saved.version == 1, Set(saved.drafts.map(\.mode)).count == saved.drafts.count else {
                throw CocoaError(.coderReadCorrupt)
            }
            editorSessions = saved.drafts.map {
                SnippetEditorSession(mode: $0.mode, draft: $0.draft, original: $0.original)
            }.filter(\.hasUnsavedChanges)
            for session in editorSessions { observe(session) }
        } catch {
            // Preserve unreadable/unknown data for recovery instead of replacing it.
            canWriteDrafts = false
            draftRecoveryError = "Cannot restore drafts from \(draftURL.path). The file is preserved. Fix it and restart; new drafts stay in memory until then."
        }
    }

    private func observe(_ session: SnippetEditorSession) {
        session.didChange = { [weak self] in self?.saveDrafts() }
    }

    private func saveDrafts() {
        guard let draftURL, canWriteDrafts else { return }
        do {
            let saved = SavedDrafts(drafts: editorSessions.filter(\.hasUnsavedChanges).map {
                SavedDraft(mode: $0.mode, original: $0.original, draft: $0.draft)
            })
            try FileManager.default.createDirectory(at: draftURL.deletingLastPathComponent(), withIntermediateDirectories: true)
            let data = try JSONEncoder().encode(saved)
            try data.write(to: draftURL, options: .atomic)
            draftRecoveryError = nil
        } catch {
            draftRecoveryError = "Cannot save drafts for recovery: \(error.localizedDescription) Keep the app open until this is resolved."
        }
    }

    var pendingEditor: SnippetEditorSession? {
        editorSessions.last { $0.hasUnsavedChanges }
    }

    func editorSession(for mode: EditorMode) -> SnippetEditorSession? {
        editorSessions.first { $0.mode == mode }
    }

    func startEditor(_ mode: EditorMode, draft: SnippetDraft) -> SnippetEditorSession {
        if let existing = editorSession(for: mode) { return existing }
        let session = SnippetEditorSession(mode: mode, draft: draft)
        observe(session)
        editorSessions.append(session)
        saveDrafts()
        return session
    }

    /// Only saving or explicitly discarding removes an unfinished editor.
    func finishEditor(_ mode: EditorMode) {
        editorSessions.removeAll { $0.mode == mode }
        saveDrafts()
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
