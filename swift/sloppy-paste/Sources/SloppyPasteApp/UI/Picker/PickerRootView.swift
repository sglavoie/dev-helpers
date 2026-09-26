import SwiftUI
import SloppyCore

/// The picker's root screen: search field with filter chips, the sectioned
/// snippet list and the ⌘D detail pane.
///
/// The list is a `LazyVStack` with manual selection rather than a `List`, so
/// the search field keeps keyboard focus while ↑/↓ (routed by KeyRouter)
/// move the selection.
struct PickerRootView: View {
    @Environment(SnippetStore.self) private var store
    @Environment(Navigator.self) private var navigator
    @Environment(AccessibilityPermission.self) private var accessibility
    @Environment(ToastCenter.self) private var toasts
    @Environment(\.pickerPanel) private var pickerPanel

    @AppStorage("picker.showingDetail") private var showingDetail = false
    @AppStorage("picker.sortOption") private var sortRaw = SortOption.updatedDesc.rawValue
    @AppStorage("picker.showRecentSection") private var showRecentSection = true

    @ViewState private var query = ""
    @ViewState private var options = SnippetFilterOptions()
    @ViewState private var showingFilters = false
    @ViewState private var selectedID: String?
    /// The snippet awaiting ⌃X delete confirmation.
    @ViewState private var pendingDelete: Snippet?
    @FocusState private var searchFocused: Bool

    private var sort: SortOption { SortOption(rawValue: sortRaw) ?? .updatedDesc }

    var body: some View {
        let now = store.now()
        let state = listState(now: now)
        let items = PickerItem.items(state)
        let selection = effectiveSelection(in: items)
        let historyAvailable = SnippetPreparation.historyAvailability(
            for: state.rows, history: store.data.placeholderHistory, now: now)

        VStack(alignment: .leading, spacing: 0) {
            searchHeader(state)
            Divider()
            HStack(spacing: 0) {
                list(state, selection: selection, historyAvailable: historyAvailable, now: now)
                    .frame(maxWidth: .infinity)
                if showingDetail {
                    Divider()
                    Group {
                        if case .snippet(let snippet) = selection {
                            SnippetDetailView(snippet: snippet)
                        } else {
                            Text("No snippet selected")
                                .foregroundStyle(.secondary)
                                .frame(maxWidth: .infinity, maxHeight: .infinity)
                        }
                    }
                    .frame(width: 330)
                }
            }
            .frame(maxHeight: .infinity)
            Divider()
            footer(state, selection: selection)
        }
        .overlay {
            if let snippet = pendingDelete {
                ConfirmationOverlay(
                    title: "Delete Snippet",
                    message: "Are you sure you want to delete \"\(snippet.title)\"?",
                    confirmTitle: "Delete",
                    confirm: confirmDelete,
                    cancel: { pendingDelete = nil })
            }
        }
        .onAppear { searchFocused = true }
        .onChange(of: query) { selectedID = nil }
        .onChange(of: options) { selectedID = nil }
        .keyBindings(bindings(selection: selection), for: .root)
    }

    // MARK: Header

