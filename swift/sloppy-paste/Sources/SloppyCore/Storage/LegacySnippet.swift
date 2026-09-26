import Foundation

/// A snippet as it may appear in older data or imports: any field past the
/// basics may be missing, v1 data carries `category` instead of `tags`, and
/// `tags` may hold non-string junk.
public struct LegacySnippet: Decodable, Sendable, Hashable {
    public var id: String
    public var title: String
    public var content: String
    public var description: String?
    /// `nil` when the key is missing or not an array.
    public var tags: [String]?
    public var category: String?
    public var createdAt: Int64
    public var updatedAt: Int64
    public var lastUsedAt: Int64?
    public var useCount: Int?
    public var isFavorite: Bool?
    public var isArchived: Bool?
    public var isPinned: Bool?

    public init(
        id: String,
        title: String,
        content: String,
        description: String? = nil,
        tags: [String]? = nil,
        category: String? = nil,
        createdAt: Int64,
        updatedAt: Int64,
        lastUsedAt: Int64? = nil,
        useCount: Int? = nil,
        isFavorite: Bool? = nil,
        isArchived: Bool? = nil,
        isPinned: Bool? = nil
    ) {
        self.id = id
        self.title = title
        self.content = content
        self.description = description
        self.tags = tags
        self.category = category
        self.createdAt = createdAt
        self.updatedAt = updatedAt
        self.lastUsedAt = lastUsedAt
        self.useCount = useCount
        self.isFavorite = isFavorite
        self.isArchived = isArchived
        self.isPinned = isPinned
    }

    public init(_ snippet: Snippet) {
        self.init(
            id: snippet.id,
            title: snippet.title,
            content: snippet.content,
            description: snippet.description,
            tags: snippet.tags,
            createdAt: snippet.createdAt,
            updatedAt: snippet.updatedAt,
            lastUsedAt: snippet.lastUsedAt,
            useCount: snippet.useCount,
            isFavorite: snippet.isFavorite,
            isArchived: snippet.isArchived,
            isPinned: snippet.isPinned
        )
    }

    private enum CodingKeys: String, CodingKey {
        case id, title, content, description, tags, category, createdAt, updatedAt
        case lastUsedAt, useCount, isFavorite, isArchived, isPinned
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        title = try c.decode(String.self, forKey: .title)
        content = try c.decode(String.self, forKey: .content)
        description = try? c.decodeIfPresent(String.self, forKey: .description)
        tags = try? c.decodeIfPresent([LossyString].self, forKey: .tags)?.compactMap(\.value)
        category = try? c.decodeIfPresent(String.self, forKey: .category)
        createdAt = Self.timestamp(c, .createdAt) ?? 0
        updatedAt = Self.timestamp(c, .updatedAt) ?? 0
        lastUsedAt = Self.timestamp(c, .lastUsedAt)
        useCount = (try? c.decodeIfPresent(Int.self, forKey: .useCount))
            ?? (try? c.decodeIfPresent(Double.self, forKey: .useCount)).flatMap { $0.flatMap(Self.truncated) }
        isFavorite = try? c.decodeIfPresent(Bool.self, forKey: .isFavorite)
        isArchived = try? c.decodeIfPresent(Bool.self, forKey: .isArchived)
        isPinned = try? c.decodeIfPresent(Bool.self, forKey: .isPinned)
    }

    /// JS numbers may have been written as floats; accept both.
    private static func timestamp(_ c: KeyedDecodingContainer<CodingKeys>, _ key: CodingKeys) -> Int64? {
        if let value = try? c.decodeIfPresent(Int64.self, forKey: key) { return value }
        if let value = try? c.decodeIfPresent(Double.self, forKey: key) { return truncated(value) }
        return nil
    }

    /// Truncates toward zero; `nil` for non-finite values or values the
    /// integer type cannot represent (plain conversion would trap).
    private static func truncated<T: BinaryInteger>(_ value: Double) -> T? {
        guard value.isFinite else { return nil }
        return T(exactly: value.rounded(.towardZero))
    }

    /// Tags from `tags` when it is an array, otherwise from a legacy string `category`.
    var sourceTags: [String] {
        if let tags { return tags }
        return category.map { [$0] } ?? []
    }

    /// The full current-shape snippet: normalises tags and backfills defaults,
    /// dropping `category`.
    public func normalized() -> Snippet {
        filled(tags: TagNormalization.deduplicateTags(TagNormalization.normalizeTags(sourceTags)))
    }

    /// Backfills defaults but keeps tags as stored.
    func filled(tags: [String]) -> Snippet {
        Snippet(
            id: id,
            title: title,
            content: content,
            description: description ?? "",
            tags: tags,
            createdAt: createdAt,
            updatedAt: updatedAt,
            lastUsedAt: lastUsedAt,
            useCount: useCount ?? 0,
            isFavorite: isFavorite ?? false,
            isArchived: isArchived ?? false,
            isPinned: isPinned ?? false
        )
    }
}

/// Decodes any JSON value, keeping it only when it is a string.
struct LossyString: Decodable, Hashable {
    let value: String?

    init(from decoder: any Decoder) throws {
        value = try? decoder.singleValueContainer().decode(String.self)
    }
}
