import Observation
import SloppyCore
import SwiftUI

/// State behind the root screen's ⌘G tag filter chooser: the filter text,
/// the keyboard selection and the entries they produce. The root owns it
/// and publishes `bindings` plus its own Esc.
@MainActor
@Observable
final class TagFilterChooserModel {
    let currentTag: String?
    private(set) var searchText = ""
    private(set) var selectedID: String?

    @ObservationIgnored private let snippets: [Snippet]
    @ObservationIgnored private let showArchived: Bool
    /// Applies the chosen `selectedTag`; nil clears the tag filter.
    @ObservationIgnored private let apply: @MainActor (String?) -> Void

    init(
        snippets: [Snippet], showArchived: Bool, currentTag: String?,
        apply: @escaping @MainActor (String?) -> Void
    ) {
        self.snippets = snippets
        self.showArchived = showArchived
        self.currentTag = currentTag
        self.apply = apply
        // Start on the active filter, so ↵ straight away keeps it.
        selectedID = entries.first { $0.selectedTag == currentTag }?.id
    }

    var entries: [TagFilterChooser.Entry] {
        TagFilterChooser.entries(
            snippets: snippets, showArchived: showArchived, selectedTag: currentTag, searchText: searchText)
    }

    /// The selected entry, falling back to the first one.
    var selection: TagFilterChooser.Entry? {
        let entries = entries
        return entries.first { $0.id == selectedID } ?? entries.first
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
        let entries = entries
        guard !entries.isEmpty else { return }
        let current = entries.firstIndex { $0.id == selection?.id } ?? 0
        selectedID = entries[min(max(current + offset, 0), entries.count - 1)].id
    }

    /// ↵ on the selection: applies it as the tag filter.
    func activate() {
        guard let selection else { return }
        apply(selection.selectedTag)
    }

    var bindings: [KeyBinding] {
        [
            KeyBinding(id: "tagFilterUp", title: "Previous", chord: KeyChord(.upArrow), showsInMenu: false) {
                [weak self] in
                self?.moveSelection(by: -1)
            },
            KeyBinding(id: "tagFilterDown", title: "Next", chord: KeyChord(.downArrow), showsInMenu: false) {
                [weak self] in
                self?.moveSelection(by: 1)
            },
            KeyBinding(
                id: "applyTagFilter", title: "Apply Tag Filter", chord: KeyChord(.return), isEnabled: selection != nil
            ) { [weak self] in
                self?.activate()
            },
        ]
    }
}

/// ⌘G on the root screen: a filterable list of tags with snippet counts.
/// ↵ applies the selection as the tag filter chip. The host handles Esc.
struct TagFilterChooserView: View {
    let model: TagFilterChooserModel
    let searchHandle: FocusHandle

    var body: some View {
        let selection = model.selection
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 8) {
                Text("Filter by Tag")
                    .font(.headline)
                EditorTextField(
                    placeholder: "Filter tags…",
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

    private func list(selection: TagFilterChooser.Entry?) -> some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 1) {
                    ForEach(model.entries) { entry in
                        line(entry, isSelected: selection?.id == entry.id)
                    }
                    if model.entries.isEmpty {
                        Text("No matching tags")
                            .font(.headline)
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

    private func line(_ entry: TagFilterChooser.Entry, isSelected: Bool) -> some View {
        let (symbol, title, count): (String, String, Int?) =
            switch entry {
            case .allTags: ("tray.full", "All Tags", nil)
            case .untagged(let count): ("tag.slash", "Untagged", count)
            case .tag(let tag, let count): ("tag", tag, count)
            }
        let isActive = entry.selectedTag == model.currentTag
        return HStack(spacing: 10) {
            Image(systemName: symbol)
                .sharpWhenZoomed()
                .foregroundStyle(.secondary)
                .frame(width: 22)
            Text(title)
                .lineLimit(1)
            if isActive {
                Image(systemName: "checkmark")
                    .sharpWhenZoomed()
                    .foregroundStyle(.green)
            }
            Spacer(minLength: 0)
            if let count {
                Text("\(count)")
                    .font(.caption)
                    .monospacedDigit()
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .background(RoundedRectangle(cornerRadius: 6).fill(isSelected ? Color.accentColor.opacity(0.25) : .clear))
        .contentShape(Rectangle())
        .id(entry.id)
        .onTapGesture {
            model.select(entry.id)
            model.activate()
        }
    }

    private func footer(selection: TagFilterChooser.Entry?) -> some View {
        HStack(spacing: 14) {
            Spacer()
            if selection != nil {
                Text("↵ Apply").fontWeight(.semibold)
            }
            Text("⎋ Back")
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
    }
}
