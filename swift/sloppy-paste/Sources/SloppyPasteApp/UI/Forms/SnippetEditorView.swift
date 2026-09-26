import AppKit
import SwiftUI
import SloppyCore

/// Creates or edits a snippet: new, edit, or new from the clipboard.
///
/// Content is an `NSTextView` with smart substitutions off. Tab/⇧Tab walk
/// title → content → description → tags through the KeyRouter (⌥Tab types
/// a tab in the content). Validation and the placeholder preview update
/// live; ⇧⌘P opens the syntax list, whose entries insert at the cursor on
/// ⌘1–⌘7. ⌘T opens the tag picker over the form, writing its result back
/// to the tags field. Esc with unsaved changes asks before discarding.
struct SnippetEditorView: View {
    let mode: EditorMode

    @Environment(SnippetStore.self) private var store
    @Environment(Navigator.self) private var navigator
    @Environment(ToastCenter.self) private var toasts

    @ViewState private var draft = SnippetDraft()
    @ViewState private var original: SnippetDraft?
    @ViewState private var missingSnippet = false
    @ViewState private var showAllErrors = false
    @ViewState private var showingSyntax = false
    @ViewState private var confirmingDiscard = false
    /// The ⌘T tag picker, shown over the form so the draft survives.
    @ViewState private var tagPicker: TagPickerModel?
    @ViewState private var tagSearchHandle = FocusHandle()
    @ViewState private var focusedField: SnippetDraft.Field?
    @ViewState private var focusHandles = Dictionary(
        uniqueKeysWithValues: SnippetDraft.Field.allCases.map { ($0, FocusHandle()) })

    private static let fieldOrder = SnippetDraft.Field.allCases

    var body: some View {
        Group {
            if missingSnippet {
                VStack(spacing: 6) {
                    Text("Snippet not found").font(.headline)
                    Text("It may have been deleted. Press Esc to go back.").foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if original != nil {
                editor
            } else {
                Color.clear
            }
        }
        .overlay {
            if let tagPicker {
                TagPickerView(model: tagPicker, searchHandle: tagSearchHandle, escapeTitle: "Done")
                    .background(Color(nsColor: .windowBackgroundColor))
            }
        }
        .overlay {
            if confirmingDiscard {
                ConfirmationOverlay(
                    title: "Discard Changes?",
                    message: "Your edits to this snippet have not been saved.",
                    confirmTitle: "Discard",
                    confirm: { navigator.pop() },
                    cancel: { confirmingDiscard = false })
            }
        }
        .onAppear(perform: start)
        .onChange(of: focusedField) { applyFocus() }
        .keyBindings(bindings, for: .editor(mode))
    }

    // MARK: Layout

    private var editor: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(navigationTitle)
                .font(.headline)
                .padding(.horizontal, 16)
                .padding(.vertical, 12)
            Divider()
            HStack(spacing: 0) {
                fields
                    .frame(maxWidth: .infinity)
                Divider()
                sidePane
                    .frame(width: 280)
            }
            .frame(maxHeight: .infinity)
            Divider()
            footer
        }
    }

