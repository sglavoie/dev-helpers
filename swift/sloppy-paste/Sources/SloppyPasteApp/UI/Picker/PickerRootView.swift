import SwiftUI
import SloppyCore

/// The picker's root screen: search field with filter chips, the sectioned
/// snippet list and the ⌘D detail pane.
///
/// The list is a `LazyVStack` with manual selection rather than a `List`;
/// KeyRouter moves the selection with ↑/↓ whether or not the search field
/// has focus. The search field starts unfocused so j/k move the selection
/// too; ⌘F, or k on the first row, focuses it and Esc hands keys back to
/// the list.
struct PickerRootView: View {
    @Environment(SnippetStore.self) var store
    @Environment(Navigator.self) var navigator
    @Environment(AccessibilityPermission.self) var accessibility
    @Environment(ToastCenter.self) var toasts
    @Environment(\.pickerPanel) var pickerPanel

    @AppStorage("picker.showingDetail") var showingDetail = false
    @AppStorage("picker.sortOption") var sortRaw = SortOption.updatedDesc.rawValue
    @AppStorage("picker.showRecentSection") var showRecentSection = true
    @AppStorage(DetailPaneWidth.defaultsKey) var storedDetailWidth = Double(DetailPaneWidth.defaultWidth)

    @ViewState var query = ""
    @ViewState var options = SnippetFilterOptions()
    @ViewState var showingFilters = false
    @ViewState var selectedID: String?
    /// The snippet awaiting ⌃X delete confirmation.
    @ViewState var pendingDelete: Snippet?
    /// The ⌘G tag filter chooser, while open.
    @ViewState var tagChooser: TagFilterChooserModel?
    @ViewState var tagChooserSearchHandle = FocusHandle()
    @FocusState var searchFocused: Bool
    /// The width of the list and detail pane row, to keep the list visible.
    @ViewState var paneRowWidth: CGFloat = 0
    /// The detail pane's width while its divider is dragged; saved on release.
    @ViewState var draggedDetailWidth: CGFloat?
    @ViewState var detailDragStartWidth: CGFloat = 0

    var sort: SortOption { SortOption(rawValue: sortRaw) ?? .updatedDesc }

