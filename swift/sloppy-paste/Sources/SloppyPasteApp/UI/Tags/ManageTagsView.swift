import SloppyCore
import SwiftUI

/// `Route.manageTags`: every tag with its usage figures. Name order shows
/// the hierarchy as an indented tree; the statistic sorts and filtering
/// show full tag paths. ↵/⌘R rename, ⌘M merge, ⌃X delete.
struct ManageTagsView: View {
    @Environment(SnippetStore.self) private var store
    @Environment(Navigator.self) private var navigator
    @Environment(ToastCenter.self) private var toasts

    @AppStorage("manageTags.sortOption") private var sortRaw = TagSortOption.nameAsc.rawValue

    @ViewState private var query = ""
    @ViewState private var selectedID: String?
    /// The tag awaiting ⌃X delete confirmation.
    @ViewState private var pendingDelete: TagStatistics?
    @ViewState private var searchHandle = FocusHandle()

    private var sort: TagSortOption { TagSortOption(rawValue: sortRaw) ?? .nameAsc }

    /// One displayed tag: its path, the label to show and its indent.
    struct Line: Identifiable {
        var tag: String
        var label: String
        var depth: Int
        var hasChildren: Bool
        var stats: TagStatistics

        var id: String { tag }
    }

    var body: some View {
        let tags = SnippetRepository(data: store.data).tags
        let lines = lines(tags: tags)
        let selection = lines.first { $0.id == ListSelection.effective(lines.map(\.id), selectedID) }
        let now = store.now()

        VStack(alignment: .leading, spacing: 0) {
            ManagementHeader(title: "Manage Tags", placeholder: "Search tags…", text: $query, handle: searchHandle) {
                Picker("Sort", selection: $sortRaw) {
                    ForEach(TagSortOption.allCases, id: \.rawValue) { Text($0.label).tag($0.rawValue) }
                }
                .pickerStyle(.menu)
                .controlSize(.small)
                .fixedSize()
            }
            Divider()
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 1) {
                        ForEach(lines) { line in
                            row(line, isSelected: line.id == selection?.id, now: now)
                                .id(line.id)
                                .onTapGesture { select(line.id) }
                        }
                        if tags.isEmpty {
                            EmptyListMessage(
                                title: "No tags yet",
                                message: "Tags will appear here as you create snippets with tags")
                        } else if lines.isEmpty {
                            EmptyListMessage(title: "No matching tags", message: "Try a different search")
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
            .frame(maxHeight: .infinity)
            Divider()
            ManagementFooter(
                summary: "\(tags.count) tag\(tags.count == 1 ? "" : "s")",
                primary: selection == nil ? nil : "↵ Rename",
                hints: selection == nil ? ["⌘K Actions", "⎋ Back"] : ["⌘M Merge", "⌃X Delete", "⌘K Actions", "⎋ Back"])
        }
        .overlay {
            if let stats = pendingDelete {
                ConfirmationOverlay(
                    title: "Delete Tag",
                    message: "Delete \"\(stats.tag)\"? This tag will be removed from all snippets.",
                    confirmTitle: "Delete",
                    confirm: confirmDelete,
                    cancel: { pendingDelete = nil })
            }
        }
        .onAppear {
            store.reloadIfChanged()
            Task { @MainActor in searchHandle.focus() }
        }
        .onChange(of: query) { selectedID = nil }
        .onChange(of: sortRaw) { selectedID = nil }
        .keyBindings(bindings(selection: selection), for: .manageTags)
    }

    private func row(_ line: Line, isSelected: Bool, now: Int64) -> some View {
        let stats = line.stats
        return HStack(spacing: 10) {
            Image(systemName: line.hasChildren ? "folder" : "tag")
                .sharpWhenZoomed()
                .foregroundStyle(.secondary)
                .frame(width: 22)
            Text(line.label)
                .lineLimit(1)
                .padding(.leading, CGFloat(line.depth) * 18)
            if line.depth > 0 {
                Text(line.tag)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
            Spacer(minLength: 0)
            CountChip(
                text: "\(stats.snippetCount) snippet\(stats.snippetCount == 1 ? "" : "s")",
                tint: stats.snippetCount > 0 ? .primary : .secondary)
            if stats.totalUsageCount > 0 {
                CountChip(text: "\(Formatting.number(stats.totalUsageCount)) uses", tint: .green)
            }
            if let lastUsed = stats.lastUsedAt {
                Text(Formatting.relativeTime(lastUsed, now: now))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .help("Last used: \(Formatting.absoluteDate(lastUsed))")
            } else if stats.snippetCount > 0 {
                CountChip(text: "Never used", tint: .orange)
            }
        }
        .selectableRow(isSelected: isSelected)
    }

    // MARK: State

    /// The tree in name order without a query; otherwise a flat, sorted list
    /// of tags whose path contains the query.
    private func lines(tags: [String]) -> [Line] {
        let stats = TagStatistics.sorted(TagStatistics.compute(snippets: store.snippets, tags: tags), by: sort)
        let statsByTag = Dictionary(stats.map { ($0.tag, $0) }, uniquingKeysWith: { first, _ in first })
        let empty = { (tag: String) in TagStatistics(tag: tag, snippetCount: 0, lastUsedAt: nil, totalUsageCount: 0) }
        let needle = query.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()

        if sort == .nameAsc && needle.isEmpty {
            return TagHierarchy.flattenTagTree(TagHierarchy.buildTagTree(tags)).map { node in
                Line(
                    tag: node.tag, label: node.name, depth: node.depth, hasChildren: node.hasChildren,
                    stats: statsByTag[node.tag] ?? empty(node.tag))
            }
        }
        return stats
            .filter { needle.isEmpty || $0.tag.lowercased().contains(needle) }
            .map { Line(tag: $0.tag, label: $0.tag, depth: 0, hasChildren: false, stats: $0) }
    }

    private func currentLines() -> [Line] {
        lines(tags: SnippetRepository(data: store.data).tags)
    }

    /// Reads the live selection; binding closures outlive the render that made them.
    private func currentSelection() -> Line? {
        let lines = currentLines()
        let id = ListSelection.effective(lines.map(\.id), selectedID)
        return lines.first { $0.id == id }
    }

    private func select(_ id: String) {
        selectedID = id
        searchHandle.focus()
    }

    private func confirmDelete() {
        guard let stats = pendingDelete else { return }
        pendingDelete = nil
        selectedID = ListSelection.neighbour(of: stats.tag, in: currentLines().map(\.id))
        store.reloadIfChanged()
        do {
            try store.mutate { repository, now in repository.deleteTag(stats.tag, now: now) }
            toasts.success("Tag deleted")
        } catch {
            toasts.failure("Failed to delete tag", message: error.localizedDescription)
        }
    }

    private func bindings(selection: Line?) -> [KeyBinding] {
        if pendingDelete != nil {
            return [
                KeyBinding(id: "confirmDeleteTag", title: "Delete", chord: KeyChord(.return)) { confirmDelete() },
                KeyBinding(id: "cancelDeleteTag", title: "Cancel", chord: KeyChord(.escape)) { pendingDelete = nil },
            ]
        }
        let hasSelection = selection != nil
        let snippetCount = selection?.stats.snippetCount ?? 0
        let rename = { @MainActor in
            if let line = currentSelection() { navigator.push(.renameTag(line.tag)) }
        }
        return ListSelection.arrowBindings(prefix: "tag") { offset in
            selectedID = ListSelection.moved(currentLines().map(\.id), from: currentSelection()?.id, by: offset)
        } + [
            KeyBinding(id: "renameTag", title: "Rename Tag", chord: KeyChord(.return), isEnabled: hasSelection) {
                rename()
            },
            KeyBinding(
                id: "renameTagCommand", title: "Rename Tag", chord: KeyChord(.character("r"), .command),
                showsInMenu: false, isEnabled: hasSelection
            ) {
                rename()
            },
            KeyBinding(
                id: "mergeTag", title: "Merge into Another Tag…", chord: KeyChord(.character("m"), .command),
                isEnabled: hasSelection
            ) {
                if let line = currentSelection() { navigator.push(.mergeTags(line.tag)) }
            },
            KeyBinding(
                id: "deleteTag", title: snippetCount > 0 ? "Delete Tag (\(snippetCount) snippets)" : "Delete Tag",
                chord: KeyChord(.character("x"), .control), isEnabled: hasSelection
            ) {
                pendingDelete = currentSelection()?.stats
            },
            KeyBinding(id: "tagSort", title: "Next Sort Order", chord: KeyChord(.character("p"), .command)) {
                let all = TagSortOption.allCases
                let index = all.firstIndex(of: sort) ?? 0
                sortRaw = all[(index + 1) % all.count].rawValue
                toasts.success("Sort: \(TagSortOption(rawValue: sortRaw)?.label ?? "")")
            },
        ]
    }
}
