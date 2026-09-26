import Foundation

/// The snippet editor's working copy (TS SnippetForm and save-clipboard):
/// validation, tag parsing and saving through `SnippetRepository`.
public struct SnippetDraft: Sendable, Hashable {
    public enum Field: Sendable, Hashable, CaseIterable {
        case title, content, description, tags
    }

    public var title: String
    public var content: String
    public var description: String
    /// Comma-separated tag names, as typed.
    public var tagsText: String

    public init(title: String = "", content: String = "", description: String = "", tags: [String] = []) {
        self.title = title
        self.content = content
        self.description = description
        tagsText = tags.joined(separator: ", ")
    }

    public init(snippet: Snippet) {
        self.init(title: snippet.title, content: snippet.content, description: snippet.description, tags: snippet.tags)
    }

    /// A new-snippet draft for clipboard text, titled after its first line.
    /// Nil when the clipboard holds no text or only whitespace.
    public init?(clipboard text: String?) {
        guard let text, !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return nil }
        self.init(title: Self.suggestedTitle(for: text), content: text)
    }

    /// The first line, trimmed, truncated to 50 characters with "...".
    public static func suggestedTitle(for text: String, maxLength: Int = 50) -> String {
        let firstLine = text.split(separator: "\n", maxSplits: 1, omittingEmptySubsequences: false).first ?? ""
        let trimmed = firstLine.trimmingCharacters(in: .whitespacesAndNewlines)
        guard trimmed.count > maxLength else { return trimmed }
        return String(trimmed.prefix(maxLength - 3)) + "..."
    }

    // MARK: Tags

    public struct ParsedTags: Sendable, Hashable {
        public var tags: [String]
        public var error: String?
    }

    /// Splits `text` on commas and validates each tag. Whitespace inside a
    /// tag becomes a hyphen and duplicates collapse; the first invalid tag's
    /// error is reported as "tag: message".
    public static func parseTags(_ text: String) -> ParsedTags {
        var tags: [String] = []
        for part in text.split(separator: ",") {
            let raw = part.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !raw.isEmpty else { continue }
            let result = Validation.validateTag(raw)
            guard result.isValid else {
                return ParsedTags(tags: tags, error: "\(raw): \(result.error ?? "Invalid tag")")
            }
            let tag = result.normalizedValue ?? raw
            if !tags.contains(tag) { tags.append(tag) }
        }
        return ParsedTags(tags: tags, error: nil)
    }

    public var tags: [String] { Self.parseTags(tagsText).tags }

    // MARK: Validation

    /// Errors by field; empty when the draft can be saved.
    public var errors: [Field: String] {
        var errors: [Field: String] = [:]
        if let error = Validation.validateTitle(title).error { errors[.title] = error }
        if let error = Validation.validateContent(content).error { errors[.content] = error }
        if let error = Self.parseTags(tagsText).error { errors[.tags] = error }
        return errors
    }

    /// The first field with an error, in form order.
    public var firstErrorField: Field? {
        let errors = errors
        return Field.allCases.first { errors[$0] != nil }
    }

    // MARK: Saving

    /// Adds the draft as a new snippet. Title, content and description are
    /// trimmed, as in the TS form.
    @discardableResult
    public func create(in repository: inout SnippetRepository, now: Int64, id: String? = nil) throws -> Snippet {
        try validate()
        return repository.addSnippet(
            title: trimmed(title), content: trimmed(content), description: trimmed(description), tags: tags,
            now: now, id: id)
    }

    /// Writes the draft over an existing snippet, keeping its usage and flags.
    @discardableResult
    public func update(id: String, in repository: inout SnippetRepository, now: Int64) throws -> Snippet {
        try validate()
        return try repository.updateSnippet(id: id, now: now) { snippet in
            snippet.title = trimmed(title)
            snippet.content = trimmed(content)
            snippet.description = trimmed(description)
            snippet.tags = tags
        }
    }

    private func validate() throws {
        if let field = firstErrorField, let message = errors[field] {
            throw SnippetDraftError.invalid(field: field, message: message)
        }
    }

    private func trimmed(_ text: String) -> String {
        text.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}

public enum SnippetDraftError: Error, Sendable, Hashable, CustomStringConvertible {
    case invalid(field: SnippetDraft.Field, message: String)

    public var description: String {
        switch self {
        case .invalid(_, let message): message
        }
    }
}

/// The editor's ⇧⌘P "Insert Placeholder Syntax" entries (TS SYNTAX_HELPERS),
/// inserted at the cursor on ⌘1–⌘7.
public struct PlaceholderSyntaxHelper: Sendable, Hashable, Identifiable {
    public var title: String
    public var subtitle: String
    public var content: String
    /// The shortcut digit, "1" to "7".
    public var key: Character

    public var id: Character { key }

    public static let all: [PlaceholderSyntaxHelper] = [
        .init(title: "Basic Placeholder", subtitle: "{{key}}", content: "{{key}}", key: "1"),
        .init(title: "With Default", subtitle: "{{key|default}}", content: "{{key|default}}", key: "2"),
        .init(title: "No-Save Placeholder", subtitle: "{{!key}}", content: "{{!key}}", key: "3"),
        .init(title: "Wrapper Placeholder", subtitle: "{{prefix:key:suffix}}", content: "{{prefix:key:suffix}}",
            key: "4"),
        .init(title: "Conditional Block", subtitle: "{{#if key}}...{{/if}}", content: "{{#if key}}\n\n{{/if}}",
            key: "5"),
        .init(title: "If/Else Block", subtitle: "{{#if key}}...{{#else}}...{{/if}}",
            content: "{{#if key}}\n\n{{#else}}\n\n{{/if}}", key: "6"),
        .init(title: "Choice Placeholder", subtitle: "{{tone[Formal|Casual|Technical]|Casual}}",
            content: "{{tone[Formal|Casual|Technical]|Casual}}", key: "7"),
    ]

    /// The placeholder name inside `content` ("key" or "tone"), as a UTF-16
    /// range relative to the start of `content`, so the editor can select it
    /// after inserting.
    public var keyRange: NSRange {
        let text = content as NSString
        let range = text.range(of: "key")
        return range.location == NSNotFound ? text.range(of: "tone") : range
    }
}
