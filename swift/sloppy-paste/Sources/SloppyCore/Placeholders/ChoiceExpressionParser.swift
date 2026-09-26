import Foundation

/// Result of parsing one trimmed `{{…}}` body for authored choices
/// (`key[one|two]`).
public enum ChoiceParseResult: Sendable, Hashable {
    case choice(ChoiceDeclaration)
    case invalidChoice(reason: String)
    /// The body has no choice intent; the legacy parser handles it.
    case legacy
}

public struct ChoiceDeclaration: Sendable, Hashable {
    public var key: String
    public var prefixWrapper: String?
    public var suffixWrapper: String?
    public var explicitDefault: String?
    public var hasExplicitDefault: Bool
    public var isSaved: Bool
    public var isRequired: Bool
    public var choices: [String]
}

/// Parses the authored-choice grammar. Works on Unicode scalars so the ASCII
/// delimiters are never merged into a grapheme with a combining mark.
public enum ChoiceExpressionParser {
    private static let supportedEscapes: Set<Unicode.Scalar> = ["|", "[", "]", "\\"]

    private static let prefixBracketReason =
        "brackets in a prefix wrapper cannot be combined with authored choices; remove the brackets from the prefix wrapper or remove the choice list"
    private static let defaultBracketReason =
        "brackets in a default value cannot be combined with authored choices; remove the brackets from the default value or remove the choice list"
    private static let suffixBracketReason =
        "brackets in a suffix wrapper cannot be combined with authored choices; remove the brackets from the suffix wrapper or remove the choice list"
    private static let danglingEscapeReason =
        "the choice list ends with a dangling escape; remove it or escape a supported character"

