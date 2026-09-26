import Foundation

/// What submitting the placeholder form does with the rendered content.
public enum PlaceholderFormMode: String, Sendable, Hashable, CaseIterable {
    /// Paste into the frontmost app and close.
    case paste
    /// Copy and close.
    case copy
    /// Copy and keep the picker open for more copies.
    case copyAndStay

    public var submitTitle: String {
        switch self {
        case .paste: "Paste & Close"
        case .copy: "Copy & Close"
        case .copyAndStay: "Copy & Stay Open"
        }
    }

    var verb: String { self == .paste ? "paste" : "copy" }
}

/// How a placeholder's value control is drawn.
public enum PlaceholderFieldControl: Sendable, Hashable {
    /// Guard-only `{{#if key}}` checkbox.
    case guardToggle
    /// Authored choices or history values, plus a custom-entry option.
    case dropdown
    case textField
}

/// A dropdown entry; `id` is what `selectOption` expects.
public struct PlaceholderFieldOption: Sendable, Hashable, Identifiable {
    public var id: String
    public var title: String
    public var isCustomEntry: Bool
}

/// All mutable state of the placeholder fill-in form: values, which fields
/// show a dropdown or free text, and which optional fields are enabled.
/// Values are seeded from placeholder history (or authored choices) on init.
public struct PlaceholderFormModel: Sendable, Hashable {
    public let snippetContent: String
    public let placeholders: [Placeholder]

    public private(set) var errors: [String: String] = [:]
    public private(set) var historySuggestions: [String: [String]] = [:]
    public private(set) var formValues: [String: String] = [:]
    public private(set) var dropdownSelections: [String: String] = [:]
    public private(set) var customValues: [String: String] = [:]
    public private(set) var useCustomInput: [String: Bool] = [:]
    public private(set) var enabledOptionals: [String: Bool] = [:]
    /// Keys pre-filled from their last-used history value.
    public private(set) var prefilledKeys: Set<String> = []

    /// - Parameters:
    ///   - snippetContent: content with system placeholders already resolved.
    ///   - maxDisplayValues: how many ranked history values each dropdown shows.
    public init(
        snippetContent: String,
        placeholders: [Placeholder],
        history: PlaceholderHistory,
        maxDisplayValues: Int = StorageConstants.defaultMaxDisplayedHistoryValues,
        now: Int64
    ) {
        self.snippetContent = snippetContent
        self.placeholders = placeholders

        for placeholder in placeholders {
            let key = placeholder.key
            if PlaceholderFormChoices.hasAuthoredChoices(placeholder) {
                let state = PlaceholderFormChoices.initialState(placeholder)
                historySuggestions[key] = []
                formValues[key] = state.formValue
                dropdownSelections[key] = state.dropdownSelection
                customValues[key] = state.customValue
                useCustomInput[key] = state.useCustomInput
                if let enabled = state.enabledOptional { enabledOptionals[key] = enabled }
                continue
            }

            let values = history[key] ?? []
            let ranked = PlaceholderHistoryRanking.rankedValues(values, now: now, limit: maxDisplayValues)
            historySuggestions[key] = ranked

            let lastUsed = PlaceholderHistoryRanking.lastUsedValue(values)
            if let lastUsed, !lastUsed.isEmpty { prefilledKeys.insert(key) }
            let initial = lastUsed ?? placeholder.defaultValue ?? ""
            formValues[key] = initial

            if ranked.isEmpty {
                useCustomInput[key] = true
            } else {
                dropdownSelections[key] = initial.isEmpty ? PlaceholderFormChoices.customValueMarker : initial
                useCustomInput[key] = initial.isEmpty
            }

            if !placeholder.isRequired && PlaceholderFormChoices.isWrapper(placeholder) {
                enabledOptionals[key] = !ranked.isEmpty || !(placeholder.defaultValue ?? "").isEmpty
            }
            if placeholder.isGuardOnly {
                enabledOptionals[key] = placeholder.defaultOn
            }
        }
    }

    // MARK: Layout

    public var requiredPlaceholders: [Placeholder] { placeholders.filter(\.isRequired) }
    public var optionalPlaceholders: [Placeholder] { placeholders.filter { !$0.isRequired } }
    /// Required fields come first, then optional ones, each in authored order.
    public var orderedPlaceholders: [Placeholder] { requiredPlaceholders + optionalPlaceholders }
    public var hasBothSections: Bool { !requiredPlaceholders.isEmpty && !optionalPlaceholders.isEmpty }

