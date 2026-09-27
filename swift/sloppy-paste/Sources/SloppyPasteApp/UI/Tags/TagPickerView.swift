import Observation
import SloppyCore
import SwiftUI

/// State behind the tag picker: the working tag set, the filter text and
/// the keyboard selection. Hosts own it, so the editor can show the picker
/// as an overlay without losing its draft, and publish `bindings` as their
/// own catalog.
@MainActor
@Observable
final class TagPickerModel {
    /// A selectable line: the "Create and add" candidate, an invalid name,
    /// or a tag row.
    enum Item: Identifiable {
        case create(String)
        case invalid(text: String, error: String)
        case row(TagPicker.Row)

        var id: String {
            switch self {
            case .create(let tag): "create:\(tag)"
            case .invalid: "invalid"
            case .row(let row): "tag:\(row.tag)"
            }
        }
    }

    let title: String
    let knownTags: [String]
    private(set) var selectedTags: [String]
    private(set) var searchText = ""
    private(set) var selectedID: String?

    @ObservationIgnored private let toasts: ToastCenter
    /// Applies a new tag set; a thrown error rolls the picker back.
    @ObservationIgnored private let commit: @MainActor ([String]) throws -> Void

    init(
        title: String, initialTags: [String], knownTags: [String], toasts: ToastCenter,
        commit: @escaping @MainActor ([String]) throws -> Void
    ) {
        self.title = title
        // Seeded once, like the extension: later store changes do not reset it.
        selectedTags = TagNormalization.removeRedundantParents(initialTags)
        self.knownTags = knownTags
        self.toasts = toasts
        self.commit = commit
    }

    var rows: [TagPicker.Row] {
        TagPicker.buildRows(knownTags: knownTags, selectedTags: selectedTags)
    }

    var visibleRows: [TagPicker.Row] {
        TagPicker.filterRows(rows, searchText: searchText)
    }

    var createCandidate: TagPicker.CreateCandidate? {
        TagPicker.createCandidate(searchText: searchText, knownTags: rows.map(\.tag))
    }

    /// Rows stored on or implied for the snippet.
    var onSnippetRows: [TagPicker.Row] { visibleRows.filter { $0.state != .unselected } }
    var availableRows: [TagPicker.Row] { visibleRows.filter { $0.state == .unselected } }

    /// Display order: the create line, then On This Snippet, then All Tags.
    var items: [Item] {
        var items: [Item] = []
        switch createCandidate {
        case .tag(let tag): items.append(.create(tag))
        case .error(let error):
            items.append(.invalid(text: searchText.trimmingCharacters(in: .whitespacesAndNewlines), error: error))
        case nil: break
        }
        return items + onSnippetRows.map(Item.row) + availableRows.map(Item.row)
    }

    /// The selected item, falling back to the first one.
    var selection: Item? {
        let items = items
        return items.first { $0.id == selectedID } ?? items.first
    }

    func setSearchText(_ text: String) {
        guard text != searchText else { return }
        searchText = text
        selectedID = nil
    }

    func select(_ id: String) {
        selectedID = id
    }

    /// Moves the selection by `offset` without wrapping.
    func moveSelection(by offset: Int) {
        let items = items
        guard !items.isEmpty else { return }
        let current = items.firstIndex { $0.id == selection?.id } ?? 0
        selectedID = items[min(max(current + offset, 0), items.count - 1)].id
    }

    /// ↵ on the selection: toggle a row, create a tag, or explain an invalid name.
    func activate() {
        switch selection {
        case .create(let tag): toggle(tag)
        case .row(let row): toggle(row.tag)
        case .invalid(_, let error): toasts.failure("Invalid tag name", message: error)
        case nil: break
        }
    }

