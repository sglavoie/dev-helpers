import SloppyCore
import SwiftUI

extension PlaceholderKeySortOption {
    var label: String {
        switch self {
        case .nameAsc: "Name (A-Z)"
        case .valueCountDesc: "Most Values"
        case .usageDesc: "Most Used"
        case .lastUsedDesc: "Recently Used"
        }
    }
}

/// `Route.history(key: nil)`: every placeholder key with stored values.
/// ↵ opens the key's values; ⌃⇧X clears them.
struct ManagePlaceholderHistoryView: View {
    @Environment(SnippetStore.self) private var store
    @Environment(Navigator.self) private var navigator
    @Environment(ToastCenter.self) private var toasts

    @AppStorage("history.sortOption") private var sortRaw = PlaceholderKeySortOption.nameAsc.rawValue

    @ViewState private var query = ""
    @ViewState private var selectedID: String?
    /// The key awaiting clear confirmation.
    @ViewState private var pendingClear: PlaceholderKeyStats?
    @ViewState private var searchHandle = FocusHandle()

    private var sort: PlaceholderKeySortOption { PlaceholderKeySortOption(rawValue: sortRaw) ?? .nameAsc }

    var body: some View {
        let stats = stats()
        let selection = stats.first { $0.key == ListSelection.effective(stats.map(\.key), selectedID) }
        let now = store.now()

        VStack(alignment: .leading, spacing: 0) {
            ManagementHeader(
                title: "Manage Placeholder History", placeholder: "Search placeholder keys…", text: $query,
                handle: searchHandle
            ) {
                Picker("Sort", selection: $sortRaw) {
                    ForEach(PlaceholderKeySortOption.allCases, id: \.rawValue) { Text($0.label).tag($0.rawValue) }
                }
                .pickerStyle(.menu)
                .controlSize(.small)
                .fixedSize()
            }
            Divider()
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 1) {
                        ForEach(stats, id: \.key) { stat in
                            row(stat, isSelected: stat.key == selection?.key, now: now)
                                .id(stat.key)
                                .onTapGesture {
                                    selectedID = stat.key
                                    searchHandle.focus()
                                }
                        }
                        if store.data.placeholderKeys.isEmpty {
                            EmptyListMessage(
                                title: "No placeholder history",
                                message: "Placeholder values will be saved here as you use them in snippets")
                        } else if stats.isEmpty {
                            EmptyListMessage(title: "No matching keys", message: "Try a different search")
                        }
                    }
                    .padding(.horizontal, 8)
                    .padding(.vertical, 6)
                }
                .onChange(of: selection?.key) {
                    guard let key = selection?.key else { return }
                    proxy.scrollTo(key)
                }
            }
            .frame(maxHeight: .infinity)
            Divider()
            ManagementFooter(
                summary: "\(store.data.placeholderKeys.count) keys",
                primary: selection == nil ? nil : "↵ View Values",
                hints: selection == nil ? ["⌘K Actions", "⎋ Back"] : ["⌃⇧X Clear", "⌘K Actions", "⎋ Back"])
        }
        .overlay {
            if let stat = pendingClear {
                ConfirmationOverlay(
                    title: "Clear Placeholder History",
                    message: "Clear all \(valuesLabel(stat.valueCount)) for \"\(stat.key)\"?",
                    confirmTitle: "Clear",
                    confirm: confirmClear,
                    cancel: { pendingClear = nil })
            }
        }
        .onAppear {
            store.reloadIfChanged()
            Task { @MainActor in searchHandle.focus() }
        }
        .onChange(of: query) { selectedID = nil }
        .onChange(of: sortRaw) { selectedID = nil }
        .keyBindings(bindings(selection: selection), for: .history(key: nil))
    }

    private func row(_ stat: PlaceholderKeyStats, isSelected: Bool, now: Int64) -> some View {
        HStack(spacing: 10) {
            Image(systemName: "text.cursor")
                .foregroundStyle(.secondary)
                .frame(width: 22)
            Text(stat.key).lineLimit(1)
            Spacer(minLength: 0)
            // The per-key storage limit, not the display preference.
            CountChip(text: "\(stat.valueCount)/\(StorageConstants.maxStoredValuesPerKey) values", tint: .primary)
            if stat.totalUseCount > 0 {
                CountChip(text: "\(stat.totalUseCount) uses", tint: .green)
            }
            if let lastUsed = stat.lastUsed {
                Text(Formatting.relativeTime(lastUsed, now: now))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .help("Last used: \(Formatting.absoluteDate(lastUsed))")
            } else {
                CountChip(text: "Never used")
            }
        }
        .selectableRow(isSelected: isSelected)
    }

    private func stats() -> [PlaceholderKeyStats] {
        let needle = query.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        let data = store.data
        let stats = data.placeholderKeys
            .filter { needle.isEmpty || $0.lowercased().contains(needle) }
            .map { PlaceholderKeyStats(key: $0, values: data.placeholderHistory(forKey: $0)) }
        return PlaceholderKeyStats.sorted(stats, by: sort)
    }

    private func currentSelection() -> PlaceholderKeyStats? {
        let stats = stats()
        let key = ListSelection.effective(stats.map(\.key), selectedID)
        return stats.first { $0.key == key }
    }

    private func confirmClear() {
        guard let stat = pendingClear else { return }
        pendingClear = nil
        selectedID = ListSelection.neighbour(of: stat.key, in: stats().map(\.key))
        store.reloadIfChanged()
        do {
            try store.mutate { repository, _ in repository.data.clearPlaceholderHistory(forKey: stat.key) }
            toasts.success("History cleared", message: "Cleared \(valuesLabel(stat.valueCount)) for \"\(stat.key)\"")
        } catch {
            toasts.failure("Failed to clear history", message: error.localizedDescription)
        }
    }

    private func bindings(selection: PlaceholderKeyStats?) -> [KeyBinding] {
        if pendingClear != nil {
            return [
                KeyBinding(id: "confirmClearKey", title: "Clear", chord: KeyChord(.return)) { confirmClear() },
                KeyBinding(id: "cancelClearKey", title: "Cancel", chord: KeyChord(.escape)) { pendingClear = nil },
            ]
        }
        let hasSelection = selection != nil
        return ListSelection.arrowBindings(prefix: "historyKey") { offset in
            selectedID = ListSelection.moved(stats().map(\.key), from: currentSelection()?.key, by: offset)
        } + [
            KeyBinding(id: "viewValues", title: "View Values", chord: KeyChord(.return), isEnabled: hasSelection) {
                if let stat = currentSelection() { navigator.push(.history(key: stat.key)) }
            },
            KeyBinding(
                id: "clearKey", title: "Clear All Values (\(selection?.valueCount ?? 0))",
                chord: KeyChord(.character("x"), [.control, .shift]), isEnabled: hasSelection
            ) {
                pendingClear = currentSelection()
            },
            KeyBinding(id: "historySort", title: "Next Sort Order", chord: KeyChord(.character("p"), .command)) {
                let all = PlaceholderKeySortOption.allCases
                let index = all.firstIndex(of: sort) ?? 0
                sortRaw = all[(index + 1) % all.count].rawValue
                toasts.success("Sort: \(PlaceholderKeySortOption(rawValue: sortRaw)?.label ?? "")")
            },
        ]
    }
}

