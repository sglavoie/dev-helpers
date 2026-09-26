import Foundation

public enum PlaceholderRenderer {
    /// The full snippet pipeline: system placeholders, then conditional blocks,
    /// then value placeholders (with placeholders extracted after the system
    /// pass, so `{{DATE}}` never becomes a user field).
    public static func render(
        _ content: String,
        values: [String: String],
        now: Int64,
        timeZone: TimeZone = .current
    ) -> String {
        let processed = SystemPlaceholders.process(content, now: now, timeZone: timeZone)
        let placeholders = PlaceholderSyntaxParser.extractPlaceholders(processed)
        let afterBlocks = ConditionalBlocks.process(processed, values: values)
        return replacePlaceholders(afterBlocks, values: values, placeholders: placeholders)
    }

    /// Replaces each value placeholder occurrence with its value, falling back
    /// to the canonical default. Wrappers apply only around a non-blank value,
    /// and occurrences whose key is not in `placeholders` stay verbatim.
    public static func replacePlaceholders(
        _ text: String,
        values: [String: String],
        placeholders: [Placeholder]
    ) -> String {
        var canonicalByKey: [String: Placeholder] = [:]
        for placeholder in placeholders { canonicalByKey[placeholder.key] = placeholder }

        var result = ""
        var cursor = text.startIndex
        for occurrence in PlaceholderSyntaxParser.parse(text).occurrences {
            result += text[cursor..<occurrence.indices.lowerBound]
            cursor = occurrence.indices.upperBound
            guard let canonical = canonicalByKey[occurrence.key] else {
                result += occurrence.raw
                continue
            }

            let value = values[occurrence.key] ?? canonical.defaultValue ?? ""
            if !value.trimmed.isEmpty {
                result += (occurrence.prefixWrapper ?? "") + value + (occurrence.suffixWrapper ?? "")
            }
        }
        return result + text[cursor...]
    }
}
