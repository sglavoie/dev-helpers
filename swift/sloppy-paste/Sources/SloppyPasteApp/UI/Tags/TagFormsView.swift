import SloppyCore
import SwiftUI

/// `Route.renameTag`: renames a tag and its descendants across all snippets.
struct RenameTagView: View {
    let tag: String

    @Environment(SnippetStore.self) private var store
    @Environment(Navigator.self) private var navigator
    @Environment(ToastCenter.self) private var toasts

    @ViewState private var newName: String?
    @ViewState private var error: String?
    @ViewState private var fieldHandle = FocusHandle()

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 10) {
                Text("Rename Tag: \(tag)")
                    .font(.headline)
                    .lineLimit(1)
                Text("New Tag Name")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                EditorTextField(
                    placeholder: "Enter new tag name",
                    text: Binding(
                        get: { newName ?? tag },
                        set: {
                            newName = $0
                            error = nil
                        }),
                    handle: fieldHandle)
                    .fixedSize(horizontal: false, vertical: true)
                if let error {
                    Text(error)
                        .font(.caption)
                        .foregroundStyle(.red)
                }
                Text("Renaming \"\(tag)\" also updates its child tags (\"\(tag)/…\" becomes \"new-name/…\").")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 12)
            Spacer(minLength: 0)
            Divider()
            ManagementFooter(summary: "", primary: "↵ Rename Tag", hints: ["⌘K Actions", "⎋ Back"])
        }
        .onAppear {
            Task { @MainActor in fieldHandle.focus() }
        }
        .keyBindings(
            [KeyBinding(id: "submitRename", title: "Rename Tag", chord: KeyChord(.return)) { submit() }],
            for: .renameTag(tag))
    }

    private func submit() {
        let trimmed = (newName ?? tag).trimmingCharacters(in: .whitespacesAndNewlines)
        let validation = Validation.validateTag(trimmed)
        guard validation.isValid else {
            error = validation.error
            return
        }
        let normalized = validation.normalizedValue ?? trimmed
        guard normalized != tag else {
            error = "New tag name must be different"
            return
        }
        store.reloadIfChanged()
        do {
            let affected = try store.mutate { repository, now in
                repository.renameTag(tag, to: normalized, now: now)
            }
            navigator.pop()
            toasts.success("Tag renamed", message: "Updated \(affected) snippet\(affected == 1 ? "" : "s")")
        } catch {
            toasts.failure("Failed to rename tag", message: error.localizedDescription)
        }
    }
}

/// `Route.mergeTags`: merges the source tag (and its descendants) into a
/// target picked from a filterable list. The source's own descendants are
/// not offered, since merging into one would nest the tag inside itself.
struct MergeTagsView: View {
    let sourceTag: String

    @Environment(SnippetStore.self) private var store
    @Environment(Navigator.self) private var navigator
    @Environment(ToastCenter.self) private var toasts

    @ViewState private var query = ""
    @ViewState private var selectedID: String?
    @ViewState private var searchHandle = FocusHandle()

    var body: some View {
        let targets = targets()
        let selection = ListSelection.effective(targets, selectedID)

        VStack(alignment: .leading, spacing: 0) {
            ManagementHeader(
                title: "Merge \"\(sourceTag)\" into…", placeholder: "Filter target tags…", text: $query,
                handle: searchHandle)
            Text(
                "All snippets with the source tag will use the target tag instead. The source tag will be removed."
            )
            .font(.caption)
            .foregroundStyle(.secondary)
            .padding(.horizontal, 16)
            .padding(.bottom, 8)
            Divider()
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 1) {
                        ForEach(targets, id: \.self) { tag in
                            HStack(spacing: 10) {
                                Image(systemName: "tag")
                                    .sharpWhenZoomed()
                                    .foregroundStyle(.secondary)
                                    .frame(width: 22)
                                Text(tag).lineLimit(1)
                                Spacer(minLength: 0)
                            }
                            .selectableRow(isSelected: tag == selection)
                            .id(tag)
                            .onTapGesture {
                                selectedID = tag
                                searchHandle.focus()
                            }
                        }
                        if targets.isEmpty {
                            EmptyListMessage(
                                title: "No target tags",
                                message: query.isEmpty ? "There is no other tag to merge into" : "Try a different filter")
                        }
                    }
                    .padding(.horizontal, 8)
                    .padding(.vertical, 6)
                }
                .onChange(of: selection) {
                    guard let selection else { return }
                    proxy.scrollTo(selection)
                }
            }
            .frame(maxHeight: .infinity)
            Divider()
            ManagementFooter(
                summary: "Source: \(sourceTag)",
                primary: selection.map { "↵ Merge into \"\($0)\"" },
                hints: ["⌘K Actions", "⎋ Back"])
        }
        .onAppear {
            store.reloadIfChanged()
            Task { @MainActor in searchHandle.focus() }
        }
        .onChange(of: query) { selectedID = nil }
        .keyBindings(bindings(hasSelection: selection != nil), for: .mergeTags(sourceTag))
    }

    private func targets() -> [String] {
        let needle = query.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        return SnippetRepository(data: store.data).tags.filter { tag in
            tag != sourceTag && !TagHierarchy.isChildOf(tag, sourceTag)
                && (needle.isEmpty || tag.lowercased().contains(needle))
        }
    }

    private func bindings(hasSelection: Bool) -> [KeyBinding] {
        ListSelection.arrowBindings(prefix: "mergeTarget") { offset in
            let targets = targets()
            selectedID = ListSelection.moved(targets, from: ListSelection.effective(targets, selectedID), by: offset)
        } + [
            KeyBinding(id: "submitMerge", title: "Merge Tags", chord: KeyChord(.return), isEnabled: hasSelection) {
                merge()
            }
        ]
    }

    private func merge() {
        guard let target = ListSelection.effective(targets(), selectedID) else { return }
        store.reloadIfChanged()
        do {
            let affected = try store.mutate { repository, now in
                try repository.mergeTags(sourceTag, into: target, now: now)
            }
            navigator.pop()
            toasts.success(
                "Tags merged", message: "Merged \"\(sourceTag)\" into \"\(target)\" (\(affected) snippets affected)")
        } catch {
            toasts.failure("Failed to merge tags", message: error.localizedDescription)
        }
    }
}
