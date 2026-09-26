import Foundation

/// Per-field hints shown under placeholder form controls.
public enum FieldPreview {
    static let maxBodyPreview = 80

    /// What an optional field will produce; nil for required fields and for
    /// guards without a matching `{{#if}}` block.
    public static func build(
        _ placeholder: Placeholder,
        snippetContent: String,
        currentValue: String,
        truncate: Bool = true
    ) -> String? {
        if placeholder.isRequired { return nil }

        if placeholder.isGuardOnly {
            guard let block = ConditionalBlocks.extractBodies(snippetContent, key: placeholder.key).first else {
                return nil
            }
            let ifText = normalizeBody(block.ifBody, truncate: truncate)
            if let elseBody = block.elseBody {
                return "Toggles: \"\(ifText)\" • Else: \"\(normalizeBody(elseBody, truncate: truncate))\""
            }
            return "Toggles: \"\(ifText)\""
        }

        if PlaceholderFormChoices.isWrapper(placeholder) {
            let displayValue = currentValue.trimmed.nilIfEmpty ?? "<value>"
            return "Wraps as: \(placeholder.prefixWrapper ?? "")\(displayValue)\(placeholder.suffixWrapper ?? "")"
        }

        switch placeholder.defaultValue {
        case nil: return "Optional — leave blank to omit."
        case "": return "Empty → renders as empty string."
        case let defaultValue?: return "Empty → uses default: \"\(defaultValue)\""
        }
    }

    /// A summary of the placeholder's rules, for the field's info tooltip.
    public static func infoText(_ placeholder: Placeholder) -> String {
        if placeholder.isGuardOnly {
            return "Conditional — checked = block shown, unchecked = block omitted"
        }
        var parts: [String] = []
        if placeholder.isRequired {
            parts.append("Required field")
        } else if PlaceholderFormChoices.isWrapper(placeholder) {
            let example = "\(placeholder.prefixWrapper ?? "")value\(placeholder.suffixWrapper ?? "")"
            parts.append("Optional wrapper • Output when included: \"\(example)\" • Uncheck to omit it")
        } else {
            parts.append("Optional (default: \"\(placeholder.defaultValue ?? "none")\")")
        }
        if let choices = placeholder.choices, !choices.isEmpty {
            parts.append("Configured choices: \(choices.map(\.jsonQuoted).joined(separator: ", "))")
            parts.append("Choose Enter custom value… to type another value")
        }
        if !placeholder.isSaved {
            parts.append("Won't be saved to history")
        }
        return parts.joined(separator: " • ")
    }

    private static func normalizeBody(_ body: String, truncate: Bool) -> String {
        let collapsed = body.replacing(/\s+/.matchingSemantics(.unicodeScalar), with: " ").trimmed
        guard truncate, collapsed.count > maxBodyPreview else { return collapsed }
        return String(collapsed.prefix(maxBodyPreview)) + "…"
    }
}
