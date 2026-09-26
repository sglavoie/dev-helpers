/// A stored text snippet. Timestamps are epoch milliseconds, matching the Raycast data.
public struct Snippet: Codable, Sendable, Hashable, Identifiable {
    public var id: String
    public var title: String
    public var content: String
    public var description: String
    public var tags: [String]
    public var createdAt: Int64
    public var updatedAt: Int64
    public var lastUsedAt: Int64?
    public var useCount: Int
    /// Bookmarked for quick filtering; does not affect list order (use `isPinned` for that).
    public var isFavorite: Bool
    public var isArchived: Bool
    public var isPinned: Bool

    public init(
        id: String,
        title: String,
        content: String,
        description: String = "",
        tags: [String] = [],
        createdAt: Int64,
        updatedAt: Int64,
        lastUsedAt: Int64? = nil,
        useCount: Int = 0,
        isFavorite: Bool = false,
        isArchived: Bool = false,
        isPinned: Bool = false
    ) {
        self.id = id
        self.title = title
        self.content = content
        self.description = description
        self.tags = tags
        self.createdAt = createdAt
        self.updatedAt = updatedAt
        self.lastUsedAt = lastUsedAt
        self.useCount = useCount
        self.isFavorite = isFavorite
        self.isArchived = isArchived
        self.isPinned = isPinned
    }
}

/// Snippet IDs keep the Raycast `snippet-<ms>-<base36>` format so merge-imports stay compatible.
public enum SnippetID {
    private static let alphabet = Array("0123456789abcdefghijklmnopqrstuvwxyz")

    public static func make(now: Int64) -> String {
        var generator = SystemRandomNumberGenerator()
        return make(now: now, using: &generator)
    }

    public static func make(now: Int64, using generator: inout some RandomNumberGenerator) -> String {
        let suffix = String((0..<9).map { _ in alphabet.randomElement(using: &generator)! })
        return "snippet-\(now)-\(suffix)"
    }
}
