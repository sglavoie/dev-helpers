import Foundation

/// Storage data as decoded before migration: every top-level field may be missing.
public struct LegacyStorageData: Decodable, Sendable, Hashable {
    public var version: Int?
    public var snippets: [LegacySnippet]
    public var tags: [String]
    public var placeholderHistory: PlaceholderHistory?

    public init(
        version: Int?,
        snippets: [LegacySnippet] = [],
        tags: [String] = [],
        placeholderHistory: PlaceholderHistory? = nil
    ) {
        self.version = version
        self.snippets = snippets
        self.tags = tags
        self.placeholderHistory = placeholderHistory
    }

    private enum CodingKeys: String, CodingKey {
        case version, snippets, tags, placeholderHistory
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        version = try? c.decodeIfPresent(Int.self, forKey: .version)
        snippets = try c.decodeIfPresent([LegacySnippet].self, forKey: .snippets) ?? []
        tags = (try? c.decodeIfPresent([LossyString].self, forKey: .tags))?.compactMap(\.value) ?? []
        placeholderHistory = try c.decodeIfPresent(PlaceholderHistory.self, forKey: .placeholderHistory)
    }
}

/// Upgrades stored data to `StorageConstants.currentVersion`, one version at a time.
public enum StorageMigrations {
    public struct Result: Sendable, Hashable {
        public var data: StorageData
        public var didMigrate: Bool
    }

    /// Decodes and migrates raw file contents. Throws when the JSON is not a
    /// storage document at all (the caller quarantines the file).
    public static func migrate(json: Data) throws -> Result {
        migrate(try JSONDecoder().decode(LegacyStorageData.self, from: json))
    }

    public static func migrate(_ legacy: LegacyStorageData) -> Result {
        var data = legacy
        var didMigrate = false
        // Snippets that went through a shape-normalising step; the rest keep
        // their stored tags and only get defaults filled in.
        var normalizedShape = false

        while let version = data.version, version < StorageConstants.currentVersion {
            guard let migration = migrations[version] else { break }
            migration(&data, &normalizedShape)
            data.version = version + 1
            didMigrate = true
        }

        if data.version != StorageConstants.currentVersion {
            data.version = StorageConstants.currentVersion
            didMigrate = true
        }

        let snippets = data.snippets.map { snippet in
            normalizedShape ? snippet.normalized() : snippet.filled(tags: snippet.sourceTags)
        }
        return Result(
            data: StorageData(
                version: StorageConstants.currentVersion,
                snippets: snippets,
                tags: data.tags,
                placeholderHistory: data.placeholderHistory ?? [:]
            ),
            didMigrate: didMigrate
        )
    }

    private typealias Migration = @Sendable (inout LegacyStorageData, inout Bool) -> Void

    /// Keyed by the version being migrated from. Steps that only backfill a
    /// default (v2 `isArchived`, v5 `description`) are covered by the final fill.
    private static let migrations: [Int: Migration] = [
        1: { data, normalized in
            data.snippets = data.snippets.map { LegacySnippet($0.normalized()) }
            normalized = true
        },
        2: { _, _ in },
        3: { data, _ in
            data.snippets = data.snippets.map { snippet in
                var copy = snippet
                copy.tags = TagNormalization.deduplicateTags(TagNormalization.normalizeTags(snippet.sourceTags))
                copy.category = nil
                return copy
            }
        },
        4: { data, _ in data.placeholderHistory = [:] },
        5: { _, _ in },
        6: { data, normalized in
            data.snippets = data.snippets.map { LegacySnippet($0.normalized()) }
            normalized = true
        },
    ]
}
