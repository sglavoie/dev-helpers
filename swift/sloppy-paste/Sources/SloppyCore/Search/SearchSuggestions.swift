import Foundation

/// An operator completion shown while the search field ends in `tag:`, `ctx:`, `is:` or `not:`.
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

    /// Up to four completions for the operator the query ends with, or none.
    public static func suggestions(for query: String, allTags: [String], allContexts: [String] = []) -> [SearchSuggestion] {
        var trimmed = Substring(query)
        while trimmed.last?.isWhitespace == true { trimmed.removeLast() }
        let text = String(trimmed)

        // `ctx:` goes first so `not:ctx:` isn't taken by the generic `not:` branch.
        if let prefix = text.dropping(suffix: "ctx:") {
            let negated = prefix.hasSuffix("not:")
            return allContexts.prefix(limit).map { context in
                SearchSuggestion(
                    title: "\(negated ? "not:" : "")ctx:\(context)",
                    subtitle: negated
                        ? "Exclude snippets in the \(context) context"
                        : "Filter to snippets in the \(context) context",
                    completion: "\(prefix)ctx:\(context) ")
            }
        }

        if let prefix = text.dropping(suffix: "tag:") {
            return allTags.prefix(limit).map { tag in
                SearchSuggestion(
                    title: "tag:\(tag)", subtitle: "Filter to snippets tagged \(tag)", completion: "\(prefix)tag:\(tag) ")
            }
        }

        if let prefix = text.dropping(operatorToken: "is:") {
            return isSuggestions.prefix(limit).map {
                SearchSuggestion(title: "is:\($0.value)", subtitle: $0.subtitle, completion: "\(prefix)is:\($0.value) ")
            }
        }

        if let prefix = text.dropping(operatorToken: "not:") {
            var items = notSuggestions.map {
                SearchSuggestion(title: "not:\($0.value)", subtitle: $0.subtitle, completion: "\(prefix)not:\($0.value) ")
            }
            if let tag = allTags.first {
                items.append(SearchSuggestion(
                    title: "not:tag:\(tag)", subtitle: "Exclude snippets tagged \(tag)",
                    completion: "\(prefix)not:tag:\(tag) "))
            }
            return Array(items.prefix(limit))
        }

        return []
    }
}

extension String {
    /// The text before `suffix`, or nil when it doesn't end with it.
    fileprivate func dropping(suffix: String) -> String? {
        hasSuffix(suffix) ? String(dropLast(suffix.count)) : nil
    }

    /// Like `dropping(suffix:)`, but only when `token` is the whole last word.
    fileprivate func dropping(operatorToken token: String) -> String? {
        guard let prefix = dropping(suffix: token), prefix.isEmpty || prefix.hasSuffix(" ") else { return nil }
        return prefix
    }
}