    private var fields: some View {
        let errors = visibleErrors
        return VStack(alignment: .leading, spacing: 10) {
            fieldLabel("Title", info: Validation.characterInfo(draft.title, maxLength: ValidationLimits.titleMaxLength))
            textField(.title, "Enter snippet title", text: $draft.title)
            errorLine(errors[.title])

            fieldLabel(
                "Content", info: Validation.characterInfo(draft.content, maxLength: ValidationLimits.contentMaxLength))
            SnippetTextView(text: $draft.content, handle: handle(.content)) { focusedField = .content }
                .frame(minHeight: 120, maxHeight: .infinity)
                .background(Color(nsColor: .textBackgroundColor).opacity(0.6), in: RoundedRectangle(cornerRadius: 5))
                .overlay(
                    RoundedRectangle(cornerRadius: 5)
                        .strokeBorder(
                            focusedField == .content ? Color.accentColor : Color.secondary.opacity(0.35),
                            lineWidth: focusedField == .content ? 2 : 1))
            errorLine(errors[.content])

            fieldLabel("Description", info: nil)
            textField(.description, "Optional description", text: $draft.description, multiline: true)

            fieldLabel("Tags", info: nil)
            textField(.tags, "Comma-separated, e.g. work/projects, email", text: $draft.tagsText)
            if let error = errors[.tags] {
                errorLine(error)
            } else {
                Text("Use slashes for hierarchy (work/projects) and dashes instead of spaces.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(16)
    }

    private func textField(
        _ field: SnippetDraft.Field, _ placeholder: String, text: Binding<String>, multiline: Bool = false
    ) -> some View {
        EditorTextField(placeholder: placeholder, text: text, handle: handle(field), multiline: multiline) {
            focusedField = field
        }
        .fixedSize(horizontal: false, vertical: true)
    }

    private func handle(_ field: SnippetDraft.Field) -> FocusHandle {
        focusHandles[field] ?? FocusHandle()
    }

    private func fieldLabel(_ title: String, info: Validation.CharacterInfo?) -> some View {
        HStack {
            Text(title).font(.caption).fontWeight(.semibold)
            Spacer()
            if let info {
                Text(info.info).font(.caption2).foregroundStyle(.secondary)
            }
        }
    }

    @ViewBuilder
    private func errorLine(_ error: String?) -> some View {
        if let error {
            Text(error)
                .font(.caption)
                .foregroundStyle(.red)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    @ViewBuilder
    private var sidePane: some View {
        if showingSyntax {
            syntaxList
        } else {
            ScrollView {
                VStack(alignment: .leading, spacing: 10) {
                    let preview = SnippetPreview.build(draft.content, now: store.now())
                    Text("Preview").font(.caption).fontWeight(.semibold)
                    Text(preview.isEmpty ? "No placeholders" : preview)
                        .font(.system(.callout, design: .monospaced))
                        .foregroundStyle(preview.isEmpty ? .secondary : .primary)
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Divider()
                    Text("Placeholder syntax").font(.caption).fontWeight(.semibold)
                    ForEach(Self.syntaxReference, id: \.0) { title, text in
                        VStack(alignment: .leading, spacing: 1) {
                            Text(title).font(.caption2).fontWeight(.semibold)
                            Text(text).font(.caption2).foregroundStyle(.secondary)
                        }
                    }
                }
                .padding(12)
            }
        }
    }

    private var syntaxList: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Insert Placeholder Syntax").font(.caption).fontWeight(.semibold)
                .padding(.bottom, 4)
            ForEach(PlaceholderSyntaxHelper.all) { helper in
                Button { insert(helper) } label: {
                    HStack(alignment: .firstTextBaseline) {
                        Text("⌘\(String(helper.key))").font(.caption).foregroundStyle(.secondary).frame(width: 26)
                        VStack(alignment: .leading, spacing: 1) {
                            Text(helper.title)
                            Text(helper.subtitle).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
                        }
                        Spacer(minLength: 0)
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .padding(.vertical, 3)
            }
            Spacer()
            Text("⎋ closes this list").font(.caption2).foregroundStyle(.secondary)
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private static let syntaxReference: [(String, String)] = [
        ("Placeholders", "Required {{name}}  Optional {{name|default}}  No-save {{!name}}"),
        ("Choices", "{{tone[Formal|Casual]}}; default: {{tone[Formal|Casual]|Casual}}"),
        ("Wrappers", "{{prefix:key:suffix}}: the wrapping text appears only with a value, e.g. {{$:price: USD}}"),
        ("Conditionals", "{{#if key}}…{{/if}}, with {{#else}} for an alternative"),
        ("System (auto)", "{{DATE}} {{TIME}} {{DATETIME}} {{TODAY}} {{NOW}} {{YEAR}} {{MONTH}} {{DAY}}"),
    ]

    private var footer: some View {
        HStack(spacing: 14) {
            Text("⇥ Next Field")
            Text("⌘T Tags")
            Text("⇧⌘P Insert Placeholder")
            Spacer()
            Text("⌘↵ \(submitTitle)").fontWeight(.semibold)
            Text("⌘K Actions")
            Text("⎋ Cancel")
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
    }

    // MARK: State

    private var navigationTitle: String {
        switch mode {
        case .new: "New Snippet"
        case .edit: "Edit Snippet"
        case .fromClipboard: "New Snippet from Clipboard"
        }
    }

    private var submitTitle: String {
        switch mode {
        case .new: "Create Snippet"
        case .edit: "Update Snippet"
        case .fromClipboard: "Save Snippet"
        }
    }

    /// Errors for fields the user has typed into, or all of them after a
    /// save attempt, so a fresh form does not open in red.
    private var visibleErrors: [SnippetDraft.Field: String] {
        let errors = draft.errors
        guard !showAllErrors else { return errors }
        return errors.filter { field, _ in
            switch field {
            case .title: !draft.title.isEmpty
            case .content: !draft.content.isEmpty
            case .tags: !draft.tagsText.isEmpty
            case .description: true
            }
        }
    }

    private var isDirty: Bool { original != nil && draft != original }

    private func start() {
        guard original == nil, !missingSnippet else { return }
        switch mode {
        case .new:
            draft = SnippetDraft()
        case .edit(let id):
            guard let snippet = store.snippets.first(where: { $0.id == id }) else {
                missingSnippet = true
                return
            }
            draft = SnippetDraft(snippet: snippet)
        case .fromClipboard:
            guard let clipboard = SnippetDraft(clipboard: NSPasteboard.general.string(forType: .string)) else {
                toasts.failure("Clipboard is empty", message: "Copy some text first, then try again")
                Task { @MainActor in navigator.pop() }
                return
            }
            draft = clipboard
        }
        original = draft
        focusedField = .title
        // The fields render only once `original` is set; focus after that pass.
        Task { @MainActor in applyFocus() }
    }

    private func applyFocus() {
        guard let focusedField, !handle(focusedField).isFocused else { return }
        handle(focusedField).focus()
    }

    private func moveFocus(by offset: Int) {
        let order = Self.fieldOrder
        let current = focusedField.flatMap { order.firstIndex(of: $0) } ?? (offset > 0 ? -1 : 0)
        focusedField = order[(current + offset + order.count) % order.count]
    }

    private func insert(_ helper: PlaceholderSyntaxHelper) {
        showingSyntax = false
        focusedField = .content
        handle(.content).insert(helper.content, selecting: helper.keyRange)
    }

    private func save() {
        if let field = draft.firstErrorField {
            showAllErrors = true
            focusedField = field
            return
        }
        store.reloadIfChanged()
        do {
            switch mode {
            case .new, .fromClipboard:
                try store.mutate { repository, now in try draft.create(in: &repository, now: now) }
            case .edit(let id):
                try store.mutate { repository, now in try draft.update(id: id, in: &repository, now: now) }
            }
        } catch {
            let title = if case .edit = mode { "Failed to update snippet" } else { "Failed to create snippet" }
            toasts.failure(title, message: String(describing: error))
            return
        }
        switch mode {
        case .new: toasts.success("Snippet created")
        case .edit: toasts.success("Snippet updated")
        case .fromClipboard: toasts.success("Snippet saved", message: draft.title.trimmingCharacters(in: .whitespaces))
        }
        navigator.pop()
    }

    /// ⌘T: tags known across the store plus the draft's own; each toggle
    /// rewrites the tags field. Tags the field cannot parse are left out.
    private func openTagPicker() {
        let current = SnippetDraft.parseTags(draft.tagsText).tags
        tagPicker = TagPickerModel(
            title: "Edit Tags", initialTags: current,
            knownTags: SnippetRepository(data: store.data).tags, toasts: toasts
        ) { tags in
            draft.tagsText = tags.joined(separator: ", ")
        }
    }

    private func closeTagPicker() {
        tagPicker = nil
        focusedField = .tags
        Task { @MainActor in applyFocus() }
    }

    private func cancel() {
        if isDirty {
            confirmingDiscard = true
        } else {
            navigator.pop()
        }
    }

    private var bindings: [KeyBinding] {
        if let tagPicker {
            return tagPicker.bindings + [
                KeyBinding(id: "closeTags", title: "Done", chord: KeyChord(.escape), showsInMenu: false) {
                    closeTagPicker()
                }
            ]
        }
        if confirmingDiscard {
            return [
                KeyBinding(id: "confirmDiscard", title: "Discard", chord: KeyChord(.return)) { navigator.pop() },
                KeyBinding(id: "cancelDiscard", title: "Keep Editing", chord: KeyChord(.escape)) {
                    confirmingDiscard = false
                },
            ]
        }
        var bindings = [
            KeyBinding(id: "save", title: submitTitle, chord: KeyChord(.return, .command), isEnabled: !missingSnippet) {
                save()
            },
            KeyBinding(id: "nextField", title: "Next Field", chord: KeyChord(.tab), showsInMenu: false) {
                moveFocus(by: 1)
            },
            KeyBinding(id: "previousField", title: "Previous Field", chord: KeyChord(.tab, .shift), showsInMenu: false) {
                moveFocus(by: -1)
            },
            KeyBinding(id: "editTags", title: "Edit Tags", chord: KeyChord(.character("t"), .command), isEnabled: !missingSnippet) {
                openTagPicker()
            },
            KeyBinding(
                id: "insertSyntax", title: showingSyntax ? "Hide Placeholder Syntax" : "Insert Placeholder Syntax",
                chord: KeyChord(.character("p"), [.command, .shift])
            ) {
                showingSyntax.toggle()
            },
            KeyBinding(id: "cancel", title: "Cancel", chord: KeyChord(.escape), showsInMenu: false) {
                if showingSyntax {
                    showingSyntax = false
                } else {
                    cancel()
                }
            },
        ]
        bindings += PlaceholderSyntaxHelper.all.map { helper in
            KeyBinding(
                id: "insertSyntax\(helper.key)", title: "Insert \(helper.title)",
                chord: KeyChord(.character(helper.key), .command), showsInMenu: false, isEnabled: showingSyntax
            ) {
                insert(helper)
            }
        }
        return bindings
    }
}
