import Foundation

/// The informal `Prefix: rest of title` convention.
///
/// Titles namespace themselves with a prefix (`asl: run`, `Refactor: debt
/// triage`). The context is derived at render time and never stored, and
/// stripping it is display-only: search keeps reading the raw title.
public struct TitleContext: Sendable, Hashable {
    /// The prefix as written, or nil when the title has none.
    public var context: String?
    /// The title without its `Prefix: ` part (unchanged when there is no context).
    public var displayTitle: String

    public init(context: String?, displayTitle: String) {
        self.context = context
        self.displayTitle = displayTitle
    }

    /// Light/dark hex colours for a context badge.
    public struct Color: Sendable, Hashable {
        public var light: String
        public var dark: String

        public init(light: String, dark: String) {
            self.light = light
            self.dark = dark
        }
    }

    /// A prefix of more words than this reads as a sentence, not a namespace.
    static let maxContextWords = 3
    /// A context this short is shown whole in the badge.
    static let maxWholeLength = 5
    /// Characters kept when a single-word context is too long to show whole.
    static let truncatedLength = 4

    static let palette: [Color] = [
        Color(light: "#2B6CB0", dark: "#7AA7E9"), // blue
        Color(light: "#2F855A", dark: "#68D391"), // green
        Color(light: "#C05621", dark: "#F6AD55"), // orange
        Color(light: "#C53030", dark: "#FC8181"), // red
        Color(light: "#6B46C1", dark: "#B794F4"), // purple
        Color(light: "#B83280", dark: "#F687B3"), // magenta
        Color(light: "#2C7A7B", dark: "#4FD1C5"), // teal
        Color(light: "#975A16", dark: "#ECC94B"), // yellow
    ]

    /// Extracts the context from a title. The prefix is alphanumeric plus
    /// spaces, underscores and hyphens, at most 24 characters and three words.
    /// Whitespace is required after the colon, which keeps URLs and
    /// `Note:something` out, and a non-blank rest keeps a bare `asl:` intact.
    public static func parse(_ title: String) -> TitleContext {
        let unmatched = TitleContext(context: nil, displayTitle: title)
        guard let match = title.wholeMatch(of: /([A-Za-z0-9][A-Za-z0-9 _\-]{0,23}):\s+(\S.*)/) else {
            return unmatched
        }
        let prefix = String(match.1).trimmingCharacters(in: .whitespacesAndNewlines)
        guard !prefix.isEmpty, wordCount(prefix) <= maxContextWords else {
            return unmatched
        }
        return TitleContext(
            context: prefix, displayTitle: String(match.2).trimmingCharacters(in: .whitespacesAndNewlines))
    }

    /// Lowercased and trimmed, so `Refactor` and `refactor` unify.
    public static func normalize(_ context: String) -> String {
        context.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    }

    /// The uppercase badge label: short contexts whole (`asl` → `ASL`), long
    /// single words truncated (`workflow` → `WORK`), multi-word contexts as
    /// initials (`Dead Code` → `DC`, `dead-code` → `DC`).
    public static func abbreviation(_ context: String) -> String {
        let trimmed = context.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return "" }

        let label: String
        if trimmed.count <= maxWholeLength {
            label = trimmed
        } else {
            let segments = trimmed.split(whereSeparator: { $0.isWhitespace || $0 == "_" || $0 == "-" })
            if segments.count > 1 {
                label = String(segments.prefix(maxWholeLength).compactMap(\.first))
            } else {
                label = String(trimmed.prefix(truncatedLength))
            }
        }
        return label.uppercased()
    }

    /// Font size that lets a badge label of this length fit the pill.
    public static func badgeFontSize(forAbbreviation abbreviation: String) -> Double {
        switch abbreviation.count {
        case 1: 16
        case 2: 14
        case 3: 12
        case 4: 10
        default: 9
        }
    }

    /// A stable palette colour for a context; casing variants share a colour.
    public static func color(_ context: String) -> Color {
        palette[Int(hash(normalize(context)) % Int64(palette.count))]
    }

    /// Distinct normalised contexts across the snippets, sorted. Feeds `ctx:` completion.
    public static func allContexts(_ snippets: [Snippet]) -> [String] {
        Set(snippets.compactMap(snippetContext)).sorted(by: TagNormalization.localeLess)
    }

    /// The normalised context of a snippet, or nil.
    public static func snippetContext(_ snippet: Snippet) -> String? {
        parse(snippet.title).context.map(normalize)
    }

    private static func wordCount(_ text: String) -> Int {
        text.split(whereSeparator: \.isWhitespace).count
    }

    /// The JS `(hash * 31 + charCode) | 0` hash over UTF-16 units, then `Math.abs`,
    /// so colours match the Raycast extension.
    private static func hash(_ normalized: String) -> Int64 {
        var hash: Int32 = 0
        for unit in normalized.utf16 {
            hash = hash &* 31 &+ Int32(unit)
        }
        return abs(Int64(hash))
    }
}