    public var filledRequiredCount: Int {
        requiredPlaceholders.filter { !(formValues[$0.key] ?? "").trimmed.isEmpty }.count
    }

    public func navigationTitle(snippetTitle: String) -> String {
        "Fill Placeholders (\(filledRequiredCount)/\(requiredPlaceholders.count)): \(snippetTitle)"
    }

    /// A banner describing history pre-fill, or nil when nothing was pre-filled.
    public func summary(mode: PlaceholderFormMode) -> String? {
        let requiredCount = requiredPlaceholders.count
        let prefilledCount = prefilledKeys.count
        let fieldWord = requiredCount == 1 ? "field" : "fields"
        if requiredCount > 0 && prefilledCount == requiredCount {
            return "All \(requiredCount) \(fieldWord) pre-filled from history — submit to \(mode.verb)."
        }
        if prefilledCount > 0 {
            return "\(prefilledCount) of \(requiredCount) \(fieldWord) pre-filled from history — review and submit."
        }
        return nil
    }

    // MARK: Fields

    public func control(for placeholder: Placeholder) -> PlaceholderFieldControl {
        if placeholder.isGuardOnly { return .guardToggle }
        let hasChoices = PlaceholderFormChoices.hasAuthoredChoices(placeholder)
        return hasChoices || !(historySuggestions[placeholder.key] ?? []).isEmpty ? .dropdown : .textField
    }

    /// Optional wrapper fields get an "Include" toggle that hides the value control.
    public func isWrapperField(_ placeholder: Placeholder) -> Bool {
        !placeholder.isRequired && PlaceholderFormChoices.isWrapper(placeholder)
    }

    /// Whether the guard or wrapper toggle is on; always true for other fields.
    public func isEnabled(_ placeholder: Placeholder) -> Bool {
        guard placeholder.isGuardOnly || isWrapperField(placeholder) else { return true }
        return enabledOptionals[placeholder.key] ?? false
    }

    public func showsCustomInput(_ placeholder: Placeholder) -> Bool {
        useCustomInput[placeholder.key] ?? false
    }

    public func value(for key: String) -> String { formValues[key] ?? "" }
    public func customValue(for key: String) -> String { customValues[key] ?? "" }

    public func dropdownSelection(for key: String) -> String {
        dropdownSelections[key]?.nilIfEmpty ?? PlaceholderFormChoices.customValueMarker
    }

    /// Authored choices (by index ID) or ranked history values, then the custom entry.
    public func dropdownOptions(for placeholder: Placeholder) -> [PlaceholderFieldOption] {
        let custom: PlaceholderFieldOption
        var options: [PlaceholderFieldOption]
        if let choices = placeholder.choices, !choices.isEmpty {
            options = choices.enumerated().map { index, choice in
                PlaceholderFieldOption(
                    id: PlaceholderFormChoices.authoredChoiceOptionID(index), title: choice, isCustomEntry: false)
            }
            custom = PlaceholderFieldOption(
                id: PlaceholderFormChoices.customValueMarker, title: "Enter custom value…", isCustomEntry: true)
        } else {
            options = (historySuggestions[placeholder.key] ?? []).map {
                PlaceholderFieldOption(id: $0, title: $0, isCustomEntry: false)
            }
            custom = PlaceholderFieldOption(
                id: PlaceholderFormChoices.customValueMarker, title: "Enter new value...", isCustomEntry: true)
        }
        options.append(custom)
        return options
    }

    /// The field label: `*` for required keys and a marker for history pre-fill.
    public func title(for placeholder: Placeholder) -> String {
        var title = placeholder.key
        if placeholder.isRequired { title += " *" }
        if prefilledKeys.contains(placeholder.key) { title += " (↻ last used)" }
        return title
    }

    /// The guard checkbox label: the authored label, else "Include key?".
    public func guardLabel(for placeholder: Placeholder) -> String {
        placeholder.label?.nilIfEmpty ?? "Include \(placeholder.key)?"
    }

