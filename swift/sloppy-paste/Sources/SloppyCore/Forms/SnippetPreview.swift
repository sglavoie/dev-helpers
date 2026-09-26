import Foundation

/// The authoring preview shown in the snippet editor: placeholder syntax is
/// annotated as `⟨…⟩` markers rather than filled in.
public enum SnippetPreview {
    static let maxPreviewLength = 500

    /// Empty when `content` has no `{{`; otherwise system placeholders, then
    /// conditional blocks, then value placeholders, capped at 500 UTF-16 units
    /// (without splitting a character).
    public static func build(_ content: String, now: Int64, timeZone: TimeZone = .current) -> String {
        guard content.contains("{{") else { return "" }

        let withSystems = previewSystemPlaceholders(content, now: now, timeZone: timeZone)
        let withConditionals = previewConditionalBlocks(withSystems)
        return capped(previewValuePlaceholders(withConditionals))
    }

    private static func previewSystemPlaceholders(_ content: String, now: Int64, timeZone: TimeZone) -> String {
        var result = content
        for name in SystemPlaceholders.names {
            guard let resolved = SystemPlaceholders.value(for: name, now: now, timeZone: timeZone) else { continue }
            result = result.replacingOccurrences(of: "{{\(name)}}", with: "⟨\(name) → \(resolved)⟩")
        }
        return result
    }

    private static func previewConditionalBlocks(_ content: String) -> String {
        let block = /\{\{#if ([^}]+)\}\}([\s\S]*?)\{\{\/if\}\}/.matchingSemantics(.unicodeScalar)
        return content.replacing(block) { match in
            var displayKey = String(match.1).trimmed
                .replacing(/\s+"[^"]*"$/.matchingSemantics(.unicodeScalar), with: "")
            if displayKey.hasPrefix("+") { displayKey.removeFirst() }
            let cleanBody = String(match.2)
                .replacingOccurrences(of: "{{#else}}", with: " | else: ")
                .replacingOccurrences(of: "{{/else}}", with: "")
            return "⟨if \(displayKey): \(cleanBody.trimmed)⟩"
        }
    }

    private static func previewValuePlaceholders(_ content: String) -> String {
        var result = ""
        var cursor = content.startIndex
        for occurrence in PlaceholderSyntaxParser.parse(content).occurrences {
            result += content[cursor..<occurrence.indices.lowerBound]
            result += previewOccurrence(occurrence)
            cursor = occurrence.indices.upperBound
        }
        return result + content[cursor...]
    }

    private static func previewOccurrence(_ occurrence: ParsedValuePlaceholderOccurrence) -> String {
        let label = displayKey(occurrence)

        if let choices = occurrence.choices {
            let list = choices.map(\.jsonQuoted).joined(separator: ", ")
            let defaultLabel =
                occurrence.hasExplicitDefault ? "; default: \((occurrence.explicitDefault ?? "").jsonQuoted)" : ""
            return "⟨\(label) — choices: \(list)\(defaultLabel)⟩"
        }

        if occurrence.hasExplicitDefault, let explicitDefault = occurrence.explicitDefault, !explicitDefault.isEmpty {
            return "⟨\(label) = \(explicitDefault.jsonQuoted)⟩"
        }
        return "⟨\(label)⟩"
    }

    private static func displayKey(_ occurrence: ParsedValuePlaceholderOccurrence) -> String {
        let prefix = occurrence.prefixWrapper?.trimmed ?? ""
        let suffix = occurrence.suffixWrapper?.trimmed ?? ""
        let main = prefix + occurrence.key
        let wrapped = suffix.isEmpty ? main : "\(main) \(suffix)"
        return occurrence.isSaved ? wrapped : "!\(wrapped)"
    }

    private static func capped(_ text: String) -> String {
        var units = 0
        var end = text.startIndex
        for index in text.indices {
            let width = text[index].utf16.count
            guard units + width <= maxPreviewLength else { break }
            units += width
            end = text.index(after: index)
        }
        return units == text.utf16.count ? text : String(text[..<end])
    }
}