    public static func parse(_ trimmedContent: String) -> ChoiceParseResult {
        var bodyString = trimmedContent
        let isSaved = !bodyString.hasPrefix("!")
        if !isSaved { bodyString = String(bodyString.dropFirst()).trimmed }
        let body = Array(bodyString.unicodeScalars)
        if !hasKeyChoiceIntent(body) { return .legacy }

        var opening = -1
        var closing = -1
        var insideChoices = false
        var sawBracket = false
        var topLevelPipes: [Int] = []
        var topLevelColons: [Int] = []

        func bracketInSuffix(_ index: Int) -> Bool {
            topLevelColons.count >= 2
                && topLevelColons[0] < opening
                && closing < topLevelColons[1]
                && index > topLevelColons[1]
        }

        var index = 0
        while index < body.count {
            defer { index += 1 }
            let character = body[index]

            if insideChoices {
                if character == "\\" {
                    if index + 1 >= body.count {
                        return .invalidChoice(reason: danglingEscapeReason)
                    }
                    let escaped = body[index + 1]
                    if !supportedEscapes.contains(escaped) {
                        return .invalidChoice(
                            reason: "unsupported escape \\\(escaped); only \\|, \\[, \\], and \\\\ are supported")
                    }
                    index += 1
                    continue
                }
                if character == "[" {
                    return .invalidChoice(
                        reason: "nested choice brackets are not allowed; escape a literal bracket as \\[")
                }
                if character == "]" {
                    closing = index
                    insideChoices = false
                }
                continue
            }

            if character == "[" {
                sawBracket = true
                if opening != -1 {
                    if let firstColon = body.firstIndex(of: ":"), opening < firstColon, index > firstColon {
                        return .invalidChoice(reason: prefixBracketReason)
                    }
                    if topLevelPipes.contains(where: { $0 > closing }) {
                        return .invalidChoice(reason: defaultBracketReason)
                    }
                    if bracketInSuffix(index) {
                        return .invalidChoice(reason: suffixBracketReason)
                    }
                    return .invalidChoice(reason: "only one choice list is allowed per placeholder expression")
                }
                opening = index
                insideChoices = true
            } else if character == "]" {
                sawBracket = true
                if opening != -1 && closing != -1 {
                    if topLevelPipes.contains(where: { $0 > closing }) {
                        return .invalidChoice(reason: defaultBracketReason)
                    }
                    if bracketInSuffix(index) {
                        return .invalidChoice(reason: suffixBracketReason)
                    }
                }
                return .invalidChoice(
                    reason: "found an unmatched closing bracket; brackets are reserved in choice-capable key positions, so rename the placeholder key to remove `]`"
                )
            } else if character == "|" {
                topLevelPipes.append(index)
            } else if character == ":" {
                topLevelColons.append(index)
            }
        }

        if !sawBracket { return .legacy }
        if insideChoices || closing == -1 {
            return .invalidChoice(reason: "the choice list is missing a closing bracket `]`")
        }

        // Keep the established rightmost-default behavior, but ignore pipes
        // inside the choice list itself.
        let defaultDelimiter = topLevelPipes.last
        let coreEnd = defaultDelimiter ?? body.count
        if opening >= coreEnd || closing >= coreEnd {
            return .invalidChoice(
                reason: "the choice list must follow the placeholder key and come before the default value")
        }

        let coreColons = topLevelColons.filter { $0 < coreEnd }
        var keyStart = 0
        var keyEnd = coreEnd
        var prefixWrapper: String?
        var suffixWrapper: String?

        if coreColons.count == 2 {
            let firstColon = coreColons[0]
            let secondColon = coreColons[1]
            keyStart = firstColon + 1
            keyEnd = secondColon
            prefixWrapper = string(body[0..<firstColon]).nilIfEmpty
            suffixWrapper = string(body[(secondColon + 1)..<coreEnd]).nilIfEmpty
        } else if !coreColons.isEmpty {
            return .invalidChoice(
                reason: "wrapper syntax must contain exactly two colons: `prefix:key[one|two]:suffix`")
        }

        if opening < keyStart || closing >= keyEnd {
            return .invalidChoice(reason: "the choice list must be attached to the placeholder key, not its wrappers")
        }

        let key = string(body[keyStart..<opening]).trimmed
        let trailingKeyText = string(body[(closing + 1)..<keyEnd]).trimmed
        if key.isEmpty { return .invalidChoice(reason: "enter a placeholder key before the choice list") }
        if !trailingKeyText.isEmpty {
            return .invalidChoice(reason: "place the closing choice bracket immediately after the final choice")
        }

        let choices: [String]
        switch decodeChoices(Array(body[(opening + 1)..<closing])) {
        case .success(let decoded): choices = decoded
        case .failure(let failure): return .invalidChoice(reason: failure.reason)
        }

        let hasExplicitDefault = defaultDelimiter != nil
        let explicitDefault = defaultDelimiter.map { string(body[($0 + 1)...]).trimmed }
        let hasNonEmptyWrappers = prefixWrapper != nil || suffixWrapper != nil

        return .choice(
            ChoiceDeclaration(
                key: key,
                prefixWrapper: prefixWrapper,
                suffixWrapper: suffixWrapper,
                explicitDefault: explicitDefault,
                hasExplicitDefault: hasExplicitDefault,
                isSaved: isSaved,
                isRequired: !hasExplicitDefault && !hasNonEmptyWrappers,
                choices: choices
            ))
    }

    /// Brackets only opt into the new grammar when they occur where a key's
    /// choice list can begin. Brackets in a legacy default or suffix remain
    /// ordinary text. A balanced bracket in a prefix is also unambiguous because
    /// two wrapper colons follow it, unless the wrapped key itself contains
    /// choice intent.
    private static func hasKeyChoiceIntent(_ body: [Unicode.Scalar]) -> Bool {
        let firstOpening = body.firstIndex(of: "[")
        let firstClosing = body.firstIndex(of: "]")
        guard let firstBracket = [firstOpening, firstClosing].compactMap({ $0 }).min() else { return false }

        let beforeBracket = body[..<firstBracket]
        if beforeBracket.contains("|") || count(beforeBracket, ":") >= 2 { return false }

        // `{{pre[fix]:key:suffix}}` is a legacy wrapper whose prefix happens to
        // contain balanced brackets, not a choice list attached to `pre`.
        if let firstOpening, firstOpening == firstBracket,
            let closing = findUnescapedClosingBracket(body, from: firstOpening + 1)
        {
            let afterBracket = Array(body[(closing + 1)...].drop(while: isJSWhitespace))
            if !beforeBracket.contains(":"), afterBracket.first == ":", count(afterBracket[...], ":") >= 2 {
                let rest = afterBracket.dropFirst()
                let keyBracket = rest.firstIndex(where: { $0 == "[" || $0 == "]" })
                let nextColon = rest.firstIndex(of: ":")

                // A bracket before the second wrapper delimiter belongs to the
                // key segment. Let the choice parser diagnose the unsupported
                // combination instead of silently accepting a mangled legacy key.
                if let keyBracket, nextColon == nil || keyBracket < nextColon! { return true }
                return false
            }
        }

        return true
    }

