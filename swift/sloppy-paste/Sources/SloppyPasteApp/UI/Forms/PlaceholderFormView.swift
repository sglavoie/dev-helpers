import SwiftUI
import SloppyCore

/// Fills a snippet's placeholders, then pastes, copies, or copies and stays.
///
/// Keyboard focus is kept in `focusedStop` and walked with Tab/⇧Tab through
/// `PlaceholderFormModel.focusStops`, so every checkbox and dropdown is
/// reachable in layout order. Text fields take real focus; checkboxes answer
/// Space and dropdowns ↑/↓ through the KeyRouter while they hold the stop.
struct PlaceholderFormView: View {
    let snippetID: String
    let mode: PlaceholderFormMode

    @Environment(SnippetStore.self) private var store
    @Environment(Navigator.self) private var navigator
    @Environment(\.pickerPanel) private var pickerPanel

    @AppStorage("placeholders.maxDisplayedHistoryValues")
    private var maxDisplayedHistoryValues = StorageConstants.defaultMaxDisplayedHistoryValues

    @ViewState private var snippet: Snippet?
    @ViewState private var model: PlaceholderFormModel?
    @ViewState private var focusedStop: PlaceholderFocusStop?
    @FocusState private var textFocus: PlaceholderFocusStop?

    private var route: Route { .placeholderForm(snippetID: snippetID, mode: mode) }

    var body: some View {
        Group {
            if let snippet, let model {
                form(snippet, model)
            } else {
                VStack(spacing: 6) {
                    Text("Snippet not found").font(.headline)
                    Text("It may have been deleted. Press Esc to go back.").foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .onAppear(perform: start)
        .onChange(of: focusedStop) {
            textFocus = focusedStop?.isTextInput == true ? focusedStop : nil
        }
        .onChange(of: textFocus) {
            if let textFocus { focusedStop = textFocus }
        }
        .keyBindings(bindings, for: route)
    }

    // MARK: Layout

    private func form(_ snippet: Snippet, _ model: PlaceholderFormModel) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 4) {
                Text(model.navigationTitle(snippetTitle: snippet.title))
                    .font(.headline)
                    .lineLimit(1)
                if let summary = model.summary(mode: mode) {
                    Label(summary, systemImage: "clock.arrow.circlepath")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 12)
            Divider()
            HStack(spacing: 0) {
                fields(model)
                    .frame(maxWidth: .infinity)
                Divider()
                preview(model)
                    .frame(width: 300)
            }
            .frame(maxHeight: .infinity)
            Divider()
            footer
        }
    }

    private func fields(_ model: PlaceholderFormModel) -> some View {
        ScrollViewReader { proxy in
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    Text("Required fields (*) must be filled. Unchecked wrapper fields are left out; "
                        + "conditional checkboxes show or hide whole blocks.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    ForEach(model.requiredPlaceholders, id: \.key) { field($0, model) }
                    if model.hasBothSections {
                        Divider()
                        Text("Optional fields")
                            .font(.caption)
                            .fontWeight(.semibold)
                            .foregroundStyle(.secondary)
                    }
                    ForEach(model.optionalPlaceholders, id: \.key) { field($0, model) }
                }
                .padding(16)
            }
            .onChange(of: focusedStop) {
                guard let focusedStop else { return }
                withAnimation { proxy.scrollTo(focusedStop) }
            }
        }
    }