    public func fieldPreview(for placeholder: Placeholder, truncate: Bool = true) -> String? {
        FieldPreview.build(placeholder, snippetContent: snippetContent, currentValue: value(for: placeholder.key),
            truncate: truncate)
    }

    // MARK: Editing

    public mutating func setEnabled(_ key: String, _ enabled: Bool) {
        enabledOptionals[key] = enabled
    }

    public mutating func setCustomValue(_ key: String, _ value: String) {
        customValues[key] = value
        formValues[key] = value
        errors[key] = nil
    }

    /// Applies a dropdown pick; an unknown authored option ID is ignored.
    public mutating func selectOption(_ optionID: String, for placeholder: Placeholder) {
        let key = placeholder.key
        if let choices = placeholder.choices, !choices.isEmpty {
            guard
                let state = PlaceholderFormChoices.resolveSelection(
                    choices, optionID: optionID, customValue: customValues[key] ?? "")
            else { return }
            dropdownSelections[key] = state.dropdownSelection
            useCustomInput[key] = state.useCustomInput
            formValues[key] = state.formValue
        } else {
            dropdownSelections[key] = optionID
            if optionID == PlaceholderFormChoices.customValueMarker {
                useCustomInput[key] = true
                formValues[key] = customValues[key] ?? ""
            } else {
                useCustomInput[key] = false
                formValues[key] = optionID
            }
        }
        errors[key] = nil
    }

    /// Resets every optional, non-guard field to its authored default (⌘D).
    public mutating func useDefaults() {
        for placeholder in placeholders where !placeholder.isRequired && !placeholder.isGuardOnly {
            let key = placeholder.key
            if PlaceholderFormChoices.hasAuthoredChoices(placeholder) {
                let state = PlaceholderFormChoices.initialState(placeholder)
                formValues[key] = state.formValue
                dropdownSelections[key] = state.dropdownSelection
                useCustomInput[key] = state.useCustomInput
                if state.useCustomInput { customValues[key] = state.customValue }
                if let enabled = state.enabledOptional { enabledOptionals[key] = enabled }
                continue
            }

            let defaultValue = placeholder.defaultValue ?? ""
            let suggestions = historySuggestions[key] ?? []
            formValues[key] = defaultValue
            customValues[key] = defaultValue
            if !suggestions.isEmpty {
                let useSuggestion = !defaultValue.isEmpty && suggestions.contains(defaultValue)
                dropdownSelections[key] = useSuggestion ? defaultValue : PlaceholderFormChoices.customValueMarker
                useCustomInput[key] = !useSuggestion
            }
            if PlaceholderFormChoices.isWrapper(placeholder) {
                enabledOptionals[key] = !defaultValue.isEmpty
            }
        }
    }

    // MARK: Output

    /// Form values with disabled toggles blanked and guards mapped to "true"/"".
    public var previewValues: [String: String] {
        guardMapped(disabledBlanked(formValues))
    }

    /// The live preview of the whole rendered snippet.
    public var previewContent: String {
        let afterBlocks = ConditionalBlocks.process(snippetContent, values: previewValues)
        return PlaceholderRenderer.replacePlaceholders(afterBlocks, values: previewValues, placeholders: placeholders)
    }

    /// Validates required fields (after blanking disabled toggles) and returns
    /// the final values, or nil after storing field errors.
    public mutating func submit() -> [String: String]? {
        let values = disabledBlanked(formValues)
        let newErrors = PlaceholderFormChoices.requiredErrors(placeholders, finalValues: values)
        guard newErrors.isEmpty else {
            errors = newErrors
            return nil
        }
        return guardMapped(values)
    }

    /// Validates and renders; nil when a required field is blank.
    public mutating func prepareSubmission() -> PreparedSnippet? {
        guard let finalValues = submit() else { return nil }
        return SnippetPreparation.submission(content: snippetContent, placeholders: placeholders,
            finalValues: finalValues)
    }

    private func disabledBlanked(_ values: [String: String]) -> [String: String] {
        var result = values
        for (key, enabled) in enabledOptionals where !enabled { result[key] = "" }
        return result
    }

    private func guardMapped(_ values: [String: String]) -> [String: String] {
        var result = values
        for placeholder in placeholders where placeholder.isGuardOnly {
            result[placeholder.key] = enabledOptionals[placeholder.key] == true ? "true" : ""
        }
        return result
    }
}