    /// Adds or removes `tag` and commits the result; implied tags cannot be
    /// toggled on their own.
    func toggle(_ tag: String) {
        let result = TagPicker.toggle(selectedTags: selectedTags, tag: tag)
        guard result.changed else {
            toasts.failure(
                "Already implied", message: "\"\(tag)\" is already implied by \"\(result.impliedBy ?? "")\"")
            return
        }
        let previous = selectedTags
        let wasAdded = result.tags.contains(TagNormalization.normalizeTag(tag))
        selectedTags = result.tags
        searchText = ""
        selectedID = "tag:\(TagNormalization.normalizeTag(tag))"
        do {
            try commit(result.tags)
            toasts.success(wasAdded ? "Tag added" : "Tag removed", message: tag)
        } catch {
            selectedTags = previous
            toasts.failure(
                wasAdded ? "Failed to add tag" : "Failed to remove tag", message: String(describing: error))
        }
    }

    /// Title of the ↵ action for the current selection.
    var primaryTitle: String? {
        switch selection {
        case .create: "Create and Add Tag"
        case .row(let row): row.state == .selected ? "Remove Tag" : "Add Tag"
        case .invalid: "Show Tag Name Error"
        case nil: nil
        }
    }

    var bindings: [KeyBinding] {
        [
            KeyBinding(id: "tagUp", title: "Previous", chord: KeyChord(.upArrow), showsInMenu: false) { [weak self] in
                self?.moveSelection(by: -1)
            },
            KeyBinding(id: "tagDown", title: "Next", chord: KeyChord(.downArrow), showsInMenu: false) { [weak self] in
                self?.moveSelection(by: 1)
            },
            KeyBinding(
                id: "toggleTag", title: primaryTitle ?? "Add Tag", chord: KeyChord(.return),
                isEnabled: selection != nil
            ) { [weak self] in
                self?.activate()
            },
        ]
    }
}

/// The unified tag picker: a filter field that doubles as the new-tag
/// name, the create line, and the On This Snippet / All Tags sections.
/// The host publishes the model's bindings and handles Esc.
struct TagPickerView: View {
    let model: TagPickerModel
    let searchHandle: FocusHandle
    /// Footer label for Esc ("Back", "Done").
    var escapeTitle = "Back"