    private func searchHeader(_ state: PickerListState) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(.secondary)
                TextField("", text: $query, prompt: Text(state.searchPlaceholder))
                    .textFieldStyle(.plain)
                    .font(.title3)
                    .focused($searchFocused)
            }
            let chips = activeChips
            if !chips.isEmpty || showingFilters {
                HStack(spacing: 6) {
                    ForEach(chips) { chip in
                        Button { chip.clear() } label: {
                            HStack(spacing: 3) {
                                Text(chip.title)
                                Image(systemName: "xmark").font(.caption2)
                            }
                            .font(.caption)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 2)
                            .background(Color.accentColor.opacity(0.18), in: Capsule())
                        }
                        .buttonStyle(.plain)
                        .help("Clear filter")
                    }
                    Spacer(minLength: 0)
                    if showingFilters {
                        filterMenus(state)
                    }
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
    }

    /// ⌘P: sort and tag dropdowns.
    private func filterMenus(_ state: PickerListState) -> some View {
        HStack(spacing: 8) {
            Picker("Sort", selection: $sortRaw) {
                ForEach(SortOption.allCases, id: \.rawValue) { Text($0.label).tag($0.rawValue) }
            }
            .fixedSize()
            Picker("Tag", selection: $options.selectedTag) {
                Text("All Tags").tag(String?.none)
                Text("Untagged").tag(String?.some(TagHierarchy.untaggedSentinel))
                Divider()
                ForEach(tagChoices(state), id: \.self) { Text($0).tag(String?.some($0)) }
            }
            .fixedSize()
        }
        .pickerStyle(.menu)
        .controlSize(.small)
    }

    private func tagChoices(_ state: PickerListState) -> [String] {
        var tags = state.list.visibleTags
        if let selected = options.selectedTag, selected != TagHierarchy.untaggedSentinel, !tags.contains(selected) {
            tags.insert(selected, at: 0)
        }
        return tags
    }

    private struct FilterChip: Identifiable {
        var id: String
        var title: String
        var clear: @MainActor () -> Void
    }

    private var activeChips: [FilterChip] {
        var chips: [FilterChip] = []
        if options.showOnlyFavorites {
            chips.append(FilterChip(id: "favorites", title: "★ Bookmarked") { options.showOnlyFavorites = false })
        }
        if options.showArchivedSnippets {
            chips.append(FilterChip(id: "archived", title: "⊟ Archived") { options.showArchivedSnippets = false })
        }
        if options.showNeedsAttention {
            chips.append(FilterChip(id: "attention", title: "⚠ Needs Attention") { options.showNeedsAttention = false })
        }
        if let tag = options.selectedTag {
            let label = tag == TagHierarchy.untaggedSentinel ? "Untagged" : tag
            chips.append(FilterChip(id: "tag", title: "Tag: \(label)") { options.selectedTag = nil })
        }
        if sort != .updatedDesc {
            chips.append(FilterChip(id: "sort", title: "Sort: \(sort.label)") {
                sortRaw = SortOption.updatedDesc.rawValue
            })
        }
        return chips
    }

    // MARK: List

    private func list(
        _ state: PickerListState, selection: PickerItem?, historyAvailable: Set<String>, now: Int64
    ) -> some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 1, pinnedViews: []) {
                    if !state.suggestions.isEmpty {
                        sectionHeader("Search Suggestions", subtitle: nil)
                        ForEach(state.suggestions, id: \.title) { suggestion in
                            suggestionRow(suggestion, isSelected: selection?.id == PickerItem.suggestion(suggestion).id)
                                .id(PickerItem.suggestion(suggestion).id)
                                .onTapGesture { select(PickerItem.suggestion(suggestion).id) }
                        }
                    }
                    ForEach(state.sections) { section in
                        if let title = section.title {
                            sectionHeader(title, subtitle: section.subtitle)
                        }
                        ForEach(section.snippets) { snippet in
                            SnippetRowView(
                                snippet: snippet,
                                isSelected: selection?.id == snippet.id,
                                compact: showingDetail,
                                showsStalenessReason: options.showNeedsAttention,
                                historyAvailable: historyAvailable.contains(snippet.id),
                                now: now)
                                .id(snippet.id)
                                .onTapGesture { select(snippet.id) }
                        }
                    }
                    if let empty = state.emptyState {
                        VStack(spacing: 6) {
                            Text(empty.title).font(.headline)
                            Text(empty.message).foregroundStyle(.secondary)
                        }
                        .frame(maxWidth: .infinity)
                        .padding(.top, 60)
                    }
                }
                .padding(.horizontal, 8)
                .padding(.vertical, 6)
            }
            .onChange(of: selection?.id) {
                guard let id = selection?.id else { return }
                proxy.scrollTo(id)
            }
        }
    }

    private func sectionHeader(_ title: String, subtitle: String?) -> some View {
        HStack(spacing: 6) {
            Text(title).fontWeight(.semibold)
            if let subtitle {
                Text(subtitle)
            }
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 10)
        .padding(.top, 8)
        .padding(.bottom, 2)
    }

    private func suggestionRow(_ suggestion: SearchSuggestion, isSelected: Bool) -> some View {
        HStack(spacing: 10) {
            Image(systemName: "text.magnifyingglass")
                .foregroundStyle(.secondary)
                .frame(width: 22)
            Text(suggestion.title)
            Text(suggestion.subtitle)
                .foregroundStyle(.secondary)
                .lineLimit(1)
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .background(RoundedRectangle(cornerRadius: 6).fill(isSelected ? Color.accentColor.opacity(0.25) : .clear))
        .contentShape(Rectangle())
    }

    // MARK: Footer

    private func footer(_ state: PickerListState, selection: PickerItem?) -> some View {
        HStack(spacing: 14) {
            if !accessibility.isTrusted {
                Button {
                    pickerPanel?.hide()
                    accessibility.openSystemSettings()
                } label: {
                    Label("Copy only: grant Accessibility to paste", systemImage: "exclamationmark.triangle.fill")
                        .foregroundStyle(.orange)
                }
                .buttonStyle(.plain)
                .help("Open System Settings › Privacy & Security › Accessibility")
            } else {
                Text("\(state.rows.count) of \(store.snippets.count) snippets")
            }
            Spacer()
            switch selection {
            case .snippet:
                Text(accessibility.isTrusted ? "↵ Paste" : "↵ Copy").fontWeight(.semibold)
                Text("⌥⌘↵ Copy")
                Text("⌘E Edit")
                if lastValuesAvailability(selection) == .available {
                    Text("⇧⌘↵ Last Values")
                }
            case .suggestion:
                Text("↵ Use Suggestion").fontWeight(.semibold)
            case nil:
                EmptyView()
            }
            Text("⌘D Details")
            Text("⌘P Filters")
            Text("⌘K Actions")
            Text("⎋ Close")
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
    }

    // MARK: State

    private func listState(now: Int64) -> PickerListState {
        PickerListModel.build(
            store.snippets, query: query, options: options, sort: sort, showRecentSection: showRecentSection, now: now)
    }

    /// The selected item, falling back to the first one when the selection
    /// is unset or was filtered away.
    private func effectiveSelection(in items: [PickerItem]) -> PickerItem? {
        items.first { $0.id == selectedID } ?? items.first
    }

    private func select(_ id: String) {
        selectedID = id
        searchFocused = true
    }

    /// Moves the selection by `offset` rows without wrapping.
    private func moveSelection(by offset: Int) {
        let items = PickerItem.items(listState(now: store.now()))
        guard !items.isEmpty else { return }
        let current = items.firstIndex { $0.id == selectedID } ?? 0
        let next = min(max(current + offset, 0), items.count - 1)
        selectedID = items[next].id
    }

    /// Reads the live selection; binding closures outlive the render that made them.
    private func currentSelection() -> PickerItem? {
        effectiveSelection(in: PickerItem.items(listState(now: store.now())))
    }

    /// ↵ on a snippet: paste it directly, or open the placeholder form first.
    private func useSelectedSnippet(_ mode: PickerPanelController.DeliveryMode) {
        guard case .snippet(let snippet) = currentSelection() else { return }
        switch SnippetPreparation.plan(for: snippet, now: store.now()) {
        case .direct(let prepared):
            pickerPanel?.deliver(prepared, snippetID: snippet.id, mode: mode)
        case .form:
            navigator.push(.placeholderForm(snippetID: snippet.id, mode: mode.formMode))
        }
    }

    /// ⇧⌘↵: paste with each required placeholder's last-used value, skipping the form.
    private func pasteSelectedWithLastValues() {
        guard case .snippet(let snippet) = currentSelection() else { return }
        do {
            let prepared = try SnippetPreparation.prepareWithLastValues(
                snippet, history: store.data.placeholderHistory, now: store.now())
            pickerPanel?.deliver(prepared, snippetID: snippet.id, mode: .paste)
        } catch {
            pickerPanel?.showFailure("Missing History", message: error.description)
        }
    }

    private var actions: SnippetActions { SnippetActions(store: store, toasts: toasts) }

    private func selectedSnippet() -> Snippet? {
        guard case .snippet(let snippet) = currentSelection() else { return nil }
        return snippet
    }

    /// Deletes the confirmed snippet and moves the selection to its neighbour.
    private func confirmDelete() {
        guard let snippet = pendingDelete else { return }
        pendingDelete = nil
        let items = PickerItem.items(listState(now: store.now()))
        if let index = items.firstIndex(where: { $0.id == snippet.id }) {
            let neighbour = items.indices.contains(index + 1) ? index + 1 : index - 1
            selectedID = items.indices.contains(neighbour) ? items[neighbour].id : nil
        }
        actions.delete(snippet)
    }

    private var manageTagsTitle: String {
        let unused = TagStatistics.unusedTags(
            snippets: store.snippets, tags: SnippetRepository(data: store.data).tags).count
        return unused > 0 ? "Manage Tags (\(unused) unused)" : "Manage Tags"
    }

    private func copyTitle(_ snippet: Snippet) {
        do {
            try Pasteboard.copy(snippet.title)
            toasts.success("Copied Title", message: snippet.title)
        } catch {
            toasts.failure("Could Not Copy", message: String(describing: error))
        }
    }

    private func lastValuesAvailability(_ selection: PickerItem?) -> SnippetPreparation.LastValuesAvailability {
        guard case .snippet(let snippet) = selection else { return .notApplicable }
        return SnippetPreparation.lastValuesAvailability(
            for: snippet, history: store.data.placeholderHistory, now: store.now())
    }

    private func bindings(selection: PickerItem?) -> [KeyBinding] {
        var isSuggestion = false
        var isSnippet = false
        switch selection {
        case .suggestion: isSuggestion = true
        case .snippet: isSnippet = true
        case nil: break
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

/// A selectable row: a search suggestion or a snippet.
enum PickerItem: Identifiable {
    case suggestion(SearchSuggestion)
    case snippet(Snippet)

    var id: String {
        switch self {
        case .suggestion(let suggestion): "suggestion:\(suggestion.title)"
        case .snippet(let snippet): snippet.id
        }
    }

    /// Suggestions first, then snippets in display order.
    static func items(_ state: PickerListState) -> [PickerItem] {
        state.suggestions.map(PickerItem.suggestion) + state.rows.map(PickerItem.snippet)
    }
}