    var body: some View {
        let now = store.now()
        let state = listState(now: now)
        let items = PickerItem.items(state)
        let selection = effectiveSelection(in: items)
        let historyAvailable = SnippetPreparation.historyAvailability(
            for: state.rows, history: store.data.placeholderHistory, now: now)

        VStack(alignment: .leading, spacing: 0) {
            searchHeader(state)
            if let pending = navigator.pendingEditor {
                HStack {
                    Text(pending.draft.title.isEmpty ? "Unfinished snippet" : pending.draft.title)
                        .lineLimit(1)
                        .foregroundStyle(.secondary)
                    Spacer()
                    Button("Resume Draft") { navigator.push(.editor(pending.mode)) }
                }
                .font(.caption)
                .padding(.horizontal, 16)
                .padding(.bottom, 8)
            }
            Divider()
            HStack(spacing: 0) {
                list(state, selection: selection, historyAvailable: historyAvailable, now: now)
                    .frame(maxWidth: .infinity)
                if showingDetail {
                    PaneDivider(
                        onBegin: { detailDragStartWidth = detailWidth },
                        onDrag: { dx in
                            draggedDetailWidth = DetailPaneWidth.dragged(
                                from: detailDragStartWidth, by: dx, in: paneRowWidth)
                        },
                        onEnd: {
                            if let draggedDetailWidth {
                                storedDetailWidth = Double(draggedDetailWidth)
                            }
                            draggedDetailWidth = nil
                        },
                        onReset: {
                            draggedDetailWidth = nil
                            storedDetailWidth = Double(DetailPaneWidth.defaultWidth)
                        })
                    Group {
                        if case .snippet(let snippet) = selection {
                            SnippetDetailView(snippet: snippet)
                        } else {
                            Text("No snippet selected")
                                .foregroundStyle(.secondary)
                                .frame(maxWidth: .infinity, maxHeight: .infinity)
                        }
                    }
                    .frame(width: detailWidth)
                }
            }
            .frame(maxHeight: .infinity)
            .onGeometryChange(for: CGFloat.self) { $0.size.width } action: { paneRowWidth = $0 }
            Divider()
            footer(state, selection: selection)
        }
        .overlay {
            if let tagChooser {
                TagFilterChooserView(model: tagChooser, searchHandle: tagChooserSearchHandle)
                    .background(Color(nsColor: .windowBackgroundColor))
            }
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
        .onAppear { searchFocused = false }
        .onChange(of: query) { selectedID = nil }
        .onChange(of: options) { selectedID = nil }
        .keyBindings(bindings(selection: selection), for: .root)
    }

    /// The detail pane's width as shown: the stored width (or the one being
    /// dragged), narrowed when the picker is too small to fit it beside the list.
    var detailWidth: CGFloat {
        let width = draggedDetailWidth ?? DetailPaneWidth.sanitized(storedDetailWidth)
        return paneRowWidth > 0 ? DetailPaneWidth.displayed(width, in: paneRowWidth) : width
    }

    // MARK: Header

    func searchHeader(_ state: PickerListState) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Image(systemName: "magnifyingglass")
                    .sharpWhenZoomed()
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
                                Image(systemName: "xmark").font(.caption2).sharpWhenZoomed()
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
    func filterMenus(_ state: PickerListState) -> some View {
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

    func tagChoices(_ state: PickerListState) -> [String] {
        var tags = state.list.visibleTags
        if let selected = options.selectedTag, selected != TagHierarchy.untaggedSentinel, !tags.contains(selected) {
            tags.insert(selected, at: 0)
        }
        return tags
    }

    struct FilterChip: Identifiable {
        var id: String
        var title: String
        var clear: @MainActor () -> Void
    }

    var activeChips: [FilterChip] {
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

    func list(
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
                        PickerSnippetRows(
                            snippets: section.snippets,
                            selectedID: selection?.id,
                            compact: showingDetail,
                            showsStalenessReason: options.showNeedsAttention,
                            historyAvailable: historyAvailable,
                            now: now,
                            select: select)
                    }
                    if let empty = state.emptyState {
                        VStack(spacing: 6) {
                            Text(empty.title).font(.headline)
                            Text(empty.message).foregroundStyle(.secondary)
                            if hasSearchOrFilters {
                                Button("Clear Search and Filters", action: clearSearchAndFilters)
                            }
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

    func sectionHeader(_ title: String, subtitle: String?) -> some View {
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

    func suggestionRow(_ suggestion: SearchSuggestion, isSelected: Bool) -> some View {
        HStack(spacing: 10) {
            Image(systemName: "text.magnifyingglass")
                .sharpWhenZoomed()
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

    func footer(_ state: PickerListState, selection: PickerItem?) -> some View {
        HStack(spacing: 14) {
            if !accessibility.isTrusted {
                Button {
                    pickerPanel?.hide()
                    accessibility.openSystemSettings()
                } label: {
                    Label("Copy only: grant Accessibility to paste", systemImage: "exclamationmark.triangle.fill")
                        .sharpWhenZoomed()
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
            Text("⌘G Tag")
            Text("⌘P Filters")
            Text("⌘K Actions")
            if searchFocused {
                Text("⎋ Back to List")
            } else {
                Text("j/k Navigate")
                Text("⌘F Search")
            }
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
    }
}

/// Keep selection among the section's view inputs so a retained section
/// refreshes its highlights even when filtering only changes other sections.
private struct PickerSnippetRows: View {
    var snippets: [Snippet]
    var selectedID: String?
    var compact: Bool
    var showsStalenessReason: Bool
    var historyAvailable: Set<String>
    var now: Int64
    var select: (String) -> Void

    var body: some View {
        ForEach(snippets) { snippet in
            SnippetRowView(
                snippet: snippet,
                isSelected: selectedID == snippet.id,
                compact: compact,
                showsStalenessReason: showsStalenessReason,
                historyAvailable: historyAvailable.contains(snippet.id),
                now: now)
                .id(snippet.id)
                .onTapGesture { select(snippet.id) }
        }
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