    @ViewBuilder
    private func field(_ placeholder: Placeholder, _ model: PlaceholderFormModel) -> some View {
        let key = placeholder.key
        VStack(alignment: .leading, spacing: 5) {
            if placeholder.isGuardOnly {
                Text(model.guardLabel(for: placeholder)).fontWeight(.medium)
                toggle("Include in output", key: key, kind: .guardToggle)
            } else {
                Text(model.title(for: placeholder)).fontWeight(.medium)
                if model.isWrapperField(placeholder) {
                    toggle("Include \(key)", key: key, kind: .includeToggle)
                }
                if model.isEnabled(placeholder) {
                    valueControl(placeholder, model)
                }
            }
            if let hint = model.fieldPreview(for: placeholder) {
                Text(hint)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
            if let error = model.errors[key] {
                Text(error)
                    .font(.caption)
                    .foregroundStyle(.red)
            }
        }
        .help(FieldPreview.infoText(placeholder))
    }

    @ViewBuilder
    private func valueControl(_ placeholder: Placeholder, _ model: PlaceholderFormModel) -> some View {
        let key = placeholder.key
        if model.control(for: placeholder) == .dropdown {
            let stop = PlaceholderFocusStop(key: key, kind: .dropdown)
            Picker(selection: selectionBinding(placeholder)) {
                ForEach(model.dropdownOptions(for: placeholder)) { option in
                    Text(option.title).tag(option.id)
                }
            } label: {
                EmptyView()
            }
            .pickerStyle(.menu)
            .labelsHidden()
            .fixedSize()
            .focusRing(focusedStop == stop)
            .simultaneousGesture(TapGesture().onEnded { focusedStop = stop })
            .id(stop)
            if model.showsCustomInput(placeholder) {
                textField(placeholder, kind: .customText, prompt: "Enter custom value...")
            }
        } else {
            textField(placeholder, kind: .text, prompt: "Enter value...")
        }
    }

    private func textField(_ placeholder: Placeholder, kind: PlaceholderFocusStop.Kind, prompt: String) -> some View {
        let key = placeholder.key
        let stop = PlaceholderFocusStop(key: key, kind: kind)
        let text = Binding<String>(
            get: { kind == .customText ? model?.customValue(for: key) ?? "" : model?.value(for: key) ?? "" },
            set: { model?.setCustomValue(key, $0) }
        )
        let promptText = placeholder.defaultValue.flatMap { $0.isEmpty ? nil : $0 } ?? prompt
        return TextField("", text: text, prompt: Text(promptText))
            .textFieldStyle(.roundedBorder)
            .focused($textFocus, equals: stop)
            .id(stop)
    }

    private func toggle(_ title: String, key: String, kind: PlaceholderFocusStop.Kind) -> some View {
        let stop = PlaceholderFocusStop(key: key, kind: kind)
        let isOn = Binding<Bool>(
            get: { model?.enabledOptionals[key] ?? false },
            set: { newValue in
                model?.setEnabled(key, newValue)
                focusedStop = stop
            }
        )
        return Toggle(title, isOn: isOn)
            .toggleStyle(.checkbox)
            .focusRing(focusedStop == stop)
            .id(stop)
    }

    private func selectionBinding(_ placeholder: Placeholder) -> Binding<String> {
        Binding(
            get: { model?.dropdownSelection(for: placeholder.key) ?? PlaceholderFormChoices.customValueMarker },
            set: { optionID in
                model?.selectOption(optionID, for: placeholder)
                focusedStop = PlaceholderFocusStop(key: placeholder.key, kind: .dropdown)
            }
        )
    }

    private func preview(_ model: PlaceholderFormModel) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Preview")
                .font(.caption)
                .fontWeight(.semibold)
                .foregroundStyle(.secondary)
            ScrollView {
                Text(model.previewContent)
                    .font(.system(.body, design: .monospaced))
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .padding(12)
    }

    private var footer: some View {
        HStack(spacing: 14) {
            Text("⇥ Next Field")
            Spacer()
            Text("⌘↵ \(mode.submitTitle)").fontWeight(.semibold)
            ForEach(alternateModes, id: \.mode) { alternate in
                Text("\(alternate.chord.label) \(alternate.mode.submitTitle)")
            }
            Text("⌘D Defaults")
            Text("⌘K Actions")
            Text("⎋ Back")
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
    }

    // MARK: Actions

    private func start() {
        guard model == nil, let stored = store.snippets.first(where: { $0.id == snippetID }) else { return }
        switch SnippetPreparation.plan(for: stored, now: store.now()) {
        case .form(let request):
            snippet = request.snippet
            let form = PlaceholderFormModel(
                snippetContent: request.snippet.content,
                placeholders: request.placeholders,
                history: store.data.placeholderHistory,
                maxDisplayValues: maxDisplayedHistoryValues,
                now: store.now())
            model = form
            focusedStop = form.focusStops.first
        case .direct(let prepared):
            // The snippet lost its placeholders since the picker opened.
            pickerPanel?.deliver(prepared, snippetID: stored.id, mode: .init(mode))
            if mode == .copyAndStay { navigator.pop() }
        }
    }

    private func submit(_ submitMode: PlaceholderFormMode) {
        guard var form = model, let snippet else { return }
        let prepared = form.prepareSubmission()
        model = form
        guard let prepared else {
            focusedStop = form.firstErrorStop
            return
        }
        pickerPanel?.deliver(prepared, snippetID: snippet.id, mode: .init(submitMode))
        // Like the extension, copy-and-stay returns to the list for another pick.
        if submitMode == .copyAndStay { navigator.pop() }
    }

    private func moveFocus(by offset: Int) {
        focusedStop = model?.focusStop(after: focusedStop, offset: offset)
    }

    /// Space on a checkbox stop toggles it.
    private func toggleFocused() {
        guard let stop = focusedStop, stop.kind == .guardToggle || stop.kind == .includeToggle else { return }
        model?.setEnabled(stop.key, !(model?.enabledOptionals[stop.key] ?? false))
    }

    /// ↑/↓ on a dropdown stop picks the neighbouring option.
    private func stepFocusedOption(by offset: Int) {
        guard let stop = focusedStop, stop.kind == .dropdown,
            let placeholder = model?.placeholders.first(where: { $0.key == stop.key }),
            let optionID = model?.adjacentOptionID(for: placeholder, offset: offset)
        else { return }
        model?.selectOption(optionID, for: placeholder)
    }

    private struct AlternateMode {
        var mode: PlaceholderFormMode
        var chord: KeyChord
    }

    /// The other submit modes, each on a fixed chord.
    private var alternateModes: [AlternateMode] {
        [
            AlternateMode(mode: .copy, chord: KeyChord(.return, [.command, .option])),
            AlternateMode(mode: .copyAndStay, chord: KeyChord(.return, [.command, .control])),
        ]
        .filter { $0.mode != mode }
    }

    private var bindings: [KeyBinding] {
        let kind = focusedStop?.kind
        let onToggle = kind == .guardToggle || kind == .includeToggle
        let onDropdown = kind == .dropdown
        var bindings = [
            KeyBinding(id: "submit", title: mode.submitTitle, chord: KeyChord(.return, .command)) {
                submit(mode)
            },
        ]
        bindings += alternateModes.map { alternate in
            KeyBinding(id: "submit.\(alternate.mode.rawValue)", title: alternate.mode.submitTitle,
                chord: alternate.chord) {
                submit(alternate.mode)
            }
        }
        bindings += [
            KeyBinding(id: "defaults", title: "Use Defaults for All Optional", chord: KeyChord(.character("d"), .command)) {
                model?.useDefaults()
            },
            KeyBinding(id: "nextField", title: "Next Field", chord: KeyChord(.tab), showsInMenu: false) {
                moveFocus(by: 1)
            },
            KeyBinding(id: "previousField", title: "Previous Field", chord: KeyChord(.tab, .shift), showsInMenu: false) {
                moveFocus(by: -1)
            },
            KeyBinding(
                id: "toggle", title: "Toggle", chord: KeyChord(.character(" ")), showsInMenu: false, isEnabled: onToggle
            ) {
                toggleFocused()
            },
            KeyBinding(
                id: "previousOption", title: "Previous Option", chord: KeyChord(.upArrow), showsInMenu: false,
                isEnabled: onDropdown
            ) {
                stepFocusedOption(by: -1)
            },
            KeyBinding(
                id: "nextOption", title: "Next Option", chord: KeyChord(.downArrow), showsInMenu: false,
                isEnabled: onDropdown
            ) {
                stepFocusedOption(by: 1)
            },
        ]
        return bindings
    }
}

private extension View {
    /// Marks the checkbox or dropdown that holds the keyboard stop.
    func focusRing(_ isFocused: Bool) -> some View {
        padding(3)
            .overlay(
                RoundedRectangle(cornerRadius: 5)
                    .stroke(Color.accentColor, lineWidth: 2)
                    .opacity(isFocused ? 1 : 0)
            )
    }
}
