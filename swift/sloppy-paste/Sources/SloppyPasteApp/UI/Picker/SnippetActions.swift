import SloppyCore

/// Snippet mutations offered from the picker (duplicate, delete, pin,
/// bookmark, archive). Each reloads external edits first, saves, and reports
/// the outcome as an in-panel toast with the extension's wording.
@MainActor
struct SnippetActions {
    let store: SnippetStore
    let toasts: ToastCenter

    /// Returns the copy, or nil on failure.
    @discardableResult
    func duplicate(_ snippet: Snippet) -> Snippet? {
        let copy = perform(failure: "Failed to duplicate snippet") { repository, now in
            try repository.duplicateSnippet(id: snippet.id, now: now)
        }
        if let copy {
            toasts.success("Snippet duplicated", message: "Created \"\(copy.title)\"")
        }
        return copy
    }

    func delete(_ snippet: Snippet) {
        let deleted: Void? = perform(failure: "Failed to delete snippet") { repository, _ in
            repository.deleteSnippet(id: snippet.id)
        }
        if deleted != nil {
            toasts.success("Snippet deleted")
        }
    }

    func togglePin(_ snippet: Snippet) {
        if let pinned = toggle(\.isPinned, snippet, failure: "Failed to toggle pin") {
            toasts.success(pinned ? "Snippet pinned" : "Snippet unpinned")
        }
    }

    func toggleBookmark(_ snippet: Snippet) {
        if let bookmarked = toggle(\.isFavorite, snippet, failure: "Failed to toggle favorite") {
            toasts.success(bookmarked ? "Bookmarked" : "Removed bookmark")
        }
    }

    func toggleArchive(_ snippet: Snippet) {
        if let archived = toggle(\.isArchived, snippet, failure: "Failed to toggle archive") {
            toasts.success(archived ? "Snippet archived" : "Snippet unarchived")
        }
    }

    private func toggle(_ flag: WritableKeyPath<Snippet, Bool>, _ snippet: Snippet, failure: String) -> Bool? {
        perform(failure: failure) { repository, now in
            try repository.toggle(flag, id: snippet.id, now: now)
        }
    }

    private func perform<T>(failure: String, _ body: (inout SnippetRepository, Int64) throws -> T) -> T? {
        store.reloadIfChanged()
        do {
            return try store.mutate(body)
        } catch {
            toasts.failure(failure, message: error.localizedDescription)
            return nil
        }
    }
}