    private static func findUnescapedClosingBracket(_ source: [Unicode.Scalar], from: Int) -> Int? {
        var index = from
        while index < source.count {
            if source[index] == "\\" {
                index += 1
            } else if source[index] == "]" {
                return index
            }
            index += 1
        }
        return nil
    }

    private struct DecodeFailure: Error {
        let reason: String
    }

    private static func decodeChoices(_ source: [Unicode.Scalar]) -> Result<[String], DecodeFailure> {
        var choices: [String] = []
        var current = String.UnicodeScalarView()

        var index = 0
        while index < source.count {
            let character = source[index]
            if character == "\\" {
                if index + 1 >= source.count {
                    return .failure(DecodeFailure(reason: danglingEscapeReason))
                }
                index += 1
                current.append(source[index])
            } else if character == "|" {
                choices.append(String(current).trimmed)
                current = String.UnicodeScalarView()
            } else {
                current.append(character)
            }
            index += 1
        }
        choices.append(String(current).trimmed)

        if choices.contains(where: \.isEmpty) {
            return .failure(DecodeFailure(reason: "choice values cannot be empty; remove the empty entry or enter a value"))
        }
        if choices.count < 2 {
            return .failure(DecodeFailure(reason: "add at least two unique choices separated by `|`"))
        }

        var seen: Set<String> = []
        for choice in choices {
            if !seen.insert(choice).inserted {
                return .failure(
                    DecodeFailure(reason: "choice \(choice.jsonQuoted) is duplicated; keep each choice unique"))
            }
        }
        return .success(choices)
    }

    private static func count(_ source: ArraySlice<Unicode.Scalar>, _ character: Unicode.Scalar) -> Int {
        source.reduce(0) { $1 == character ? $0 + 1 : $0 }
    }

    private static func string<S: Sequence<Unicode.Scalar>>(_ scalars: S) -> String {
        var view = String.UnicodeScalarView()
        view.append(contentsOf: scalars)
        return String(view)
    }

    private static func isJSWhitespace(_ scalar: Unicode.Scalar) -> Bool {
        scalar == "\u{FEFF}" || scalar.properties.isWhitespace
    }
}

extension String {
    /// JavaScript `String.prototype.trim()`: strips whitespace, line
    /// terminators and the BOM.
    var trimmed: String {
        String(
            String.UnicodeScalarView(
                unicodeScalars
                    .drop(while: { $0 == "\u{FEFF}" || $0.properties.isWhitespace })
                    .reversed()
                    .drop(while: { $0 == "\u{FEFF}" || $0.properties.isWhitespace })
                    .reversed()))
    }

    var nilIfEmpty: String? { isEmpty ? nil : self }

    /// `JSON.stringify` of a string, so diagnostics match the TS messages.
    var jsonQuoted: String {
        var result = "\""
        for scalar in unicodeScalars {
            switch scalar {
            case "\"": result += "\\\""
            case "\\": result += "\\\\"
            case "\n": result += "\\n"
            case "\r": result += "\\r"
            case "\t": result += "\\t"
            case "\u{08}": result += "\\b"
            case "\u{0C}": result += "\\f"
            case _ where scalar.value < 0x20:
                result += String(format: "\\u%04x", scalar.value)
            default:
                result.unicodeScalars.append(scalar)
            }
        }
        return result + "\""
    }
}
