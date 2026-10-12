import SwiftUI
import SloppyCore

extension PickerRootView {
    // MARK: State

    var hasSearchOrFilters: Bool {
        !query.isEmpty || options != SnippetFilterOptions()
    }

    func clearSearchAndFilters() {
        query = ""
        options = SnippetFilterOptions()
        selectedID = nil
        searchFocused = false
    }

    func listState(now: Int64) -> PickerListState {
        PickerListModel.build(
            store.snippets, query: query, options: options, sort: sort, showRecentSection: showRecentSection, now: now)
    }

    /// The selected item, falling back to the first one when the selection
    /// is unset or was filtered away.
    func effectiveSelection(in items: [PickerItem]) -> PickerItem? {
        items.first { $0.id == selectedID } ?? items.first
    }

    /// ⌘G: choose the tag filter from a filterable list.
    func openTagChooser() {
        searchFocused = false
        tagChooser = TagFilterChooserModel(
            snippets: store.snippets, showArchived: options.showArchivedSnippets, currentTag: options.selectedTag
        ) { tag in
            options.selectedTag = tag
            closeTagChooser()
        }
    }

    /// Ends editing first: removing the chooser's field while it edits would
    /// leave an orphaned field editor as first responder.
    func closeTagChooser() {
        NSApp.keyWindow?.makeFirstResponder(nil)
        tagChooser = nil
        searchFocused = false
    }

    func select(_ id: String) {
        selectedID = id
        searchFocused = false
    }

    /// Moves the selection by `offset` rows without wrapping.
    func moveSelection(by offset: Int) {
        let items = PickerItem.items(listState(now: store.now()))
        guard !items.isEmpty else { return }
        let current = items.firstIndex { $0.id == selectedID } ?? 0
        let next = min(max(current + offset, 0), items.count - 1)
        selectedID = items[next].id
    }

    /// k: moves up, or focuses the search field from the first row.
    func moveUpOrFocusSearch() {
        let items = PickerItem.items(listState(now: store.now()))
        if let first = items.first, effectiveSelection(in: items)?.id != first.id {
            moveSelection(by: -1)
        } else {
            searchFocused = true
        }
    }

    /// Reads the live selection; binding closures outlive the render that made them.
    func currentSelection() -> PickerItem? {
        effectiveSelection(in: PickerItem.items(listState(now: store.now())))
    }

    /// ↵ on a snippet: paste it directly, or open the placeholder form first.
    func useSelectedSnippet(_ mode: PickerPanelController.DeliveryMode) {
        guard case .snippet(let snippet) = currentSelection() else { return }
        switch SnippetPreparation.plan(for: snippet, now: store.now()) {
        case .direct(let prepared):
            pickerPanel?.deliver(prepared, snippetID: snippet.id, mode: mode)
        case .form:
            navigator.push(.placeholderForm(snippetID: snippet.id, mode: mode.formMode))
        }
    }

    /// ⇧⌘↵: paste with each required placeholder's last-used value, skipping the form.
    func pasteSelectedWithLastValues() {
        guard case .snippet(let snippet) = currentSelection() else { return }
        do {
            let prepared = try SnippetPreparation.prepareWithLastValues(
                snippet, history: store.data.placeholderHistory, now: store.now())
            pickerPanel?.deliver(prepared, snippetID: snippet.id, mode: .paste)
        } catch {
            pickerPanel?.showFailure("Missing History", message: error.description)
        }
    }

    var actions: SnippetActions { SnippetActions(store: store, toasts: toasts) }

    func selectedSnippet() -> Snippet? {
        guard case .snippet(let snippet) = currentSelection() else { return nil }
        return snippet
    }

    /// Deletes the confirmed snippet and moves the selection to its neighbour.
    func confirmDelete() {
        guard let snippet = pendingDelete else { return }
        pendingDelete = nil
        let items = PickerItem.items(listState(now: store.now()))
        if let index = items.firstIndex(where: { $0.id == snippet.id }) {
            let neighbour = items.indices.contains(index + 1) ? index + 1 : index - 1
            selectedID = items.indices.contains(neighbour) ? items[neighbour].id : nil
        }
        actions.delete(snippet)
    }

    var manageTagsTitle: String {
        let unused = TagStatistics.unusedTags(
            snippets: store.snippets, tags: SnippetRepository(data: store.data).tags).count
        return unused > 0 ? "Manage Tags (\(unused) unused)" : "Manage Tags"
    }

    func copyTitle(_ snippet: Snippet) {
        do {
            try Pasteboard.copy(snippet.title)
            toasts.success("Copied Title", message: snippet.title)
        } catch {
            toasts.failure("Could Not Copy", message: String(describing: error))
        }
    }

    func lastValuesAvailability(_ selection: PickerItem?) -> SnippetPreparation.LastValuesAvailability {
        guard case .snippet(let snippet) = selection else { return .notApplicable }
        return SnippetPreparation.lastValuesAvailability(
            for: snippet, history: store.data.placeholderHistory, now: store.now())
    }
}