/// `Route.history(key:)`: one key's stored values, ranked and capped at the
/// display preference. ↵ edits a value in place; ⌃X deletes it.
struct PlaceholderHistoryDetailView: View {
    let key: String

    @Environment(SnippetStore.self) private var store
    @Environment(Navigator.self) private var navigator
    @Environment(ToastCenter.self) private var toasts

    @AppStorage("placeholders.maxDisplayedHistoryValues")
    private var maxDisplayedHistoryValues = StorageConstants.defaultMaxDisplayedHistoryValues

    @ViewState private var query = ""
    @ViewState private var selectedID: String?
    @ViewState private var pendingDelete: String?
    /// The value being edited and the edit field's text and error.
    @ViewState private var editing: String?
    @ViewState private var editText = ""
    @ViewState private var editError: String?
    @ViewState private var searchHandle = FocusHandle()
    @ViewState private var editHandle = FocusHandle()

    var body: some View {
        let stored = store.data.placeholderHistory(forKey: key)
        let values = values()
        let selection = values.first { $0.value == ListSelection.effective(values.map(\.value), selectedID) }
        let now = store.now()

        VStack(alignment: .leading, spacing: 0) {
            ManagementHeader(
                title: "Values for \"\(key)\"", placeholder: "Search values…", text: $query, handle: searchHandle)
            Divider()
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 1) {
                        if !values.isEmpty {
                            Text(
                                "Showing \(values.count) of \(stored.count) stored values (display limit \(maxDisplayedHistoryValues), storage limit \(StorageConstants.maxStoredValuesPerKey))"
                            )
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .padding(.horizontal, 10)
                            .padding(.top, 8)
                            .padding(.bottom, 2)
                        }
                        ForEach(values, id: \.value) { value in
                            row(value, isSelected: value.value == selection?.value, now: now)
                                .id(value.value)
                                .onTapGesture {
                                    selectedID = value.value
                                    searchHandle.focus()
                                }
                        }
                        if stored.isEmpty {
                            EmptyListMessage(title: "No values", message: "This placeholder key has no saved values")
                        } else if values.isEmpty {
                            EmptyListMessage(title: "No matching values", message: "Try a different search")
                        }
                    }
                    .padding(.horizontal, 8)
                    .padding(.vertical, 6)
                }
                .onChange(of: selection?.value) {
                    guard let value = selection?.value else { return }
                    proxy.scrollTo(value)
                }
            }
            .frame(maxHeight: .infinity)
            Divider()
            ManagementFooter(
                summary: valuesLabel(stored.count),
                primary: selection == nil ? nil : "↵ Edit Value",
                hints: selection == nil ? ["⌘K Actions", "⎋ Back"] : ["⌃X Delete", "⌘K Actions", "⎋ Back"])
        }
        .overlay {
            if let value = pendingDelete {
                ConfirmationOverlay(
                    title: "Delete Value",
                    message: "Delete \"\(value)\" from history for \"\(key)\"?",
                    confirmTitle: "Delete",
                    confirm: confirmDelete,
                    cancel: { pendingDelete = nil })
            } else if editing != nil {
                editOverlay
            }
        }
        .onAppear {
            store.reloadIfChanged()
            Task { @MainActor in searchHandle.focus() }
        }
        .onChange(of: query) { selectedID = nil }
        .keyBindings(bindings(selection: selection), for: .history(key: key))
    }

    private func row(_ value: PlaceholderHistoryValue, isSelected: Bool, now: Int64) -> some View {
        HStack(spacing: 10) {
            Image(systemName: "text.alignleft")
                .foregroundStyle(.secondary)
                .frame(width: 22)
            Text(value.value).lineLimit(1)
            Spacer(minLength: 0)
            CountChip(text: "\(value.useCount) use\(value.useCount == 1 ? "" : "s")", tint: .green)
            Text(Formatting.relativeTime(value.lastUsed, now: now))
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .selectableRow(isSelected: isSelected)
        .help(
            "Last used: \(Formatting.absoluteDate(value.lastUsed))\nCreated: \(Formatting.absoluteDate(value.createdAt))"
        )
    }

    /// An in-panel edit form over the list; ↵ saves and ⎋ cancels.
    private var editOverlay: some View {
        ZStack {
            Color.black.opacity(0.25)
                .onTapGesture(perform: cancelEdit)
            VStack(alignment: .leading, spacing: 10) {
                Text("Edit Value for \"\(key)\"").font(.headline)
                EditorTextField(
                    placeholder: "Enter new value",
                    text: Binding(
                        get: { editText },
                        set: {
                            editText = $0
                            editError = nil
                        }),
                    handle: editHandle)
                    .fixedSize(horizontal: false, vertical: true)
                if let editError {
                    Text(editError)
                        .font(.caption)
                        .foregroundStyle(.red)
                }
                HStack {
                    Spacer()
                    Button("Cancel  ⎋", action: cancelEdit)
                    Button("Update Value  ↵", action: submitEdit)
                        .buttonStyle(.borderedProminent)
                }
            }
            .padding(18)
            .frame(width: 420)
            .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 10))
            .shadow(radius: 12)
        }
        .onAppear {
            Task { @MainActor in editHandle.focus() }
        }
    }

    // MARK: State

    private func values() -> [PlaceholderHistoryValue] {
        let ranked = PlaceholderHistoryRanking.rank(store.data.placeholderHistory(forKey: key), now: store.now())
        let limited = ranked.prefix(max(1, maxDisplayedHistoryValues))
        let needle = query.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        return limited.filter { needle.isEmpty || $0.value.lowercased().contains(needle) }
    }

    private func currentSelection() -> PlaceholderHistoryValue? {
        let values = values()
        let id = ListSelection.effective(values.map(\.value), selectedID)
        return values.first { $0.value == id }
    }

    private func startEdit() {
        guard let value = currentSelection() else { return }
        editing = value.value
        editText = value.value
        editError = nil
    }

    private func cancelEdit() {
        editing = nil
        editError = nil
        searchHandle.focus()
    }

    private func submitEdit() {
        guard let oldValue = editing else { return }
        let newValue = editText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !newValue.isEmpty else {
            editError = "Value cannot be empty"
            return
        }
        guard newValue != oldValue else {
            editError = "New value must be different"
            return
        }
        store.reloadIfChanged()
        do {
            try store.mutate { repository, _ in
                try repository.data.updatePlaceholderValue(key: key, oldValue: oldValue, newValue: newValue)
            }
            editing = nil
            selectedID = newValue
            searchHandle.focus()
            toasts.success("Value updated", message: "Updated value for \"\(key)\"")
        } catch StorageError.duplicatePlaceholderValue {
            editError = StorageError.duplicatePlaceholderValue.localizedDescription
        } catch {
            toasts.failure("Failed to update value", message: error.localizedDescription)
        }
    }

    private func confirmDelete() {
        guard let value = pendingDelete else { return }
        pendingDelete = nil
        selectedID = ListSelection.neighbour(of: value, in: values().map(\.value))
        store.reloadIfChanged()
        do {
            try store.mutate { repository, _ in repository.data.deletePlaceholderValue(key: key, value: value) }
            toasts.success("Value deleted")
            // The key is gone once its last value is.
            if store.data.placeholderHistory(forKey: key).isEmpty {
                navigator.pop()
            }
        } catch {
            toasts.failure("Failed to delete value", message: error.localizedDescription)
        }
    }

    private func bindings(selection: PlaceholderHistoryValue?) -> [KeyBinding] {
        if pendingDelete != nil {
            return [
                KeyBinding(id: "confirmDeleteValue", title: "Delete", chord: KeyChord(.return)) { confirmDelete() },
                KeyBinding(id: "cancelDeleteValue", title: "Cancel", chord: KeyChord(.escape)) { pendingDelete = nil },
            ]
        }
        if editing != nil {
            return [
                KeyBinding(id: "submitEditValue", title: "Update Value", chord: KeyChord(.return)) { submitEdit() },
                KeyBinding(id: "cancelEditValue", title: "Cancel", chord: KeyChord(.escape)) { cancelEdit() },
            ]
        }
        let hasSelection = selection != nil
        return ListSelection.arrowBindings(prefix: "historyValue") { offset in
            selectedID = ListSelection.moved(values().map(\.value), from: currentSelection()?.value, by: offset)
        } + [
            KeyBinding(id: "editValue", title: "Edit Value", chord: KeyChord(.return), isEnabled: hasSelection) {
                startEdit()
            },
            KeyBinding(
                id: "deleteValue", title: "Delete Value", chord: KeyChord(.character("x"), .control),
                isEnabled: hasSelection
            ) {
                pendingDelete = currentSelection()?.value
            },
        ]
    }
}

private func valuesLabel(_ count: Int) -> String {
    "\(count) value\(count == 1 ? "" : "s")"
}
