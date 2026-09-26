import Foundation

public enum ValidationLimits {
    public static let titleMaxLength = 200
    /// About 100 KB of text.
    public static let contentMaxLength = 100_000
    public static let tagMaxLength = 50
    public static let tagMinLength = 1
    /// Maximum hierarchy depth, e.g. `a/b/c/d/e`.
    public static let tagMaxDepth = 5
}

public struct ValidationResult: Sendable, Hashable {
    public var isValid: Bool
    public var error: String?
    /// Set when the accepted value differs from the input (e.g. a tag with spaces).
    public var normalizedValue: String?

    public init(isValid: Bool, error: String? = nil, normalizedValue: String? = nil) {
        self.isValid = isValid
        self.error = error
        self.normalizedValue = normalizedValue
    }

    public static let valid = ValidationResult(isValid: true)

    public static func invalid(_ error: String) -> ValidationResult {
        ValidationResult(isValid: false, error: error)
    }
}

/// Input validation for snippet titles, content and tags. Error strings match the TS extension.
public enum Validation {
    public static func validateTitle(_ title: String) -> ValidationResult {
        let trimmed = title.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return .invalid("Title is required")
        }
        if trimmed.count > ValidationLimits.titleMaxLength {
            return .invalid(
                "Title must be \(ValidationLimits.titleMaxLength) characters or less (currently \(trimmed.count))")
        }
        return .valid
    }

    /// Checks presence and size, then reports the first placeholder syntax
    /// diagnostic from `syntaxDiagnostic` (by default the placeholder parser's
    /// first diagnostic message, or nil when the content parses cleanly).
    public static func validateContent(
        _ content: String,
        syntaxDiagnostic: (String) -> String? = { PlaceholderSyntaxParser.parse($0).diagnostics.first?.message }
    ) -> ValidationResult {
        let trimmed = content.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return .invalid("Content is required")
        }
        if trimmed.count > ValidationLimits.contentMaxLength {
            let sizeKB = String(format: "%.1f", Double(trimmed.count) / 1000)
            let maxKB = String(format: "%.0f", Double(ValidationLimits.contentMaxLength) / 1000)
            return .invalid("Content is too large (\(sizeKB)KB). Maximum size is \(maxKB)KB.")
        }
        if let diagnostic = syntaxDiagnostic(content) {
            return .invalid(diagnostic)
        }
        return .valid
    }

    /// Validates a tag name. Whitespace runs become hyphens and the result is lowercased;
    /// when that changes the (trimmed) input, `normalizedValue` carries the result.
    public static func validateTag(_ tag: String) -> ValidationResult {
        let trimmed = tag.trimmingCharacters(in: .whitespacesAndNewlines)
        let normalized = trimmed.replacing(/\s+/, with: "-").lowercased()

        if normalized.isEmpty {
            return .invalid("Tag name is required")
        }
        if normalized.count > ValidationLimits.tagMaxLength {
            return .invalid(
                "Tag name must be \(ValidationLimits.tagMaxLength) characters or less (currently \(normalized.count))")
        }
        if normalized.hasPrefix("/") || normalized.hasSuffix("/") {
            return .invalid("Tag cannot start or end with a slash")
        }
        if normalized.contains("//") {
            return .invalid("Tag cannot contain consecutive slashes")
        }
        let allowed = normalized.unicodeScalars.allSatisfy { scalar in
            scalar.isASCII && (CharacterSet.alphanumerics.contains(scalar) || "-_/".unicodeScalars.contains(scalar))
        }
        if !allowed {
            return .invalid("Tag can only contain letters, numbers, hyphens, underscores, and slashes")
        }
        let segments = normalized.split(separator: "/", omittingEmptySubsequences: false)
        if segments.count > ValidationLimits.tagMaxDepth {
            return .invalid("Tag hierarchy too deep (max \(ValidationLimits.tagMaxDepth) levels)")
        }
        if segments.contains(where: \.isEmpty) {
            return .invalid("Tag hierarchy segments cannot be empty")
        }
        if normalized != trimmed {
            return ValidationResult(isValid: true, normalizedValue: normalized)
        }
        return .valid
    }

    public struct CharacterInfo: Sendable, Hashable {
        public var count: Int
        public var remaining: Int
        public var info: String
    }

    /// Character count summary for a field with a length limit.
    public static func characterInfo(_ text: String, maxLength: Int) -> CharacterInfo {
        let count = text.count
        let remaining = maxLength - count
        let percentage = Double(count) / Double(maxLength) * 100

        let info: String
        if percentage < 50 {
            info = "\(count) / \(maxLength)"
        } else if percentage < 90 {
            info = "\(count) / \(maxLength) (\(remaining) remaining)"
        } else if count <= maxLength {
            info = "⚠️ \(remaining) characters remaining"
        } else {
            info = "❌ Exceeds limit by \(abs(remaining)) characters"
        }
        return CharacterInfo(count: count, remaining: remaining, info: info)
    }
}
