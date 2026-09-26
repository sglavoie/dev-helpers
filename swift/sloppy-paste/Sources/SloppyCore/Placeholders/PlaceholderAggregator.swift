import Foundation

public enum PlaceholderAggregator {
    /// Collapses occurrences into one placeholder per key, then appends
    /// guard-only keys from `{{#if key}}` in source order.
    public static func aggregate(_ text: String, occurrences: [ParsedValuePlaceholderOccurrence]) -> [Placeholder] {
        var placeholders: [Placeholder] = []
        var indexByKey: [String: Int] = [:]

        for occurrence in occurrences {
            let placeholder = placeholder(from: occurrence)
            if let existingIndex = indexByKey[occurrence.key] {
                if occurrence.isChoiceDeclaration && placeholders[existingIndex].choices == nil {
                    // A declaration owns field-level metadata even if a plain
                    // reference was authored first. Keep the field's original
                    // ordering in the form.
                    placeholders[existingIndex] = placeholder
                }
            } else {
                indexByKey[occurrence.key] = placeholders.count
                placeholders.append(placeholder)
            }
        }

        // Preserve the established guard-only second pass and ordering.
        let ifRegex = /\{\{#if\s+(\+?)(\S+?)(?:\s+"([^"]*)")?\s*\}\}/.matchingSemantics(.unicodeScalar)
        for match in text.matches(of: ifRegex) {
            let key = String(match.output.2).trimmed
            if key.isEmpty || indexByKey[key] != nil { continue }
            indexByKey[key] = placeholders.count
            placeholders.append(
                Placeholder(
                    key: key,
                    isRequired: false,
                    isSaved: false,
                    isGuardOnly: true,
                    label: match.output.3.map(String.init),
                    defaultOn: match.output.1 == "+"
                ))
        }

        return placeholders
    }

    private static func placeholder(from occurrence: ParsedValuePlaceholderOccurrence) -> Placeholder {
        Placeholder(
            key: occurrence.key,
            choices: occurrence.choices,
            defaultValue: occurrence.explicitDefault,
            isRequired: occurrence.isRequired,
            isSaved: occurrence.isSaved,
            prefixWrapper: occurrence.prefixWrapper,
            suffixWrapper: occurrence.suffixWrapper
        )
    }

    /// One diagnostic per declaration of a key whose declarations disagree on
    /// choices, default, save policy or required/optional status.
    public static func findChoiceConflicts(_ occurrences: [ParsedValuePlaceholderOccurrence]) -> [PlaceholderSyntaxDiagnostic] {
        var keys: [String] = []
        var byKey: [String: [ParsedValuePlaceholderOccurrence]] = [:]
        for occurrence in occurrences where occurrence.isChoiceDeclaration {
            if byKey[occurrence.key] == nil { keys.append(occurrence.key) }
            byKey[occurrence.key, default: []].append(occurrence)
        }

        var diagnostics: [PlaceholderSyntaxDiagnostic] = []
        for key in keys {
            let declarations = byKey[key] ?? []
            if declarations.count < 2 { continue }
            let differingFields = differingDeclarationFields(declarations)
            if differingFields.isEmpty { continue }

            for declaration in declarations {
                diagnostics.append(
                    PlaceholderSyntaxDiagnostic(
                        range: declaration.range,
                        expression: declaration.raw,
                        message: "Conflicting authored choices for \(key.jsonQuoted) in \(declaration.raw): all declarations must use the same \(differingFields.joined(separator: ", "))."
                    ))
            }
        }
        return diagnostics
    }

    private static func differingDeclarationFields(_ declarations: [ParsedValuePlaceholderOccurrence]) -> [String] {
        var fields: [String] = []
        if !allEqual(declarations.map(\.choices)) { fields.append("choices") }
        if !allEqual(declarations.map { "\($0.hasExplicitDefault):\($0.explicitDefault ?? "")" }) {
            fields.append("default")
        }
        if !allEqual(declarations.map(\.isSaved)) { fields.append("save policy") }
        if !allEqual(declarations.map(\.isRequired)) { fields.append("required/optional status") }
        return fields
    }

    private static func allEqual<T: Equatable>(_ values: [T]) -> Bool {
        values.allSatisfy { $0 == values[0] }
    }
}
