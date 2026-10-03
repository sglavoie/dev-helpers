import Foundation

/// A completion for an unfinished search operator or its value.
public struct SearchSuggestion: Sendable, Hashable {
    public var title: String
    public var subtitle: String
    /// The whole search text to put in the field when the suggestion is picked.
    public var completion: String

    public init(title: String, subtitle: String, completion: String) {
        self.title = title
        self.subtitle = subtitle
        self.completion = completion
    }
}

public enum SearchSuggestions {
    static let limit = 4

    static let isSuggestions: [(value: String, subtitle: String)] = [
        ("favorite", "Show only bookmarked snippets"),
        ("bookmarked", "Show only bookmarked snippets"),
        ("archived", "Show only archived snippets"),
        ("untagged", "Show only untagged snippets"),
    ]

    static let notSuggestions: [(value: String, subtitle: String)] = [
        ("archived", "Exclude archived snippets"),
        ("favorite", "Exclude bookmarked snippets"),
        ("untagged", "Exclude untagged snippets"),
    ]

    /// Up to four completions for the final token. Completed values stop
    /// suggesting so Return selects a snippet instead of another completion.
    public static func suggestions(for query: String, allTags: [String], allContexts: [String] = []) -> [SearchSuggestion] {
        var trimmed = Substring(query)
        while trimmed.last?.isWhitespace == true { trimmed.removeLast() }
        let start = trimmed.lastIndex(where: \.isWhitespace).map { trimmed.index(after: $0) } ?? trimmed.startIndex
        let prefix = String(trimmed[..<start])
        let token = String(trimmed[start...])
        // Operators inside quoted phrases are literal search text.
        guard prefix.filter({ $0 == "\"" }).count.isMultiple(of: 2), !token.contains("\"") else { return [] }
        // Check the compound operators before the generic `not:` branch.
        guard let operation = ["not:ctx:", "not:tag:", "ctx:", "tag:", "is:", "not:"]
            .first(where: { token.hasPrefix($0) }) else { return [] }
        let value = String(token.dropFirst(operation.count)).lowercased()
        // A space ends a value; retain completions for an empty operator (`ctx: `).
        guard value.isEmpty || query.last?.isWhitespace != true else { return [] }

        let candidates: [(value: String, subtitle: String)]
        switch operation {
        case "ctx:", "not:ctx:":
            candidates = allContexts.map { context in
                (context, operation == "not:ctx:"
                    ? "Exclude snippets in the \(context) context"
                    : "Filter to snippets in the \(context) context")
            }
        case "tag:", "not:tag:":
            candidates = allTags.map { tag in
                (tag, operation == "not:tag:"
                    ? "Exclude snippets tagged \(tag)" : "Filter to snippets tagged \(tag)")
            }
        case "is:": candidates = isSuggestions
        default:
            candidates = notSuggestions + allTags.map { ("tag:\($0)", "Exclude snippets tagged \($0)") }
        }
        guard !candidates.contains(where: { $0.value.lowercased() == value }) else { return [] }
        return candidates.filter { $0.value.lowercased().hasPrefix(value) }.prefix(limit).map {
            let title = operation + $0.value
            return SearchSuggestion(title: title, subtitle: $0.subtitle, completion: prefix + title + " ")
        }
    }
}
