import Foundation

/// A source range in UTF-16 offsets (`start` inclusive, `end` exclusive), the
/// same units as the TS offsets and `NSRange`.
public struct PlaceholderSourceRange: Sendable, Hashable {
    public var start: Int
    public var end: Int

    public init(start: Int, end: Int) {
        self.start = start
        self.end = end
    }
}

public struct ParsedValuePlaceholderOccurrence: Sendable, Hashable {
    /// The complete authored expression, including braces.
    public var raw: String
    public var range: PlaceholderSourceRange
    /// The same range as `String.Index` bounds into the parsed text.
    public var indices: Range<String.Index>
    public var key: String
    public var prefixWrapper: String?
    public var suffixWrapper: String?
    /// Present when the expression has a trailing `|default`, including an empty default.
    public var explicitDefault: String?
    public var hasExplicitDefault: Bool
    public var isSaved: Bool
    public var isRequired: Bool
    public var choices: [String]?
    public var isChoiceDeclaration: Bool
}

public struct PlaceholderSyntaxDiagnostic: Sendable, Hashable {
    public var range: PlaceholderSourceRange
    /// The complete offending expression, including braces.
    public var expression: String
    public var message: String
}

public struct PlaceholderSyntaxResult: Sendable, Hashable {
    public var occurrences: [ParsedValuePlaceholderOccurrence]
    public var placeholders: [Placeholder]
    public var diagnostics: [PlaceholderSyntaxDiagnostic]
}

public enum PlaceholderSyntaxParser {
    private static let controlExpressions: Set<String> = ["#else", "/else", "/if"]

    /// Parses all user-value placeholder syntax in one pass.
    ///
    /// Invalid authored-choice expressions receive diagnostics but deliberately
    /// use the legacy parser for their runtime occurrence. This keeps imported or
    /// clipboard-saved malformed snippets safe to run while editor validation can
    /// reject the same content.
    public static func parse(_ text: String) -> PlaceholderSyntaxResult {
        var occurrences: [ParsedValuePlaceholderOccurrence] = []
        var diagnostics: [PlaceholderSyntaxDiagnostic] = []

        for expression in scanExpressions(text) {
            let trimmed = expression.content.trimmed
            if trimmed.hasPrefix("#if ") || controlExpressions.contains(trimmed) { continue }

            let occurrence: ParsedValuePlaceholderOccurrence
            switch ChoiceExpressionParser.parse(trimmed) {
            case .choice(let choice):
                occurrence = ParsedValuePlaceholderOccurrence(
                    raw: expression.raw,
                    range: expression.range,
                    indices: expression.indices,
                    key: choice.key,
                    prefixWrapper: choice.prefixWrapper,
                    suffixWrapper: choice.suffixWrapper,
                    explicitDefault: choice.explicitDefault,
                    hasExplicitDefault: choice.hasExplicitDefault,
                    isSaved: choice.isSaved,
                    isRequired: choice.isRequired,
                    choices: choice.choices,
                    isChoiceDeclaration: true
                )
            case .invalidChoice(let reason):
                diagnostics.append(
                    PlaceholderSyntaxDiagnostic(
                        range: expression.range,
                        expression: expression.raw,
                        message: "Invalid authored choices in \(expression.raw): \(reason)"
                    ))
                occurrence = parseLegacyOccurrence(expression)
            case .legacy:
                occurrence = parseLegacyOccurrence(expression)
            }

            if !occurrence.key.isEmpty { occurrences.append(occurrence) }
        }

        diagnostics += PlaceholderAggregator.findChoiceConflicts(occurrences)
        diagnostics = diagnostics.stableSorted {
            ($0.range.start, $0.range.end) < ($1.range.start, $1.range.end)
        }

        return PlaceholderSyntaxResult(
            occurrences: occurrences,
            placeholders: PlaceholderAggregator.aggregate(text, occurrences: occurrences),
            diagnostics: diagnostics
        )
    }

    /// Unique placeholders in form order (`extractPlaceholders` in TS).
    public static func extractPlaceholders(_ text: String) -> [Placeholder] {
        parse(text).placeholders
    }

    struct ScannedExpression {
        var raw: String
        var range: PlaceholderSourceRange
        var indices: Range<String.Index>
        var content: String
    }

    /// Finds `{{…}}` expressions like the TS `/\{\{([^}]+)\}\}/g` scan.
    static func scanExpressions(_ text: String) -> [ScannedExpression] {
        let regex = /\{\{([^}]+)\}\}/.matchingSemantics(.unicodeScalar)
        return text.matches(of: regex).map { match in
            let start = text.utf16.distance(from: text.startIndex, to: match.range.lowerBound)
            let length = text.utf16.distance(from: match.range.lowerBound, to: match.range.upperBound)
            return ScannedExpression(
                raw: String(match.output.0),
                range: PlaceholderSourceRange(start: start, end: start + length),
                indices: match.range,
                content: String(match.output.1)
            )
        }
    }

    private static func parseLegacyOccurrence(_ expression: ScannedExpression) -> ParsedValuePlaceholderOccurrence {
        var content = expression.content.trimmed
        let isSaved = !content.hasPrefix("!")
        if !isSaved { content = String(content.dropFirst()).trimmed }

        let pipeIndex = content.lastIndex(of: "|")
        let hasExplicitDefault = pipeIndex != nil
        let explicitDefault = pipeIndex.map { String(content[content.index(after: $0)...]).trimmed }
        let coreContent = pipeIndex.map { String(content[..<$0]).trimmed } ?? content
        let parts = coreContent.split(separator: ":", omittingEmptySubsequences: false).map(String.init)

        let key: String
        var prefixWrapper: String?
        var suffixWrapper: String?

        if parts.count == 1 {
            key = parts[0].trimmed
        } else if parts.count == 3 {
            prefixWrapper = parts[0].nilIfEmpty
            key = parts[1].trimmed
            suffixWrapper = parts[2].nilIfEmpty
        } else {
            key = coreContent
        }

        let hasNonEmptyWrappers = prefixWrapper != nil || suffixWrapper != nil
        return ParsedValuePlaceholderOccurrence(
            raw: expression.raw,
            range: expression.range,
            indices: expression.indices,
            key: key,
            prefixWrapper: prefixWrapper,
            suffixWrapper: suffixWrapper,
            explicitDefault: explicitDefault,
            hasExplicitDefault: hasExplicitDefault,
            isSaved: isSaved,
            isRequired: !hasExplicitDefault && !hasNonEmptyWrappers,
            isChoiceDeclaration: false
        )
    }
}