    var body: some View {
        let selection = model.selection
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 8) {
                Text(model.title)
                    .font(.headline)
                    .lineLimit(1)
                EditorTextField(
                    placeholder: "Filter tags, or type a new tag name…",
                    text: Binding(get: { model.searchText }, set: { model.setSearchText($0) }),
                    handle: searchHandle)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 12)
            Divider()
            list(selection: selection)
                .frame(maxHeight: .infinity)
            Divider()
            footer(selection: selection)
        }
        .onAppear {
            Task { @MainActor in searchHandle.focus() }
        }
    }

    private func list(selection: TagPickerModel.Item?) -> some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 1) {
                    switch model.createCandidate {
                    case .tag(let tag):
                        sectionHeader("Create")
                        line(
                            .create(tag), selection: selection, symbol: "plus.circle.fill", tint: .green,
                            title: "Create and add \"\(tag)\"")
                    case .error(let error):
                        sectionHeader("Create")
                        line(
                            .invalid(text: model.searchText, error: error), selection: selection,
                            symbol: "exclamationmark.circle.fill", tint: .red,
                            title: model.searchText.trimmingCharacters(in: .whitespacesAndNewlines), subtitle: error)
                    case nil:
                        EmptyView()
                    }
                    if !model.onSnippetRows.isEmpty {
                        sectionHeader("On This Snippet")
                        ForEach(model.onSnippetRows, id: \.tag) { row(for: $0, selection: selection) }
                    }
                    if !model.availableRows.isEmpty {
                        sectionHeader("All Tags")
                        ForEach(model.availableRows, id: \.tag) { row(for: $0, selection: selection) }
                    }
                    if model.visibleRows.isEmpty && model.createCandidate == nil {
                        VStack(spacing: 6) {
                            Text(model.rows.isEmpty ? "No tags yet" : "No matching tags").font(.headline)
                            Text("Type a tag name to create it").foregroundStyle(.secondary)
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

    private func row(for row: TagPicker.Row, selection: TagPickerModel.Item?) -> some View {
        let symbol = row.state == .unselected ? "circle" : "checkmark.circle.fill"
        let tint: Color =
            switch row.state {
            case .selected: .green
            case .implied, .unselected: .secondary
            }
        return line(
            .row(row), selection: selection, symbol: symbol, tint: tint, title: row.tag,
            badge: row.state == .implied ? "implied" : nil,
            help: row.impliedBy.map { "Implied by \"\($0)\"" })
    }

    private func line(
        _ item: TagPickerModel.Item, selection: TagPickerModel.Item?, symbol: String, tint: Color, title: String,
        subtitle: String? = nil, badge: String? = nil, help: String? = nil
    ) -> some View {
        HStack(spacing: 10) {
            Image(systemName: symbol)
                .sharpWhenZoomed()
                .foregroundStyle(tint)
                .frame(width: 22)
            Text(title)
                .lineLimit(1)
            if let subtitle {
                Text(subtitle)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
            Spacer(minLength: 0)
            if let badge {
                Text(badge)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 1)
                    .background(Color.secondary.opacity(0.15), in: Capsule())
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .background(
            RoundedRectangle(cornerRadius: 6).fill(selection?.id == item.id ? Color.accentColor.opacity(0.25) : .clear)
        )
        .contentShape(Rectangle())
        .help(help ?? "")
        .id(item.id)
        .onTapGesture {
            model.select(item.id)
            model.activate()
        }
    }

    private func sectionHeader(_ title: String) -> some View {
        Text(title)
            .font(.caption)
            .fontWeight(.semibold)
            .foregroundStyle(.secondary)
            .padding(.horizontal, 10)
            .padding(.top, 8)
            .padding(.bottom, 2)
    }

    private func footer(selection: TagPickerModel.Item?) -> some View {
        HStack(spacing: 14) {
            Text("\(model.selectedTags.count) tag\(model.selectedTags.count == 1 ? "" : "s") on this snippet")
            Spacer()
            if let title = model.primaryTitle {
                Text("↵ \(title)").fontWeight(.semibold)
            }
            Text("⌘K Actions")
            Text("⎋ \(escapeTitle)")
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
    }
}

/// `Route.tagPicker`: edits a saved snippet's tags, saving each toggle.
struct TagPickerScreen: View {
    let snippetID: String

    @Environment(SnippetStore.self) private var store
    @Environment(ToastCenter.self) private var toasts

    @ViewState private var model: TagPickerModel?
    @ViewState private var missingSnippet = false
    @ViewState private var searchHandle = FocusHandle()

    var body: some View {
        Group {
            if let model {
                TagPickerView(model: model, searchHandle: searchHandle)
            } else if missingSnippet {
                VStack(spacing: 6) {
                    Text("Snippet not found").font(.headline)
                    Text("It may have been deleted. Press Esc to go back.").foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                Color.clear
            }
        }
        .onAppear(perform: start)
        .keyBindings(model?.bindings ?? [], for: .tagPicker(snippetID: snippetID))
    }

    private func start() {
        guard model == nil, !missingSnippet else { return }
        store.reloadIfChanged()
        guard let snippet = store.snippets.first(where: { $0.id == snippetID }) else {
            missingSnippet = true
            return
        }
        let id = snippetID
        model = TagPickerModel(
            title: "Tags for \"\(snippet.title)\"", initialTags: snippet.tags,
            knownTags: SnippetRepository(data: store.data).tags, toasts: toasts
        ) { [store] tags in
            store.reloadIfChanged()
            try store.mutate { repository, now in
                try repository.updateSnippet(id: id, now: now) { $0.tags = tags }
            }
        }
    }
}
