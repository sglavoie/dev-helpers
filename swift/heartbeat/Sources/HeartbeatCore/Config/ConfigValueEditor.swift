import Foundation

public enum ConfigEditError: Error, Equatable, Sendable, CustomStringConvertible {
    case keyNotFound(String)
    case malformed(String)
    case verificationFailed(String)

    public var description: String {
        switch self {
        case .keyNotFound(let path): "\(path) is not set in the config file"
        case .malformed(let reason): "cannot follow the config's JSON5: \(reason)"
        case .verificationFailed(let reason): "the edited config did not check out: \(reason)"
        }
    }
}

/// Replaces one number in `config.json` in place, so its comments and layout survive.
public enum ConfigValueEditor {
    /// Sets `agents.<label>.maxAgeSeconds`. Only an existing key is changed; none is ever added.
    public static func replaceAgentMaxAge(in text: String, label: String, value: Int) throws -> String {
        let path = ["agents", label, "maxAgeSeconds"]
        let tokens = try JSON5Tokenizer(text).tokens()
        var walker = Walker(tokens: tokens, target: path)
        try walker.value(path: [])
        guard walker.index == tokens.count else { throw ConfigEditError.malformed("text after the top-level value") }
        guard !walker.matches.isEmpty else { throw ConfigEditError.keyNotFound(path.joined(separator: ".")) }

        var output = text
        for range in walker.matches.sorted(by: { $0.lowerBound > $1.lowerBound }) {
            output.replaceSubrange(range, with: String(value))
        }

        do {
            let (config, _) = try ConfigLoader.parse(Data(output.utf8))
            guard config.agents[label]?.maxAgeSeconds == value else {
                throw ConfigEditError.verificationFailed("maxAgeSeconds reads back as \(config.agents[label]?.maxAgeSeconds.map(String.init) ?? "unset")")
            }
        } catch let error as ConfigEditError {
            throw error
        } catch {
            throw ConfigEditError.verificationFailed("\(error)")
        }
        return output
    }

    /// Follows objects and arrays and records the range of each number found at `target`.
    struct Walker {
        let tokens: [JSON5Tokenizer.Token]
        let target: [String]
        var index = 0
        var matches: [Range<String.Index>] = []

        init(tokens: [JSON5Tokenizer.Token], target: [String]) {
            self.tokens = tokens
            self.target = target
        }

        mutating func next() throws -> JSON5Tokenizer.Token {
            guard index < tokens.count else { throw ConfigEditError.malformed("unexpected end") }
            defer { index += 1 }
            return tokens[index]
        }

        func peek() -> JSON5Tokenizer.Token? {
            index < tokens.count ? tokens[index] : nil
        }

        mutating func value(path: [String]) throws {
            let token = try next()
            switch token.kind {
            case .punctuation("{"):
                while true {
                    if case .punctuation("}") = peek()?.kind { index += 1; return }
                    let key = try next()
                    let name: String
                    switch key.kind {
                    case .string(let text), .bare(let text): name = text
                    default: throw ConfigEditError.malformed("expected a key")
                    }
                    guard case .punctuation(":") = try next().kind else { throw ConfigEditError.malformed("expected ':' after \(name)") }
                    try value(path: path + [name])
                    let after = try next()
                    if case .punctuation(",") = after.kind { continue }
                    if case .punctuation("}") = after.kind { return }
                    throw ConfigEditError.malformed("expected ',' or '}' after \(name)")
                }
            case .punctuation("["):
                while true {
                    if case .punctuation("]") = peek()?.kind { index += 1; return }
                    try value(path: path + ["[]"])
                    let after = try next()
                    if case .punctuation(",") = after.kind { continue }
                    if case .punctuation("]") = after.kind { return }
                    throw ConfigEditError.malformed("expected ',' or ']'")
                }
            case .bare(let text):
                if path == target, Int(text) != nil || Double(text) != nil { matches.append(token.range) }
            case .string:
                break
            case .punctuation(let mark):
                throw ConfigEditError.malformed("unexpected '\(mark)'")
            }
        }
    }
}

/// Splits JSON5 into strings, bare words (numbers, identifiers, literals) and punctuation, skipping comments.
struct JSON5Tokenizer {
    struct Token {
        enum Kind: Equatable {
            case punctuation(Character)
            case string(String)
            case bare(String)
        }
        var kind: Kind
        var range: Range<String.Index>
    }

    let text: String
    init(_ text: String) { self.text = text }

    func tokens() throws -> [Token] {
        var tokens: [Token] = []
        var position = text.startIndex
        while position < text.endIndex {
            let character = text[position]
            let rest = text[position...]
            if character.isWhitespace {
                position = text.index(after: position)
            } else if rest.hasPrefix("//") {
                position = rest.firstIndex { $0 == "\n" || $0 == "\r\n" } ?? text.endIndex
            } else if rest.hasPrefix("/*") {
                guard let end = rest.range(of: "*/") else { throw ConfigEditError.malformed("unclosed comment") }
                position = end.upperBound
            } else if "{}[]:,".contains(character) {
                let end = text.index(after: position)
                tokens.append(Token(kind: .punctuation(character), range: position..<end))
                position = end
            } else if character == "\"" || character == "'" {
                var value = ""
                var cursor = text.index(after: position)
                while true {
                    guard cursor < text.endIndex else { throw ConfigEditError.malformed("unclosed string") }
                    let next = text[cursor]
                    cursor = text.index(after: cursor)
                    if next == character { break }
                    if next == "\\" {
                        guard cursor < text.endIndex else { throw ConfigEditError.malformed("unclosed string") }
                        value.append(text[cursor])
                        cursor = text.index(after: cursor)
                    } else {
                        value.append(next)
                    }
                }
                tokens.append(Token(kind: .string(value), range: position..<cursor))
                position = cursor
            } else {
                var cursor = position
                while cursor < text.endIndex, !text[cursor].isWhitespace, !"{}[]:,\"'".contains(text[cursor]),
                      !text[cursor...].hasPrefix("//"), !text[cursor...].hasPrefix("/*") {
                    cursor = text.index(after: cursor)
                }
                tokens.append(Token(kind: .bare(String(text[position..<cursor])), range: position..<cursor))
                position = cursor
            }
        }
        return tokens
    }
}
