import SwiftUI
import SloppyCore

extension PickerRootView {
    func bindings(selection: PickerItem?) -> [KeyBinding] {
        var isSuggestion = false
        var isSnippet = false
        switch selection {
        case .suggestion: isSuggestion = true
        case .snippet: isSnippet = true
        case nil: break
        }
        if let tagChooser {
            return tagChooser.bindings + [
                KeyBinding(id: "closeTagFilter", title: "Back", chord: KeyChord(.escape), showsInMenu: false) {
                    closeTagChooser()
                }
            ]
        }
        if pendingDelete != nil {
            return [
                KeyBinding(id: "confirmDelete", title: "Delete", chord: KeyChord(.return)) { confirmDelete() },
                KeyBinding(id: "cancelDelete", title: "Cancel", chord: KeyChord(.escape)) { pendingDelete = nil },
            ]
        }
        let lastValues = lastValuesAvailability(selection)
        let snippet: Snippet? = if case .snippet(let snippet) = selection { snippet } else { nil }
        // The extension offered this only on stale snippets; archived ones are already done.
        let staleReason = snippet.flatMap { snippet in
            snippet.isArchived ? nil : Staleness.analyze(snippet, now: store.now()).stalenessReason
        }
        return [
            KeyBinding(id: "up", title: "Previous", chord: KeyChord(.upArrow), showsInMenu: false) {
                moveSelection(by: -1)
            },
            KeyBinding(id: "down", title: "Next", chord: KeyChord(.downArrow), showsInMenu: false) {
                moveSelection(by: 1)
            },
            // j/k only while the search field is unfocused, so they still type.
            KeyBinding(
                id: "vimUp", title: "Previous", chord: searchFocused ? nil : KeyChord(.character("k")),
                showsInMenu: false
            ) {
                moveUpOrFocusSearch()
            },
            KeyBinding(
                id: "vimDown", title: "Next", chord: searchFocused ? nil : KeyChord(.character("j")),
                showsInMenu: false
            ) {
                moveSelection(by: 1)
            },
            KeyBinding(id: "focusSearch", title: "Search", chord: KeyChord(.character("f"), .command)) {
                searchFocused = true
            },
            // Esc clears a typed query first, then leaves the search field; a further Esc closes the panel.
            KeyBinding(
                id: "clearQuery", title: "Clear Search", chord: query.isEmpty ? nil : KeyChord(.escape),
                showsInMenu: false
            ) {
                query = ""
            },
            KeyBinding(
                id: "leaveSearch", title: "Leave Search", chord: searchFocused ? KeyChord(.escape) : nil,
                showsInMenu: false
            ) {
                selectedID = currentSelection()?.id
                searchFocused = false
            },
            KeyBinding(
                id: "applySuggestion", title: "Use Suggestion", chord: KeyChord(.return), showsInMenu: false,
                isEnabled: isSuggestion
            ) {
                if case .suggestion(let suggestion) = currentSelection() {
                    query = suggestion.completion
                }
            },
            KeyBinding(id: "paste", title: "Paste", chord: KeyChord(.return), isEnabled: isSnippet) {
                useSelectedSnippet(.paste)
            },
            KeyBinding(
                id: "pasteCommand", title: "Paste", chord: KeyChord(.return, .command), showsInMenu: false,
                isEnabled: isSnippet
            ) {
                useSelectedSnippet(.paste)
            },
            KeyBinding(
                id: "copy", title: "Copy to Clipboard", chord: KeyChord(.return, [.command, .option]),
                isEnabled: isSnippet
            ) {
                useSelectedSnippet(.copy)
            },
            KeyBinding(
                id: "copyAndStay", title: "Copy and Keep Open", chord: KeyChord(.return, [.command, .control]),
                isEnabled: isSnippet
            ) {
                useSelectedSnippet(.copyAndStay)
            },
            KeyBinding(
                id: "pasteLastValues",
                title: lastValues == .available ? "Paste with Last Values" : "Paste with Last Values (no history yet)",
                chord: KeyChord(.return, [.command, .shift]),
                isEnabled: lastValues != .notApplicable
            ) {
                pasteSelectedWithLastValues()
            },
            // ⌘C only while the search field is empty, so it still copies typed text.
            KeyBinding(
                id: "copyTitle", title: "Copy Title",
                chord: query.isEmpty ? KeyChord(.character("c"), .command) : nil, isEnabled: isSnippet
            ) {
                if let snippet = selectedSnippet() { copyTitle(snippet) }
            },
            KeyBinding(id: "new", title: "New Snippet", chord: KeyChord(.character("n"), .command)) {
                navigator.push(.editor(.new))
            },
            KeyBinding(id: "resumeDraft", title: "Resume Draft", isEnabled: navigator.pendingEditor != nil) {
                if let pending = navigator.pendingEditor { navigator.push(.editor(pending.mode)) }
            },
            KeyBinding(
                id: "newFromClipboard", title: "New Snippet from Clipboard",
                chord: KeyChord(.character("n"), [.command, .option])
            ) {
                navigator.push(.editor(.fromClipboard))
            },
            KeyBinding(id: "edit", title: "Edit Snippet", chord: KeyChord(.character("e"), .command), isEnabled: isSnippet)
            {
                if let snippet = selectedSnippet() { navigator.push(.editor(.edit(snippetID: snippet.id))) }
            },
            KeyBinding(
                id: "duplicate", title: "Duplicate Snippet", chord: KeyChord(.character("d"), [.command, .shift]),
                isEnabled: isSnippet
            ) {
                if let snippet = selectedSnippet(), let copy = actions.duplicate(snippet) {
                    selectedID = copy.id
                    navigator.push(.editor(.edit(snippetID: copy.id)))
                }
            },
            KeyBinding(id: "delete", title: "Delete Snippet", chord: KeyChord(.character("x"), .control), isEnabled: isSnippet)
            {
                pendingDelete = selectedSnippet()
            },
            KeyBinding(
                id: "pin", title: snippet?.isPinned == true ? "Unpin Snippet" : "Pin Snippet",
                chord: KeyChord(.character("p"), [.command, .shift]), isEnabled: isSnippet
            ) {
                if let snippet = selectedSnippet() { actions.togglePin(snippet) }
            },
            KeyBinding(
                id: "bookmark", title: snippet?.isFavorite == true ? "Remove Bookmark" : "Add Bookmark",
                chord: KeyChord(.character("v"), [.command, .shift]), isEnabled: isSnippet
            ) {
                if let snippet = selectedSnippet() { actions.toggleBookmark(snippet) }
            },
            KeyBinding(
                id: "archive", title: snippet?.isArchived == true ? "Unarchive Snippet" : "Archive Snippet",
                chord: KeyChord(.character("a"), [.command, .shift]), isEnabled: isSnippet
            ) {
                if let snippet = selectedSnippet() { actions.toggleArchive(snippet) }
            },
            KeyBinding(
                id: "archiveStale", title: "Archive — \(staleReason ?? "Stale Snippet")",
                chord: KeyChord(.character("u"), [.command, .shift]), isEnabled: staleReason != nil
            ) {
                if let snippet = selectedSnippet(), snippet.isArchived == false,
                   Staleness.analyze(snippet, now: store.now()).isStale {
                    actions.toggleArchive(snippet)
                }
            },
            KeyBinding(id: "editTags", title: "Edit Tags", chord: KeyChord(.character("t"), .command), isEnabled: isSnippet)
            {
                if let snippet = selectedSnippet() { navigator.push(.tagPicker(snippetID: snippet.id)) }
            },
            KeyBinding(id: "manageTags", title: manageTagsTitle, chord: KeyChord(.character("t"), [.command, .option])) {
                navigator.push(.manageTags)
            },
            KeyBinding(id: "manageHistory", title: "Manage Placeholder History") {
                navigator.push(.history(key: nil))
            },
            KeyBinding(id: "toggleDetail", title: "Toggle Detail View", chord: KeyChord(.character("d"), .command)) {
                showingDetail.toggle()
            },
            KeyBinding(id: "filters", title: "Sort and Tag Filters", chord: KeyChord(.character("p"), .command)) {
                showingFilters.toggle()
            },
            KeyBinding(id: "filterByTag", title: "Filter by Tag…", chord: KeyChord(.character("g"), .command)) {
                openTagChooser()
            },
            KeyBinding(id: "clearSearchAndFilters", title: "Clear Search and Filters", isEnabled: hasSearchOrFilters) {
                clearSearchAndFilters()
            },
            KeyBinding(
                id: "favorites", title: options.showOnlyFavorites ? "Show All Snippets" : "Show Bookmarked",
                chord: KeyChord(.character("f"), [.command, .shift])
            ) {
                options.showOnlyFavorites.toggle()
            },
            KeyBinding(
                id: "recent", title: showRecentSection ? "Hide Recent Section" : "Show Recent Section",
                chord: KeyChord(.character("r"), .command)
            ) {
                showRecentSection.toggle()
            },
            KeyBinding(
                id: "archived", title: options.showArchivedSnippets ? "Hide Archived Snippets" : "Show Archived Snippets",
                chord: KeyChord(.character("b"), .command)
            ) {
                options.showArchivedSnippets.toggle()
            },
            KeyBinding(
                id: "needsAttention",
                title: options.showNeedsAttention ? "Show All Snippets" : "Show Snippets Needing Attention",
                chord: KeyChord(.character("n"), [.command, .shift])
            ) {
                options.showNeedsAttention.toggle()
            },
            KeyBinding(id: "import", title: "Import Snippets…", chord: KeyChord(.character("i"), [.command, .shift])) {
                pickerPanel?.commands.importData()
            },
            KeyBinding(id: "export", title: "Export All Snippets…", chord: KeyChord(.character("e"), [.command, .shift])) {
                pickerPanel?.commands.exportData()
            },
            KeyBinding(id: "storage", title: "View Storage Info", chord: KeyChord(.character("s"), [.command, .shift])) {
                if let description = pickerPanel?.commands.storageDescription() {
                    toasts.success("Storage Information", message: description)
                }
            },
            KeyBinding(id: "settings", title: "Settings…", chord: KeyChord(.character(","), .command)) {
                pickerPanel?.commands.openSettings()
            },
        ]
    }
}
