import Foundation

/// What a highlighted piece of `{{…}}` syntax is, for coloring.
public enum PlaceholderSyntaxKind: Sendable, Hashable {
    /// `{{` and `}}`.
    case brace
    /// `#if`, `#else`, `/else` and `/if`.
    case control
    /// A value placeholder's key, or a conditional block's guard key.
    case key
    /// A built-in `DATE`, `TIME`, … placeholder.
    case system
    /// `!`, `+`, `|`, `:`, `[` and `]`.
    case punctuation
    /// Defaults, wrappers, choices and conditional labels.
    case literal
}

public struct PlaceholderSyntaxSpan: Sendable, Hashable {
    public var range: PlaceholderSourceRange
    public var kind: PlaceholderSyntaxKind

    public init(_ start: Int, _ end: Int, _ kind: PlaceholderSyntaxKind) {
        range = PlaceholderSourceRange(start: start, end: end)
        self.kind = kind
    }
}

/// Splits snippet content into colorable placeholder-syntax spans for display.
/// Purely lexical: it follows the grammar closely enough to color it, but it
/// never decides how a snippet runs, so malformed input still gets colored.
public enum PlaceholderHighlighter {
    /// Non-overlapping spans in UTF-16 offsets, in document order. Text outside
    /// `{{…}}` expressions is not covered.
    public static func spans(_ text: String) -> [PlaceholderSyntaxSpan] {
        var spans: [PlaceholderSyntaxSpan] = []
        for expression in PlaceholderSyntaxParser.scanExpressions(text) {
            let start = expression.range.start
            let end = expression.range.end
            spans.append(PlaceholderSyntaxSpan(start, start + 2, .brace))
            spans += body(Array(expression.content.utf16), offset: start + 2)
            spans.append(PlaceholderSyntaxSpan(end - 2, end, .brace))
        }
        return spans.filter { $0.range.start < $0.range.end }
    }

    // Every delimiter is ASCII, so scanning UTF-16 code units never splits a
    // surrogate pair on a match.
    private static let space = UInt16(UInt8(ascii: " "))
    private static let bang = UInt16(UInt8(ascii: "!"))
    private static let plus = UInt16(UInt8(ascii: "+"))
    private static let pipe = UInt16(UInt8(ascii: "|"))
    private static let colon = UInt16(UInt8(ascii: ":"))
    private static let open = UInt16(UInt8(ascii: "["))
    private static let close = UInt16(UInt8(ascii: "]"))
    private static let backslash = UInt16(UInt8(ascii: "\\"))
    private static let quote = UInt16(UInt8(ascii: "\""))

    private static func isSpace(_ unit: UInt16) -> Bool {
        unit == space || unit == 9 || unit == 10 || unit == 13
    }

    private static func body(_ units: [UInt16], offset: Int) -> [PlaceholderSyntaxSpan] {
        var lower = 0
        var upper = units.count
        while lower < upper, isSpace(units[lower]) { lower += 1 }
        while upper > lower, isSpace(units[upper - 1]) { upper -= 1 }
        let trimmed = String(decoding: units[lower..<upper], as: UTF16.self)

        if ["#else", "/else", "/if"].contains(trimmed) {
            return [PlaceholderSyntaxSpan(offset + lower, offset + upper, .control)]
        }
        if SystemPlaceholders.names.contains(trimmed) {
            return [PlaceholderSyntaxSpan(offset + lower, offset + upper, .system)]
        }
        if trimmed.hasPrefix("#if"), upper - lower > 3, isSpace(units[lower + 3]) {
            return condition(units, lower: lower, upper: upper, offset: offset)
        }
        return value(units, lower: lower, upper: upper, offset: offset)
    }

