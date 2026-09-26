import Foundation

/// Dropdown option IDs and field state for placeholders with authored choices.
public enum PlaceholderFormChoices {
    /// The dropdown option that switches a field to free-text input.
    public static let customValueMarker = "__CUSTOM_VALUE__"
    static let authoredChoiceOptionPrefix = "__AUTHORED_CHOICE__"
    public static let requiredFieldError = "This field is required"

    public static func hasAuthoredChoices(_ placeholder: Placeholder) -> Bool {
        !(placeholder.choices ?? []).isEmpty
    }

    public static func isWrapper(_ placeholder: Placeholder) -> Bool {
        placeholder.prefixWrapper != nil || placeholder.suffixWrapper != nil
    }

    /// Index-based IDs, so an authored choice can never collide with the custom marker.
    public static func authoredChoiceOptionID(_ index: Int) -> String {
        "\(authoredChoiceOptionPrefix)\(index)"
    }

    public static func authoredChoiceValue(_ choices: [String], optionID: String) -> String? {
        guard optionID.hasPrefix(authoredChoiceOptionPrefix) else { return nil }
        let rawIndex = optionID.dropFirst(authoredChoiceOptionPrefix.count)
        guard !rawIndex.isEmpty, rawIndex.allSatisfy({ $0.isASCII && $0.isNumber }),
            let index = Int(rawIndex), choices.indices.contains(index)
        else { return nil }
        return choices[index]
    }

    /// The initial state: the matching explicit default, else Custom holding an
    /// out-of-list default, else the first choice. Optional wrappers start
    /// enabled only with a non-empty default.
    public static func initialState(_ placeholder: Placeholder) -> AuthoredChoiceFieldState {
        let choices = placeholder.choices ?? []
        let enabledOptional: Bool? =
            !placeholder.isRequired && isWrapper(placeholder) ? !(placeholder.defaultValue ?? "").isEmpty : nil

        if let defaultValue = placeholder.defaultValue {
            if let index = choices.firstIndex(of: defaultValue) {
                return AuthoredChoiceFieldState(
                    formValue: choices[index],
                    dropdownSelection: authoredChoiceOptionID(index),
                    customValue: "",
                    useCustomInput: false,
                    enabledOptional: enabledOptional
                )
            }
            return AuthoredChoiceFieldState(
                formValue: defaultValue,
                dropdownSelection: customValueMarker,
                customValue: defaultValue,
                useCustomInput: true,
                enabledOptional: enabledOptional
            )
        }

        return AuthoredChoiceFieldState(
            formValue: choices.first ?? "",
            dropdownSelection: choices.isEmpty ? customValueMarker : authoredChoiceOptionID(0),
            customValue: "",
            useCustomInput: choices.isEmpty,
            enabledOptional: enabledOptional
        )
    }

    /// The state after picking `optionID`; nil for an unknown option. The custom
    /// text is kept across switches between authored and Custom modes.
    public static func resolveSelection(
        _ choices: [String],
        optionID: String,
        customValue: String
    ) -> AuthoredChoiceFieldState? {
        if optionID == customValueMarker {
            return AuthoredChoiceFieldState(
                formValue: customValue,
                dropdownSelection: customValueMarker,
                customValue: customValue,
                useCustomInput: true
            )
        }
        guard let authoredValue = authoredChoiceValue(choices, optionID: optionID) else { return nil }
        return AuthoredChoiceFieldState(
            formValue: authoredValue,
            dropdownSelection: optionID,
            customValue: customValue,
            useCustomInput: false
        )
    }

    /// Errors keyed by placeholder key for required fields left blank.
    public static func requiredErrors(_ placeholders: [Placeholder], finalValues: [String: String]) -> [String: String] {
        var errors: [String: String] = [:]
        for placeholder in placeholders where placeholder.isRequired {
            if (finalValues[placeholder.key] ?? "").trimmed.isEmpty {
                errors[placeholder.key] = requiredFieldError
            }
        }
        return errors
    }
}

public struct AuthoredChoiceFieldState: Sendable, Hashable {
    public var formValue: String
    public var dropdownSelection: String
    public var customValue: String
    public var useCustomInput: Bool
    /// Set only for optional wrapper fields.
    public var enabledOptional: Bool?

    public init(
        formValue: String,
        dropdownSelection: String,
        customValue: String,
        useCustomInput: Bool,
        enabledOptional: Bool? = nil
    ) {
        self.formValue = formValue
        self.dropdownSelection = dropdownSelection
        self.customValue = customValue
        self.useCustomInput = useCustomInput
        self.enabledOptional = enabledOptional
    }
}
