import Foundation

/// Final content ready for the clipboard, plus the values to track with the use.
public struct PreparedSnippet: Sendable, Hashable {
    public var content: String
    public var placeholderValues: [PlaceholderValueToRecord]

    public init(content: String, placeholderValues: [PlaceholderValueToRecord] = []) {
        self.content = content
        self.placeholderValues = placeholderValues
    }
}

/// A snippet whose system placeholders are already resolved, and the user
/// placeholders the form must ask for.
public struct PlaceholderFormRequest: Sendable, Hashable {
    public var snippet: Snippet
    public var placeholders: [Placeholder]

    public init(snippet: Snippet, placeholders: [Placeholder]) {
        self.snippet = snippet
        self.placeholders = placeholders
    }
}

public struct MissingPlaceholderHistoryError: Error, Sendable, Hashable, CustomStringConvertible {
    public var placeholderKey: String

    public init(placeholderKey: String) {
        self.placeholderKey = placeholderKey
    }

    public var description: String { "No history for {{\(placeholderKey)}}" }
}

/// Turns a stored snippet into clipboard content: directly, through the
/// placeholder form, or with the last-used values.
public enum SnippetPreparation {
    public enum Plan: Sendable, Hashable {
        /// No user placeholders: the content is final.
        case direct(PreparedSnippet)
        case form(PlaceholderFormRequest)
    }

    public enum LastValuesAvailability: Sendable, Hashable {
        /// The snippet has no required user placeholders, so the action is hidden.
        case notApplicable
        /// Some required key has no history yet.
        case unavailable
        case available
    }

    /// Resolves system placeholders, then either finishes the content or asks for a form.
    public static func plan(for snippet: Snippet, now: Int64, timeZone: TimeZone = .current) -> Plan {
        let processed = SystemPlaceholders.process(snippet.content, now: now, timeZone: timeZone)
        let placeholders = PlaceholderSyntaxParser.extractPlaceholders(processed)
        guard placeholders.isEmpty else {
            var formSnippet = snippet
            formSnippet.content = processed
            return .form(PlaceholderFormRequest(snippet: formSnippet, placeholders: placeholders))
        }
        return .direct(PreparedSnippet(content: ConditionalBlocks.process(processed, values: [:])))
    }

    /// Renders already system-processed content with the form's final values.
    public static func submission(
        content: String,
        placeholders: [Placeholder],
        finalValues: [String: String]
    ) -> PreparedSnippet {
        let afterBlocks = ConditionalBlocks.process(content, values: finalValues)
        return PreparedSnippet(
            content: PlaceholderRenderer.replacePlaceholders(afterBlocks, values: finalValues, placeholders: placeholders),
            placeholderValues: trackedValues(placeholders, finalValues: finalValues)
        )
    }

    /// Every placeholder with its submitted value and save flag; the repository
    /// applies the no-save and blank exclusions when recording.
    public static func trackedValues(
        _ placeholders: [Placeholder],
        finalValues: [String: String]
    ) -> [PlaceholderValueToRecord] {
        placeholders.map { placeholder in
            PlaceholderValueToRecord(
                key: placeholder.key,
                value: finalValues[placeholder.key] ?? "",
                isSaved: placeholder.isSaved
            )
        }
    }

    /// Required user keys (system names excluded) after resolving system placeholders.
    public static func requiredUserKeys(in content: String, now: Int64, timeZone: TimeZone = .current) -> [String] {
        let systemKeys = Set(SystemPlaceholders.names)
        let processed = SystemPlaceholders.process(content, now: now, timeZone: timeZone)
        return PlaceholderSyntaxParser.extractPlaceholders(processed)
            .filter { $0.isRequired && !systemKeys.contains($0.key) }
            .map(\.key)
    }

    public static func lastValuesAvailability(
        for snippet: Snippet,
        history: PlaceholderHistory,
        now: Int64,
        timeZone: TimeZone = .current
    ) -> LastValuesAvailability {
        let required = requiredUserKeys(in: snippet.content, now: now, timeZone: timeZone)
        if required.isEmpty { return .notApplicable }
        return required.allSatisfy { hasLastUsedValue(history, key: $0) } ? .available : .unavailable
    }

    /// IDs of snippets with required user keys that all have a last-used value.
    public static func historyAvailability(
        for snippets: [Snippet],
        history: PlaceholderHistory,
        now: Int64,
        timeZone: TimeZone = .current
    ) -> Set<String> {
        var result = Set<String>()
        for snippet in snippets
        where lastValuesAvailability(for: snippet, history: history, now: now, timeZone: timeZone) == .available {
            result.insert(snippet.id)
        }
        return result
    }

    /// "Paste with Last Values": required keys take their last-used value and
    /// optional keys with a default take the default.
    public static func prepareWithLastValues(
        _ snippet: Snippet,
        history: PlaceholderHistory,
        now: Int64,
        timeZone: TimeZone = .current
    ) throws(MissingPlaceholderHistoryError) -> PreparedSnippet {
        let systemKeys = Set(SystemPlaceholders.names)
        let processed = SystemPlaceholders.process(snippet.content, now: now, timeZone: timeZone)
        let placeholders = PlaceholderSyntaxParser.extractPlaceholders(processed)
        var finalValues: [String: String] = [:]

        for placeholder in placeholders where placeholder.isRequired && !systemKeys.contains(placeholder.key) {
            guard let lastValue = PlaceholderHistoryRanking.lastUsedValue(history[placeholder.key] ?? []) else {
                throw MissingPlaceholderHistoryError(placeholderKey: placeholder.key)
            }
            finalValues[placeholder.key] = lastValue
        }
        for placeholder in placeholders where !placeholder.isRequired && finalValues[placeholder.key] == nil {
            if let defaultValue = placeholder.defaultValue { finalValues[placeholder.key] = defaultValue }
        }

        let userPlaceholders = placeholders.filter { !systemKeys.contains($0.key) }
        let afterBlocks = ConditionalBlocks.process(processed, values: finalValues)
        return PreparedSnippet(
            content: PlaceholderRenderer.replacePlaceholders(afterBlocks, values: finalValues, placeholders: placeholders),
            placeholderValues: trackedValues(userPlaceholders, finalValues: finalValues)
        )
    }

    private static func hasLastUsedValue(_ history: PlaceholderHistory, key: String) -> Bool {
        !(PlaceholderHistoryRanking.lastUsedValue(history[key] ?? []) ?? "").isEmpty
    }
}