    /// `#if +key "Label"`.
    private static func condition(_ units: [UInt16], lower: Int, upper: Int, offset: Int) -> [PlaceholderSyntaxSpan] {
        var spans = [PlaceholderSyntaxSpan(offset + lower, offset + lower + 3, .control)]
        var index = lower + 3
        while index < upper, isSpace(units[index]) { index += 1 }
        if index < upper, units[index] == plus {
            spans.append(PlaceholderSyntaxSpan(offset + index, offset + index + 1, .punctuation))
            index += 1
        }
        let keyStart = index
        while index < upper, !isSpace(units[index]), units[index] != quote { index += 1 }
        spans.append(PlaceholderSyntaxSpan(offset + keyStart, offset + index, .key))
        while index < upper, isSpace(units[index]) { index += 1 }
        if index < upper {
            spans.append(PlaceholderSyntaxSpan(offset + index, offset + upper, .literal))
        }
        return spans
    }

    /// `!prefix:key[a|b]:suffix|default`.
    private static func value(_ units: [UInt16], lower: Int, upper: Int, offset: Int) -> [PlaceholderSyntaxSpan] {
        var spans: [PlaceholderSyntaxSpan] = []
        var start = lower
        if start < upper, units[start] == bang {
            spans.append(PlaceholderSyntaxSpan(offset + start, offset + start + 1, .punctuation))
            start += 1
        }

        // Top-level `|` and `:` positions, skipping anything inside `[…]`.
        var pipes: [Int] = []
        var colons: [Int] = []
        var depth = 0
        var index = start
        while index < upper {
            let unit = units[index]
            if depth > 0, unit == backslash {
                index += 2
                continue
            }
            if unit == open { depth += 1 }
            if unit == close, depth > 0 { depth -= 1 }
            if depth == 0, unit == pipe { pipes.append(index) }
            if depth == 0, unit == colon { colons.append(index) }
            index += 1
        }

        var coreEnd = upper
        if let defaultPipe = pipes.last {
            coreEnd = defaultPipe
            colons.removeAll { $0 > defaultPipe }
            spans.append(PlaceholderSyntaxSpan(offset + defaultPipe, offset + defaultPipe + 1, .punctuation))
            spans.append(PlaceholderSyntaxSpan(offset + defaultPipe + 1, offset + upper, .literal))
        }

        if colons.count == 2 {
            let (first, second) = (colons[0], colons[1])
            spans.append(PlaceholderSyntaxSpan(offset + start, offset + first, .literal))
            spans.append(PlaceholderSyntaxSpan(offset + first, offset + first + 1, .punctuation))
            spans += key(units, lower: first + 1, upper: second, offset: offset)
            spans.append(PlaceholderSyntaxSpan(offset + second, offset + second + 1, .punctuation))
            spans.append(PlaceholderSyntaxSpan(offset + second + 1, offset + coreEnd, .literal))
        } else {
            spans += key(units, lower: start, upper: coreEnd, offset: offset)
        }
        return spans.sorted { $0.range.start < $1.range.start }
    }

    /// `key` or `key[a|b]`.
    private static func key(_ units: [UInt16], lower: Int, upper: Int, offset: Int) -> [PlaceholderSyntaxSpan] {
        guard let bracket = units[lower..<upper].firstIndex(of: open) else {
            return [PlaceholderSyntaxSpan(offset + lower, offset + upper, .key)]
        }
        var spans = [
            PlaceholderSyntaxSpan(offset + lower, offset + bracket, .key),
            PlaceholderSyntaxSpan(offset + bracket, offset + bracket + 1, .punctuation),
        ]
        var choiceStart = bracket + 1
        var index = bracket + 1
        while index < upper {
            let unit = units[index]
            if unit == backslash {
                index += 2
                continue
            }
            if unit == pipe || unit == close {
                spans.append(PlaceholderSyntaxSpan(offset + choiceStart, offset + index, .literal))
                spans.append(PlaceholderSyntaxSpan(offset + index, offset + index + 1, .punctuation))
                choiceStart = index + 1
            }
            index += 1
        }
        spans.append(PlaceholderSyntaxSpan(offset + choiceStart, offset + upper, .literal))
        return spans
    }
}
