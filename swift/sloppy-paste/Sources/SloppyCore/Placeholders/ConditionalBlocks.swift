import Foundation

/// The branches of one `{{#if key}}…{{/if}}` block, as authored.
public struct ConditionalBlockBody: Sendable, Hashable {
    public var ifBody: String
    public var elseBody: String?

    public init(ifBody: String, elseBody: String? = nil) {
        self.ifBody = ifBody
        self.elseBody = elseBody
    }
}

/// `{{#if key}}…{{#else}}…{{/else}}{{/if}}` blocks. `{{/else}}` is optional,
/// the key may carry a `+` default-on prefix and a trailing `"Label"`.
///
/// All matching uses Unicode-scalar semantics so a combining mark or emoji
/// modifier next to a tag can never hide it, matching the JS UTF-16 regexes.
public enum ConditionalBlocks {
    private static let maxIterations = 10

    /// Resolves every block against `values`, innermost first (up to 10
    /// nesting levels). A key is truthy when its value is non-blank; guard-only
    /// keys use `"true"` and `""`. One leading and one trailing newline of the
    /// selected body are dropped, so a block on its own lines leaves no blank
    /// line behind. Placeholders inside the chosen body are left for
    /// `PlaceholderRenderer.replacePlaceholders`.
    public static func process(_ text: String, values: [String: String]) -> String {
        // Bodies exclude any nested `{{#if `, so only leaf blocks match.
        let leaf = /\{\{#if ([^}]+)\}\}((?:(?!\{\{#if\s)[\s\S])*?)(?:\{\{#else\}\}((?:(?!\{\{#if\s)[\s\S])*?)(?:\{\{\/else\}\})?)?\{\{\/if\}\}/
            .matchingSemantics(.unicodeScalar)

        var result = text
        for _ in 0..<maxIterations {
            var foundLeaf = false
            result = result.replacing(leaf) { match in
                foundLeaf = true
                let value = values[conditionKey(String(match.output.1))] ?? ""
                let body = value.trimmed.isEmpty ? match.output.3.map(String.init) ?? "" : String(match.output.2)
                return stripOuterNewlines(body)
            }
            if !foundLeaf { break }
        }
        return result
    }

    /// The top-level bodies of every block guarded by `key`, in document
    /// order, for field previews. Nested blocks stay inside their parent body;
    /// unterminated blocks are skipped.
    public static func extractBodies(_ text: String, key: String) -> [ConditionalBlockBody] {
        let open = /\{\{#if\s+\+?([^\s}"]+)(?:\s+"[^"]*")?\s*\}\}/.matchingSemantics(.unicodeScalar)
        var results: [ConditionalBlockBody] = []
        var searchStart = text.startIndex

        while searchStart < text.endIndex, let match = text[searchStart...].firstMatch(of: open) {
            searchStart = match.range.upperBound
            guard String(match.output.1).trimmed == key,
                  let close = matchingClose(in: text, from: match.range.upperBound)
            else { continue }

            results.append(splitTopLevelElse(String(text[match.range.upperBound..<close.lowerBound])))
            searchStart = close.upperBound
        }
        return results
    }

    /// `rawKey.trim()` minus a trailing `"Label"` and a leading `+`.
    static func conditionKey(_ rawKey: String) -> String {
        var key = rawKey.trimmed.replacing(/\s+"[^"]*"$/.matchingSemantics(.unicodeScalar), with: "")
        if key.unicodeScalars.first == "+" { key.unicodeScalars.removeFirst() }
        return key
    }

    /// JS `.replace(/^\n/, "").replace(/\n$/, "")`, on scalars so a trailing
    /// `\r\n` loses only its `\n`.
    private static func stripOuterNewlines(_ body: String) -> String {
        var scalars = body.unicodeScalars
        if scalars.first == "\n" { scalars.removeFirst() }
        if scalars.last == "\n" { scalars.removeLast() }
        return String(scalars)
    }

    private static func matchingClose(in text: String, from start: String.Index) -> Range<String.Index>? {
        let token = /\{\{#if\s|\{\{\/if\}\}/.matchingSemantics(.unicodeScalar)
        var depth = 1
        for match in text[start...].matches(of: token) {
            if match.output == "{{/if}}" {
                depth -= 1
                if depth == 0 { return match.range }
            } else {
                depth += 1
            }
        }
        return nil
    }

    private static func splitTopLevelElse(_ body: String) -> ConditionalBlockBody {
        let token = /\{\{#if\s|\{\{\/if\}\}|\{\{#else\}\}/.matchingSemantics(.unicodeScalar)
        var depth = 0
        for match in body.matches(of: token) {
            switch match.output {
            case "{{#else}}" where depth == 0:
                var elseBody = String(body[match.range.upperBound...])
                if elseBody.hasSuffix("{{/else}}") { elseBody.unicodeScalars.removeLast(9) }
                return ConditionalBlockBody(ifBody: String(body[..<match.range.lowerBound]), elseBody: elseBody)
            case "{{#else}}": break
            case "{{/if}}": depth -= 1
            default: depth += 1
            }
        }
        return ConditionalBlockBody(ifBody: body)
    }
}
