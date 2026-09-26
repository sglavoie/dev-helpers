/// One remembered value for a placeholder key.
public struct PlaceholderHistoryValue: Codable, Sendable, Hashable {
    public var value: String
    public var useCount: Int
    public var lastUsed: Int64
    public var createdAt: Int64

    public init(value: String, useCount: Int, lastUsed: Int64, createdAt: Int64) {
        self.value = value
        self.useCount = useCount
        self.lastUsed = lastUsed
        self.createdAt = createdAt
    }
}

/// Placeholder key (e.g. "name") to its remembered values.
public typealias PlaceholderHistory = [String: [PlaceholderHistoryValue]]

/// The on-disk document, in the same shape as the Raycast `StorageData`.
public struct StorageData: Codable, Sendable, Hashable {
    public var version: Int
    public var snippets: [Snippet]
    public var tags: [String]
    public var placeholderHistory: PlaceholderHistory

    public init(
        version: Int = StorageConstants.currentVersion,
        snippets: [Snippet] = [],
        tags: [String] = [],
        placeholderHistory: PlaceholderHistory = [:]
    ) {
        self.version = version
        self.snippets = snippets
        self.tags = tags
        self.placeholderHistory = placeholderHistory
    }

    public static var empty: StorageData { StorageData() }
}

/// The export document written by Raycast (⇧⌘E) and by this app.
public struct ExportData: Codable, Sendable, Hashable {
    public var version: String
    public var exportedAt: Int64
    public var snippets: [Snippet]
    public var tags: [String]
    public var placeholderHistory: PlaceholderHistory

    public init(
        version: String,
        exportedAt: Int64,
        snippets: [Snippet],
        tags: [String],
        placeholderHistory: PlaceholderHistory
    ) {
        self.version = version
        self.exportedAt = exportedAt
        self.snippets = snippets
        self.tags = tags
        self.placeholderHistory = placeholderHistory
    }
}
